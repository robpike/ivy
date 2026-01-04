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

	frameSizes []int // size of each stack frame on the call stack
	stack      []*value.Var

	Stack []*value.Frame

	Globals value.Symtab

	//  UnaryFn maps the names of unary functions (ops) to their implementations.
	UnaryFn map[string]*Function
	//  BinaryFn maps the names of binary functions (ops) to their implementations.
	BinaryFn map[string]*Function
	// Defs is a list of defined ops, in time order.  It is used when saving the
	// Context to a file.
	Defs []OpDef
	// Names of variables declared in the currently-being-parsed function.
	variables []string

	// Already in a traceback.
	tracing bool
}

// NewContext returns a new execution context: the stack and variables,
// plus the execution configuration.
func NewContext(conf *config.Config) value.Context {
	c := &Context{
		config:   conf,
		Globals:  make(value.Symtab),
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
		c.Globals[name] = value.NewVar(name, val)
	} else {
		v.Assign(val)
	}
}

// Local returns the value of the local variable with index i.
func (c *Context) Local(i int) *value.Var {
	v := c.stack[len(c.stack)-i]
	if v == nil {
		v = value.NewVar("", nil)
		c.stack[len(c.stack)-i] = v
	}
	return v
}

func (c *Context) initStack() {
	c.stack = c.stack[:0]
	c.Stack = c.Stack[:0]
}

func (c *Context) TopOfStack() *value.Frame {
	return c.Stack[len(c.Stack)-1]
}

// push pushes a new local frame onto the context stack.
func (c *Context) push(fn *Function) {
	n := len(c.stack)
	for cap(c.stack) < n+len(fn.Locals) {
		c.stack = append(c.stack[:cap(c.stack)], nil)
	}
	c.frameSizes = append(c.frameSizes, len(fn.Locals))
	c.stack = c.stack[:n+len(fn.Locals)]
	c.Stack = append(c.Stack, fn.newFrame())
}

// pop pops the top frame from the stack.
func (c *Context) pop() {
	n := c.frameSizes[len(c.frameSizes)-1]
	c.frameSizes = c.frameSizes[:len(c.frameSizes)-1]
	c.stack = c.stack[:len(c.stack)-n]
	c.Stack = c.Stack[:len(c.Stack)-1]
}

// Errorf panics with the formatted string, with type Error.
func (c *Context) Errorf(format string, args ...interface{}) {
	if c.tracing {
		return
	}
	c.tracing = true // In case we panic again trying to print the trace.
	defer func() { c.tracing = false }()
	err := value.Error(fmt.Sprintf(format, args...))
	c.StackTrace()
	c.initStack()
	panic(err)
}

// StackTrace prints the execution stack, and wipes it.
// There may be conditions under which it will cause trouble
// by printing invalid values, but it tries to be safe.
// TODO: Should be able to do this without wiping the stack.
func (c *Context) StackTrace() {
	const max = 25
	n := len(c.Stack)
	if n > max {
		fmt.Fprintf(c.Config().ErrOutput(), "\t•> stack truncated: %d calls total; showing innermost\n", n)
		n = max
	}
	// We need to print the innermost, then pop it and go around.
	// But we want the output to be innermost last, so save and reverse.
	lines := []string{}
	for range n {
		if len(c.Stack) == 0 {
			break
		}
		f := c.TopOfStack()
		if !f.Inited {
			continue
		}
		left := c.ArgPrint(f.Left)
		if left != "" {
			left += " "
		}
		right := c.ArgPrint(f.Right)
		lines = append(lines, fmt.Sprintf("\t•> %s%s %s\n", left, f.Name, right))
		c.pop()
	}
	for i := len(lines) - 1; i >= 0; i-- {
		fmt.Fprint(c.Config().ErrOutput(), lines[i])
	}
}

func (c *Context) ArgPrint(arg value.Expr) string {
	s := ""
	switch a := arg.(type) {
	case nil:
		return "" // No parens.
	default:
		s = fmt.Sprintf("%T %s", a, a.ProgString())
	case *value.VarExpr:
		s = a.Eval(c).Sprint(c)
	case value.VectorExpr:
		s = a.Eval(c).Sprint(c)
	}
	if len(s) > 50 {
		s = s[:50] + "..."
	}
	return "(" + s + ")"
}

var indent = "| "

// TraceIndent returns an indentation marker showing the depth of the stack.
func (c *Context) TraceIndent() string {
	n := 2 * len(c.Stack)
	if len(indent) < n {
		indent = strings.Repeat("| ", n+10)
	}
	return indent[:n]
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
		v, ok := value.EvalCharEqual(left, op == "==", right)
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

// Define defines the function and installs it. It also performs
// some error checking and adds the function to the sequencing
// information used by the save method.
func (c *Context) Define(fn *Function) {
	c.noVar(fn.Name)
	if fn.IsBinary {
		c.BinaryFn[fn.Name] = fn
	} else {
		c.UnaryFn[fn.Name] = fn
	}
	// Update the sequence of definitions.
	// First, if it's last (a very common case) there's nothing to do.
	if len(c.Defs) > 0 {
		last := c.Defs[len(c.Defs)-1]
		if last.Name == fn.Name && last.IsBinary == fn.IsBinary {
			return
		}
	}
	// Is it already defined?
	i, ok := c.LookupFn(fn.Name, fn.IsBinary)
	if ok {
		c.Defs = append(c.Defs[:i], c.Defs[i+1:]...)
	}
	// It is now the most recent definition.
	c.Defs = append(c.Defs, OpDef{fn.Name, fn.IsBinary})
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
		c.Globals = make(value.Symtab)
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
// preventing an op from being defined with the same name as a variable,
// which could cause problems. A variable with value zero is considered to
// be OK, so one can clear a variable before defining a symbol. A cleared
// variable is removed from the global symbol table.
// noVar also prevents defining builtin variables as ops.
func (c *Context) noVar(name string) {
	if name == "_" || name == "pi" || name == "e" { // Cannot redefine these.
		value.Errorf(`cannot define op with name %q`, name)
	}
	sym := c.Globals[name]
	if sym == nil {
		return
	}
	if i, ok := sym.Value().(value.Int); ok && i == 0 {
		delete(c.Globals, name)
		return
	}
	value.Errorf("cannot define op %s; it is a variable; use ')clear %[1]s' to clear)", name)
}

// noOp is the dual of noVar. It also checks for assignment to builtins.
// It just errors out if there is a conflict.
func (c *Context) noOp(name string) {
	if name == "pi" || name == "e" { // Cannot redefine these.
		value.Errorf("cannot reassign %q", name)
	}
	if c.UnaryFn[name] == nil && c.BinaryFn[name] == nil {
		return
	}
	value.Errorf("cannot define variable %s; it is an op", name)
}

// Declare makes the name a variable while parsing the next function.
func (c *Context) Declare(name string) {
	c.variables = append(c.variables, name)
}

// ForgetAll forgets the declared variables.
func (c *Context) ForgetAll() {
	c.variables = nil
}

func (c *Context) isVariable(op string) bool {
	for _, s := range c.variables {
		if op == s {
			return true
		}
	}
	return false
}
