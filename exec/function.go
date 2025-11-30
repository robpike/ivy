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
	IsBinary bool
	Name     string
	Left     value.Expr
	Right    value.Expr
	Body     []value.Expr
	Locals   []string
	Globals  []string
	Source   string
	// At time of definition; needed to parse saved source correctly.
	Ibase int
}

// argProgString builds a string representation of arg, to be used in printing the
// source to an op. If the argument is a vector, it needs special handling to get
// parentheses and nesting.
func argProgString(b *strings.Builder, arg value.Expr) {
	switch expr := arg.(type) {
	case *value.VarExpr:
		b.WriteString(expr.ProgString())
		return
	case value.VectorExpr:
		b.WriteRune('(')
		for i, elem := range expr {
			if i > 0 {
				b.WriteRune(' ')
			}
			argProgString(b, elem)
		}
		b.WriteRune(')')
	default:
		b.WriteString(fmt.Sprintf("<unknown type in op print: %T>", arg))
	}
}

func (fn *Function) String() string {
	var b strings.Builder
	b.WriteString("op ")
	if fn.IsBinary {
		argProgString(&b, fn.Left)
		b.WriteRune(' ')
	}
	b.WriteString(fn.Name)
	b.WriteRune(' ')
	argProgString(&b, fn.Right)
	b.WriteString(" = ")
	if len(fn.Body) == 1 {
		b.WriteString(fn.Body[0].ProgString())
	} else {
		for _, stmt := range fn.Body {
			b.WriteString("\n\t")
			b.WriteString(stmt.ProgString())
		}
	}
	return b.String()
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
	value.Assign(context, fn.Right, right, right)
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
	value.Assign(context, fn.Left, left, left)
	value.Assign(context, fn.Right, right, right)
	v := value.EvalFunctionBody(c, fn.Name, fn.Body)
	if v == nil {
		value.Errorf("no value returned by %q", fn.Name)
	}
	return v
}
