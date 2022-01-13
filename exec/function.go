// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

import (
	"fmt"

	"robpike.io/ivy/value"
)

// Function represents a unary or binary user-defined operator.
type Function struct {
	IsBinary bool
	Name     string
	Left     string
	Right    string
	Body     []value.Expr
	Locals   []string
	Globals  []string
}

func (fn *Function) String() string {
	left := ""
	if fn.IsBinary {
		left = fn.Left + " "
	}
	s := fmt.Sprintf("op %s%s %s =", left, fn.Name, fn.Right)
	if len(fn.Body) == 1 {
		return s + " " + fn.Body[0].ProgString()
	}
	for _, stmt := range fn.Body {
		s += "\n\t" + stmt.ProgString()
	}
	return s
}

func (fn *Function) EvalUnary(context value.Context, right value.Value) value.Value {
	if fn.Body == nil {
		value.Errorf("unary %q undefined", fn.Name)
	}
	// It's known to be an exec.Context.
	c := context.(*Context)
	if uint(len(c.frameSizes)) >= c.config.MaxStack() {
		value.Errorf("stack overflow calling %q", fn.Name)
	}
	c.push(fn)
	defer c.pop()
	c.AssignLocal(1, right)
	v := value.EvalFunctionBody(c, fn.Name, fn.Body)
	if v == nil {
		value.Errorf("no value returned by %q", fn.Name)
	}
	return v
}

func (fn *Function) EvalBinary(context value.Context, left, right value.Value) value.Value {
	if fn.Body == nil {
		value.Errorf("binary %q undefined", fn.Name)
	}
	// It's known to be an exec.Context.
	c := context.(*Context)
	if uint(len(c.frameSizes)) >= c.config.MaxStack() {
		value.Errorf("stack overflow calling %q", fn.Name)
	}
	c.push(fn)
	defer c.pop()
	c.AssignLocal(1, left)
	c.AssignLocal(2, right)
	v := value.EvalFunctionBody(c, fn.Name, fn.Body)
	if v == nil {
		value.Errorf("no value returned by %q", fn.Name)
	}
	return v
}
