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

func assign(context Context, b *BinaryExpr) Value {
	rhs := b.Right.Eval(context).Inner()
	Assign(context, b.Left, b.Right, rhs)
	return Assignment{Value: rhs.Copy()}
}

func Assign(context Context, left, right Expr, rhs Value) {
	// We know the left is a variableExpr or index expression.
	// Special handling as we must not evaluate the left - it is an l-
	// But we need to process the indexing, if it is an index expression.
	switch lhs := left.(type) {
	case *VarExpr:
		if lhs.Local >= 1 {
			context.AssignLocal(lhs.Local, rhs)
		} else {
			context.AssignGlobal(lhs.Name, rhs)
		}
		return
	case *IndexExpr:
		switch lhs.Left.(type) {
		case *VarExpr:
			IndexAssign(context, lhs, lhs.Left, lhs.Right, right, rhs)
			return
		}
	case VectorExpr:
		// Simultaneous assignment requires evaluation of RHS before assignment.
		rhs, ok := rhs.(*Vector)
		if !ok {
			Errorf("rhs of assignment to (%s) not a vector", lhs.ProgString())
		}
		if len(lhs) != rhs.Len() {
			Errorf("length mismatch in assignment to (%s)", lhs.ProgString())
		}
		for i := rhs.Len() - 1; i >= 0; i-- {
			Assign(context, lhs[i], nil, rhs.At(i))
		}
		return
	}
	// unexpected: parser should have caught this
	Errorf("internal error: cannot assign to %s", left.ProgString())
}
