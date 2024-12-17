// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

// Code for assignment, a little intricate as there are many cases and many
// validity checks.

import (
	"robpike.io/ivy/value"
)

// Assignment is an implementation of Value that is created as the result of an assignment.
// It can be type-asserted to discover whether the returned value was created by assignment,
// such as is done in the interpreter to avoid printing the results of assignment expressions.
type Assignment struct {
	value.Value
}

var scalarShape = []int{1} // The assignment shape vector for a scalar value.

func assignment(context value.Context, b *binary) value.Value {
	// We know the left is a variableExpr or index expression.
	// Special handling as we must not evaluate the left - it is an l-value.
	// But we need to process the indexing, if it is an index expression.
	rhs := b.right.Eval(context).Inner()
	switch lhs := b.left.(type) {
	case *variableExpr:
		if lhs.local >= 1 {
			context.AssignLocal(lhs.local, rhs)
		} else {
			context.AssignGlobal(lhs.name, rhs)
		}
		return Assignment{Value: rhs}
	case *index:
		switch lhs.left.(type) {
		case *variableExpr:
			value.IndexAssign(context, lhs, lhs.left, lhs.right, b.right, rhs)
			return Assignment{Value: rhs}
		case *index:
			// Old x[i][j]. Show new syntax.
			n := 0
			for x := lhs; x != nil; x, _ = x.left.(*index) {
				n += len(x.right)
			}
			list := make([]value.Expr, n)
			last := lhs.left
			for x := lhs; x != nil; x, _ = x.left.(*index) {
				n -= len(x.right)
				copy(list[n:], x.right)
				last = x.left
			}
			fixed := &index{left: last, right: list}
			value.Errorf("cannot assign to %s; use %v", b.left.ProgString(), fixed.ProgString())
		}
	case sliceExpr:
		// Simultaneous assignment requires evaluation of RHS before assignment.
		rhs, ok := b.right.Eval(context).Inner().(*value.Vector)
		if !ok {
			value.Errorf("rhs of assignment to (%s) not a vector", lhs.ProgString())
		}
		if len(lhs) != rhs.Len() {
			value.Errorf("length mismatch in assignment to (%s)", lhs.ProgString())
		}
		values := make([]value.Value, rhs.Len())
		for i := rhs.Len() - 1; i >= 0; i-- {
			values[i] = rhs.At(i).Eval(context).Inner()
		}
		for i, v := range lhs {
			vbl := v.(*variableExpr) // Guaranteed to be only a variable on LHS.
			if vbl.local >= 1 {
				context.AssignLocal(vbl.local, values[i])
			} else {
				context.AssignGlobal(vbl.name, values[i])
			}
		}
		return Assignment{value.NewVector(values)}
	}
	value.Errorf("cannot assign to %s", b.left.ProgString())
	panic("not reached")
}
