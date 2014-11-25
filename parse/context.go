// Copyright 2014 Rob Pike. All rights reserved.
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
}

// NewexecContext returns a new context.
func NewContext() value.Context {
	return &execContext{
		stack:    []symtab{make(symtab)},
		unaryFn:  make(map[string]*function),
		binaryFn: make(map[string]*function),
	}
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
func (c *execContext) Assign(name string, value value.Value) {
	n := len(c.stack)
	if n == 0 {
		c.stack[0][name] = value
		return
	}
	// In this function?
	globals := c.stack[0]
	frame := c.stack[n-1]
	_, globallyDefined := globals[name]
	if _, ok := frame[name]; ok || !globallyDefined {
		frame[name] = value
		return
	}
	// Assign global variable.
	globals[name] = value
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
