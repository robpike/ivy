// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

// Code for assignment, a little intricate as there are many cases and many
// validity checks.

// QuietValue is an implementation of Value that is created as the result of an
// assignment or print operator. It can be type-asserted to discover whether to
// avoid printing the results of the expression.
type QuietValue struct {
	Value
}

var scalarShape = []int{1} // The assignment shape vector for a scalar

func assign(context Context, b *BinaryExpr) Value {
	rhs := b.Right.Eval(context).Inner()
	Assign(context, b.Left, b.Right, rhs)
	return QuietValue{Value: rhs}
}

func Assign(c Context, left, right Expr, rhs Value) {
	// We know the left is a variableExpr or index expression.
	// Special handling as we must not evaluate the left - it is an l-value.
	// But we need to process the indexing, if it is an index expression.
	switch lhs := left.(type) {
	case *VarExpr:
		frame := c.TopOfStack()
		if frame == nil {
			c.AssignGlobal(lhs.Name, rhs)
			return
		}
		for _, variable := range frame.Vars {
			name := variable.Name()
			if name == lhs.Name {
				switch variable.state {
				case Unknown:
					c.Local(name).Assign(rhs)
					variable.state = LocalVar
					return
				case LocalVar:
					c.Local(name).Assign(rhs)
					variable.state = LocalVar
					return
				case GlobalVar:
					c.AssignGlobal(name, rhs)
					variable.state = GlobalVar // We could delete it but it's fine to leave it.
					return
				}
				c.Errorf("internal error: unknown local state %d for %s in assign", variable.state, variable.name)
			}
		}
		// A global not mentioned in the current function.
		c.AssignGlobal(lhs.Name, rhs)
		return
	case *IndexExpr:
		switch lv := lhs.Left.(type) {
		case *VarExpr:
			IndexAssign(c, lhs, lhs.Left, lv, lhs.Right, right, rhs)
			return
		}
	case VectorExpr:
		// Simultaneous assignment requires evaluation of RHS before assignment.
		rhs, ok := rhs.(*Vector)
		if !ok {
			c.Errorf("rhs of assignment to (%s) not a vector", DebugProgString(lhs))
		}
		if len(lhs) != rhs.Len() {
			c.Errorf("length mismatch in assignment to (%s)", DebugProgString(lhs))
		}
		for i := rhs.Len() - 1; i >= 0; i-- {
			Assign(c, lhs[i], nil, rhs.At(i))
		}
		return
	}
	c.Errorf("cannot assign to %s", DebugProgString(left))
}
