// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

// Code for assignment, a little intricate as there are many cases and many
// validity checks.

// Assignment is an implementation of Value that is created as the result of an assignment.
// It can be type-asserted to discover whether the returned value was created by assignment,
// such as is done in the interpreter to avoid printing the results of assignment expressions.
type Assignment struct {
	Value
}

var scalarShape = []int{1} // The assignment shape vector for a scalar

func Assign(context Context, b *BinaryExpr) Value {
	// We know the left is a variableExpr or index expression.
	// Special handling as we must not evaluate the left - it is an l-
	// But we need to process the indexing, if it is an index expression.
	rhs := b.Right.Eval(context).Inner()
	switch lhs := b.Left.(type) {
	case *VarExpr:
		if lhs.Local >= 1 {
			context.AssignLocal(lhs.Local, rhs)
		} else {
			context.AssignGlobal(lhs.Name, rhs)
		}
		return Assignment{Value: rhs}
	case *IndexExpr:
		switch lhs.Left.(type) {
		case *VarExpr:
			IndexAssign(context, lhs, lhs.Left, lhs.Right, b.Right, rhs)
			return Assignment{Value: rhs}
		case *IndexExpr:
			// Old x[i][j]. Show new syntax.
			n := 0
			for x := lhs; x != nil; x, _ = x.Left.(*IndexExpr) {
				n += len(x.Right)
			}
			list := make([]Expr, n)
			last := lhs.Left
			for x := lhs; x != nil; x, _ = x.Left.(*IndexExpr) {
				n -= len(x.Right)
				copy(list[n:], x.Right)
				last = x.Left
			}
			fixed := &IndexExpr{Left: last, Right: list}
			Errorf("cannot assign to %s; use %v", b.Left.ProgString(), fixed.ProgString())
		}
	case VectorExpr:
		// Simultaneous assignment requires evaluation of RHS before assignment.
		rhs, ok := b.Right.Eval(context).Inner().(*Vector)
		if !ok {
			Errorf("rhs of assignment to (%s) not a vector", lhs.ProgString())
		}
		if len(lhs) != rhs.Len() {
			Errorf("length mismatch in assignment to (%s)", lhs.ProgString())
		}
		values := make([]Value, rhs.Len())
		for i := rhs.Len() - 1; i >= 0; i-- {
			values[i] = rhs.At(i).Eval(context).Inner()
		}
		for i, v := range lhs {
			vbl := v.(*VarExpr) // Guaranteed to be only a variable on LHS.
			if vbl.Local >= 1 {
				context.AssignLocal(vbl.Local, values[i])
			} else {
				context.AssignGlobal(vbl.Name, values[i])
			}
		}
		return Assignment{NewVector(values)}
	}
	Errorf("cannot assign to %s", b.Left.ProgString())
	panic("not reached")
}
