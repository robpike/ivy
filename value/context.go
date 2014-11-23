// Copyright 2014 Rob Pike. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

type Expr interface {
	String() string

	Eval(context *Context) Value
}

type symtab map[string]Value

// Context holds execution context, specifically the binding of names to values.
type Context struct {
	stack []symtab
}

// NewContext returns a new context.
func NewContext() *Context {
	return &Context{
		stack: []symtab{make(symtab)},
	}
}

// Lookup returns the value of a symbol.
func (c *Context) Lookup(name string) Value {
	for i := len(c.stack) - 1; i >= 0; i-- {
		v := c.stack[i][name]
		if v != nil {
			return v
		}
	}
	return nil
}

// AssignLocal binds a value to the name in the current function.
func (c *Context) AssignLocal(name string, value Value) {
	c.stack[len(c.stack)-1][name] = value
}

// Assign assigns the variable the value. The variable must
// be defined either in the current function or globally.
// Inside a function, new variables become locals.
func (c *Context) Assign(name string, value Value) {
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
func (c *Context) Push() {
	c.stack = append(c.stack, make(symtab))
}

// Pop pops the top frame from the stack.
func (c *Context) Pop() {
	c.stack = c.stack[:len(c.stack)-1]
}

// Eval evaluates a list of expressions.
func (c *Context) Eval(exprs []Expr) []Value {
	var values []Value
	for _, expr := range exprs {
		v := expr.Eval(c)
		if v != nil {
			values = append(values, v)
		}
	}
	return values
}
