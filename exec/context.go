// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec // import "robpike.io/ivy/exec"

import (
	"fmt"
	"strings"

	"robpike.io/ivy/config"
	"robpike.io/ivy/value"
)

// Context holds execution context, specifically the binding of names to values and operators.
// It is the only implementation of ../value/Context, but since it references the value
// package, there would be a cycle if that package depended on this type definition.
type Context struct {
	// config is the configuration state used for evaluation, printing, etc.
	// Accessed through the value.Context Config method.
	config *config.Config

	Stack []*value.Frame

	Globals map[string]*value.Var

	//  UnaryFn maps the names of unary functions (ops) to their implementations.
	UnaryFn map[string]*Function
	//  BinaryFn maps the names of binary functions (ops) to their implementations.
	BinaryFn map[string]*Function
	// Defs is a list of defined ops in order of creation.
	Defs []OpDef

	// pos records the source position.
	pos value.Pos

	// Used to silence tracing for caught failures.
	disableTracing bool
}

// NewContext returns a new execution context: the stack and variables,
// plus the execution configuration.
func NewContext(conf *config.Config) value.Context {
	c := &Context{
		config:   conf,
		Globals:  map[string]*value.Var{},
		Stack:    []*value.Frame{},
		UnaryFn:  make(map[string]*Function),
		BinaryFn: make(map[string]*Function),
	}
	c.SetConstants()
	return c
}

func (c *Context) Config() *config.Config {
	return c.config
}

// SetConstants re-assigns the fundamental constant values using the current
// setting of floating-point precision.
func (c *Context) SetConstants() {
	e, pi := value.Consts(c)
	c.AssignGlobal("e", e)
	c.AssignGlobal("pi", pi)
}

// Global returns a global variable, or nil if the symbol is not defined globally.
func (c *Context) Global(name string) *value.Var {
	return c.Globals[name]
}

// AssignGlobal assigns to the named global variable, creating it if needed.
func (c *Context) AssignGlobal(name string, val value.Value) {
	v := c.Globals[name]
	if v == nil {
		c.Globals[name] = value.NewVar(name, val, value.GlobalVar)
	} else {
		v.Assign(val)
	}
}

// IsLocal reports whether the identifier names a defined local variable.
func (c *Context) IsLocal(name string) bool {
	if len(c.Stack) == 0 {
		return false
	}
	for _, v := range c.TopOfStack().Vars {
		if v.Name() == name {
			return v.State() == value.LocalVar
		}
	}
	return false
}

// Local returns the Var descriptor for the named local variable,
// or nil if it is not present.
func (c *Context) Local(name string) *value.Var {
	for _, variable := range c.TopOfStack().Vars { // Usually a short list.
		if variable.Name() == name {
			return variable
		}
	}
	return nil
}

func (c *Context) initStack() {
	c.Stack = c.Stack[:0]
}

func (c *Context) TopOfStack() *value.Frame {
	if len(c.Stack) == 0 {
		return nil
	}
	return c.Stack[len(c.Stack)-1]
}

// push pushes a new local frame onto the context stack.
func (c *Context) push(fn *Function) {
	c.Stack = append(c.Stack, fn.newFrame())
}

// pop pops the top frame from the stack.
func (c *Context) pop() {
	c.Stack = c.Stack[:len(c.Stack)-1]
}

func (c *Context) Pos() value.Pos {
	return c.pos
}

func (c *Context) SetPos(file string, line, offset int) {
	c.pos = value.Pos{
		File:   file,
		Line:   line,
		Offset: offset,
	}
}

// Errorf panics with the formatted string, with type Error.
func (c *Context) Errorf(format string, args ...interface{}) {
	disableTracing := c.disableTracing
	c.disableTracing = true // In case we panic after this point.
	defer func() { c.disableTracing = false }()
	err := value.Error{
		Pos: c.pos,
		Err: fmt.Sprintf(format, args...),
	}
	if !disableTracing {
		c.StackTrace()
	}
	c.initStack()
	panic(err)
}

func (c *Context) DisableTracing(t bool) {
	c.disableTracing = t
}

// Eval evaluates a list of expressions.
func (c *Context) Eval(exprs []value.Expr) []value.Value {
	var values []value.Value
	for _, expr := range exprs {
		v := expr.Eval(c)
		if v != nil {
			values = append(values, v)
		}
	}
	return values
}

// EvalUnary evaluates a unary operator, including reductions and scans.
func (c *Context) EvalUnary(op string, right value.Value) value.Value {
	if len(op) > 1 {
		switch op[len(op)-1] {
		case '/':
			value.TraceUnary(c, 2, op, right)
			return value.Reduce(c, op[:len(op)-1], right)
		case '\\':
			value.TraceUnary(c, 2, op, right)
			return value.Scan(c, op[:len(op)-1], right)
		case '%':
			if len(op) > 2 {
				switch op[len(op)-2] {
				case '/':
					value.TraceUnary(c, 2, op, right)
					return value.ReduceFirst(c, op[:len(op)-2], right)
				case '\\':
					value.TraceUnary(c, 2, op, right)
					return value.ScanFirst(c, op[:len(op)-2], right)
				}
			}
		case '@':
			value.TraceUnary(c, 2, op, right)
			return value.Each(c, op, right)
		}
	}
	fn, userDefined := c.unary(op)
	if fn == nil {
		c.Errorf("unary %q not implemented", op)
	}
	if userDefined {
		value.TraceUnary(c, 1, op, right)
	}
	return fn.EvalUnary(c, right)
}

func (c *Context) unary(op string) (fn value.UnaryOp, userDefined bool) {
	userFn := c.UnaryFn[op]
	if userFn != nil {
		return userFn, true
	}
	builtin := value.UnaryOps[op]
	if builtin != nil {
		return builtin, false
	}
	return nil, false
}

func (c *Context) UserDefined(op string, isBinary bool) bool {
	if isBinary {
		return c.BinaryFn[op] != nil
	}
	return c.UnaryFn[op] != nil
}

// EvalBinary evaluates a binary operator, including products.
func (c *Context) EvalBinary(left value.Value, op string, right value.Value) value.Value {
	// Special handling for the equal and non-equal operators, which must avoid
	// type conversions involving Char.
	if op == "==" || op == "!=" {
		v, ok := value.EvalCharEqual(c, left, op == "==", right)
		if ok {
			value.TraceBinary(c, 2, left, op, right) // Only trace if we've done it.
			return v
		}
	}
	if strings.Trim(op, "@") != op {
		value.TraceBinary(c, 2, left, op, right)
		return value.BinaryEach(c, left, op, right)
	}
	if strings.Contains(op, ".") {
		value.TraceBinary(c, 2, left, op, right)
		return value.Product(c, left, op, right)
	}
	fn, userDefined := c.binary(op)
	if fn == nil {
		c.Errorf("binary %q not implemented", op)
	}
	if userDefined {
		value.TraceBinary(c, 1, left, op, right)
	}
	return fn.EvalBinary(c, left, right)
}

func (c *Context) binary(op string) (fn value.BinaryOp, userDefined bool) {
	user := c.BinaryFn[op]
	if user != nil {
		return user, true
	}
	builtin := value.BinaryOps[op]
	if builtin != nil {
		return builtin, false
	}
	return nil, false
}

// Define installs the function in the Context after a little more error checking.
func (c *Context) Define(fn *Function) {
	c.noVar(fn.Name)
	if fn.IsBinary {
		c.BinaryFn[fn.Name] = fn
	} else {
		c.UnaryFn[fn.Name] = fn
	}
	// Update the OpDefs list.
	// First, if it's last (a very common case) there's nothing to do.
	if len(c.Defs) > 0 {
		last := c.Defs[len(c.Defs)-1]
		if last.Name == fn.Name && last.IsBinary == fn.IsBinary {
			return
		}
	}
	// Is it already defined? If so we can just replace the old definition.
	def := OpDef{fn.Name, fn.IsBinary}
	i, ok := c.LookupFn(fn.Name, fn.IsBinary)
	if ok {
		c.Defs[i] = def
	} else {
		c.Defs = append(c.Defs, def)
	}
}

// UndefineAll deletes all user-defined names of the types
// specified by the arguments, of which several may be set.
func (c *Context) UndefineAll(unary, binary, vars bool) {
	if unary || binary {
		// Take care in iterating as list changes as we go.
		defs := make([]OpDef, len(c.Defs))
		copy(defs, c.Defs)
		for _, def := range defs {
			if unary {
				c.UndefineOp(def.Name, false)
			}
			if binary {
				c.UndefineOp(def.Name, true)
			}
		}
	}
	if vars {
		c.Globals = make(map[string]*value.Var)
	}
}

// UndefineOp removes the op with the given name and arity and reports
// whether it was present.
func (c *Context) UndefineOp(name string, binary bool) bool {
	i, ok := c.LookupFn(name, binary)
	if !ok {
		return false
	}
	if binary {
		delete(c.BinaryFn, name)
	} else {
		delete(c.UnaryFn, name)
	}
	c.Defs = append(c.Defs[:i], c.Defs[i+1:]...)
	return true
}

// UndefineVar removes the named variable and reports whether it was present.
func (c *Context) UndefineVar(name string) bool {
	if _, ok := c.Globals[name]; ok {
		delete(c.Globals, name)
		return true
	}
	return false
}

// RestoreOp restores the argument function to the definition data structures,
// preserving its order in the definition list. Used by the parser to replace a
// function whose redefinition failed.
func (c *Context) RestoreOp(i int, fn *Function) bool {
	if fn.IsBinary {
		c.BinaryFn[fn.Name] = fn
	} else {
		c.UnaryFn[fn.Name] = fn
	}
	defs := c.Defs[:i]
	defs = append(defs, OpDef{fn.Name, fn.IsBinary})
	defs = append(defs, c.Defs[i:]...)
	c.Defs = defs
	return true
}

// LookupFn returns the index into the definition list for the function.
func (c *Context) LookupFn(name string, isBinary bool) (int, bool) {
	for i, def := range c.Defs {
		if def.Name != name || def.IsBinary != isBinary {
			continue
		}
		return i, true
	}
	return 0, false
}

// noVar guarantees that there is no global variable with that name,
// preventing an op from being defined with the same name as a variable by accident.
// A variable with value zero is considered to
// be OK, so one can clear a variable before defining a symbol. A cleared
// variable is removed from the global symbol table.
func (c *Context) noVar(name string) {
	if name == "_" || name == "pi" || name == "e" { // Cannot redefine these.
		c.Errorf(`cannot define op with name %q`, name)
	}
	sym := c.Globals[name]
	if sym == nil {
		return
	}
	if i, ok := sym.Value().(value.Int); ok && i == 0 {
		delete(c.Globals, name)
		return
	}
	c.Errorf("cannot define op %s; it is a variable; use ')clear %[1]s' to clear)", name)
}

// FlushSavedParses clears all saved parses of ops in this context.
func (c *Context) FlushSavedParses() {
	for _, fn := range c.BinaryFn {
		value.FlushState(fn.Body)
	}
	for _, fn := range c.UnaryFn {
		value.FlushState(fn.Body)
	}
}
