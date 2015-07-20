// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import "robpike.io/ivy/value"

type symtab map[string]value.Value

// execContext holds execution context, specifically the binding of names to values and operators.
type execContext struct {
	stack    []symtab
	unaryFn  map[string]*function
	binaryFn map[string]*function
	defs     []opDef
}

// NewContext returns a new execution context: the stack and variables.
func NewContext() value.Context {
	c := &execContext{
		stack:    []symtab{make(symtab)},
		unaryFn:  make(map[string]*function),
		binaryFn: make(map[string]*function),
	}
	c.SetConstants()
	return c
}

// SetConstants re-assigns the fundamental constant values using the current
// setting of floating-point precision.
func (c *execContext) SetConstants() {
	syms := c.stack[0]
	syms["e"], syms["pi"] = value.Consts()
}

// Lookup returns the value of a symbol.
func (c *execContext) Lookup(name string) value.Value {
	for i := len(c.stack) - 1; i >= 0; i-- {
		v := c.stack[i][name]
		if v != nil {
			return v
		}
	}
	return nil
}

// AssignLocal binds a value to the name in the current function.
func (c *execContext) AssignLocal(name string, value value.Value) {
	c.stack[len(c.stack)-1][name] = value
}

// Assign assigns the variable the value. The variable must
// be defined either in the current function or globally.
// Inside a function, new variables become locals.
func (c *execContext) Assign(name string, val value.Value) {
	n := len(c.stack)
	if n == 0 {
		value.Errorf("empty stack; cannot happen")
	}
	globals := c.stack[0]
	if n > 1 {
		// In this function?
		frame := c.stack[n-1]
		_, globallyDefined := globals[name]
		if _, ok := frame[name]; ok || !globallyDefined {
			frame[name] = val
			return
		}
	}
	// Assign global variable.
	c.noOp(name)
	globals[name] = val
}

// Push pushes a new frame onto the context stack.
func (c *execContext) Push() {
	c.stack = append(c.stack, make(symtab))
}

// Pop pops the top frame from the stack.
func (c *execContext) Pop() {
	c.stack = c.stack[:len(c.stack)-1]
}

// Eval evaluates a list of expressions.
func (c *execContext) Eval(exprs []value.Expr) []value.Value {
	var values []value.Value
	for _, expr := range exprs {
		v := expr.Eval(c)
		if v != nil {
			values = append(values, v)
		}
	}
	return values
}

// define defines the function and installs it. It also performs
// some error checking and adds the function to the sequencing
// information used by the save method.
func (c *execContext) define(fn *function) {
	c.noVar(fn.name)
	if fn.isBinary {
		c.binaryFn[fn.name] = fn
	} else {
		c.unaryFn[fn.name] = fn
	}
	// Update the sequence of definitions.
	// First, if it's last (a very common case) there's nothing to do.
	if len(c.defs) > 0 {
		last := c.defs[len(c.defs)-1]
		if last.name == fn.name && last.isBinary == fn.isBinary {
			return
		}
	}
	// Is it already defined?
	for i, def := range c.defs {
		if def.name == fn.name && def.isBinary == fn.isBinary {
			// Yes. Drop it.
			copy(c.defs[i:], c.defs[i+1:])
			c.defs = c.defs[:len(c.defs)-1]
			break
		}
	}
	// It is now the most recent definition.
	c.defs = append(c.defs, opDef{fn.name, fn.isBinary})
}

// noVar guarantees that there is no global variable with that name,
// preventing an op from being defined with the same name as a variable,
// which could cause problems. A variable with value zero is considered to
// be OK, so one can clear a variable before defining a symbol. A cleared
// variable is removed from the global symbol table.
// noVar also prevents defining builtin variables as ops.
func (c *execContext) noVar(name string) {
	if name == "_" || name == "pi" || name == "e" { // Cannot redefine these.
		value.Errorf(`cannot define op with name %q`, name)
	}
	sym := c.stack[0][name]
	if sym == nil {
		return
	}
	if i, ok := sym.(value.Int); ok && i == 0 {
		delete(c.stack[0], name)
		return
	}
	value.Errorf("cannot define op %s; it is a variable (%[1]s=0 to clear)", name)
}

// noOp is the dual of noVar. It also checks for assignment to builtins.
// It just errors out if there is a conflict.
func (c *execContext) noOp(name string) {
	if name == "pi" || name == "e" { // Cannot redefine these.
		value.Errorf(`cannot reassign %q`, name)
	}
	if c.unaryFn[name] == nil && c.binaryFn[name] == nil {
		return
	}
	value.Errorf("cannot define variable %s; it is an op", name)
}
