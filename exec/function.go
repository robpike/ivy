// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

import (
	"fmt"
	"strings"

	"robpike.io/ivy/value"
)

// Function represents a unary or binary user-defined operator.
type Function struct {
	IsBinary  bool
	Name      string
	Left      value.Expr
	Right     value.Expr
	Body      value.StatementList
	Variables []string // Names mentioned in the body that could be vars.
	Source    string
	HasRet    bool
	// At time of definition; needed to parse saved source correctly.
	Ibase int
}

// Used for debugging. The output is not valid Ivy syntax.
func (fn *Function) String() string {
	var b strings.Builder
	b.WriteString("op ")
	if fn.IsBinary {
		b.WriteString(value.DebugProgString(fn.Left))
		b.WriteRune(' ')
	}
	b.WriteString(fn.Name)
	b.WriteRune(' ')
	b.WriteString(value.DebugProgString(fn.Right))
	b.WriteString(" = {")
	b.WriteString(fn.Source)
	fmt.Fprintf(&b, "} {Variables: %s} ", fn.Variables)
	return b.String()
}

func (fn *Function) newFrame() *value.Frame {
	frame := &value.Frame{
		Op:       fn.Name,
		IsBinary: fn.IsBinary,
		Left:     fn.Left,
		Right:    fn.Right,
		Inited:   false,
		Vars:     value.Symtab{},
	}
	frame.Vars = make([]*value.Var, len(fn.Variables))
	for i, id := range fn.Variables {
		frame.Vars[i] = value.NewVar(id, nil, value.Unknown)
	}
	return frame
}

func (fn *Function) EvalUnary(context value.Context, right value.Value) value.Value {
	// It's known to be an exec.Context.
	c := context.(*Context)
	if fn.Body == nil {
		c.Errorf("unary %q undefined", fn.Name)
	}
	if uint(len(c.Stack)) >= c.config.MaxStack() {
		c.Errorf("stack overflow calling %q", fn.Name)
	}
	c.push(fn)
	value.Assign(context, fn.Right, right, right)
	c.TopOfStack().Inited = true
	v := value.EvalFunctionBody(c, fn.Name, fn.Body, fn.HasRet)
	if v == nil {
		c.Errorf("no value returned by %q", fn.Name)
	}
	c.pop() // Don't defer, so if we get an error we can print the stack.
	return v
}

func (fn *Function) EvalBinary(context value.Context, left, right value.Value) value.Value {
	if fn.Body == nil {
		context.Errorf("binary %q undefined", fn.Name)
	}
	// It's known to be an exec.Context.
	c := context.(*Context)
	if uint(len(c.Stack)) >= c.config.MaxStack() {
		c.Errorf("stack overflow calling %q", fn.Name)
	}
	c.push(fn)
	value.Assign(context, fn.Left, left, left)
	value.Assign(context, fn.Right, right, right)
	c.TopOfStack().Inited = true
	v := value.EvalFunctionBody(c, fn.Name, fn.Body, fn.HasRet)
	if v == nil {
		c.Errorf("no value returned by %q", fn.Name)
	}
	c.pop() // Don't defer, so if we get an error we can print the stack.
	return v
}
