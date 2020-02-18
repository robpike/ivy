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
	case variableExpr:
		context.Assign(lhs.name, rhs)
		return Assignment{Value: rhs}
	case *binary:
		if lhs.op == "[]" {
			return indexedAssignment(context, lhs, b.right, rhs)
		}
	}
	value.Errorf("cannot assign %s to %s", b.left.ProgString(), b.right.ProgString())
	panic("not reached")
}

// indexedAssignment handles general assignment to indexed expressions on the LHS.
// The LHS must be derived from a variable to make sure it is an l-value.
func indexedAssignment(context value.Context, lhs *binary, rhsExpr value.Expr, rhs value.Value) value.Value {
	// We walk down the index chain evaluating indexes and
	// comparing them to the shape vector of the LHS.
	// Once we're there, we copy the rhs to the lhs, doing a slice copy.
	// rhsExpr is for diagnostics (only), as it gives a better error print.
	slice, shape := dataAndShape(true, lhs, lvalueOf(context, lhs.left))
	indexes := indexesOf(context, lhs)
	origin := int(value.Int(context.Config().Origin()))
	offset := 0
	var i int
	for i = range shape {
		if i >= len(indexes) {
			value.Errorf("rank error assigning %s to %s", rhs, lhs.ProgString())
		}
		size := shapeProduct(shape[i+1:])
		index := indexes[i] - origin
		if index < 0 || shape[i] <= index {
			value.Errorf("index of out of range in assignment")
		}
		// We're either going to skip this block, or we're at the
		// end of the indexes and we're going to assign it.
		if i < len(indexes)-1 {
			// Skip.
			offset += index * size
			continue
		}
		// Assign.
		rhsData, rhsShape := dataAndShape(false, rhsExpr, rhs)
		dataSize := shapeProduct(rhsShape)
		// Shapes must match.
		if !sameShape(shape[i+1:], rhsShape) {
			value.Errorf("data size/shape mismatch assigning %s to %s", rhs, lhs.ProgString())
		}
		offset += index * size
		if dataSize == 1 {
			slice[offset] = rhsData[0]
		} else {
			copy(slice[offset:offset+size], rhsData)
		}
		return Assignment{Value: rhs}
	}
	value.Errorf("cannot assign to element of %s", lhs.left.ProgString())
	panic("not reached")
}

func dataAndShape(mustBeLvalue bool, expr value.Expr, val value.Value) ([]value.Value, []int) {
	switch v := val.(type) {
	case value.Vector:
		return v, toInt([]value.Value{value.Int(len(v))})
	case *value.Matrix:
		return v.Data(), v.Shape()
	default:
		if mustBeLvalue {
			return nil, nil
		}
		return []value.Value{val}, scalarShape
	}
}

func shapeProduct(shape []int) int {
	elemSize := 1
	for _, v := range shape {
		elemSize *= v
	}
	return elemSize
}

// sameShape reports whether the two assignment shape vectors are equivalent.
// The lhs in particular can be empty if we have exhausted the indexes, but that
// just means we are assigning to a scalar element, and is OK.
func sameShape(a, b []int) bool {
	if len(a) == 0 {
		a = scalarShape
	}
	if len(b) == 0 {
		b = scalarShape
	}
	if len(a) != len(b) {
		return false
	}
	for i, av := range a {
		if av != b[i] {
			return false
		}
	}
	return true
}

func toInt(v []value.Value) []int {
	res := make([]int, len(v))
	for i, val := range v {
		res[i] = int(val.(value.Int))
	}
	return res
}

// lvalueOf walks the index tree to find the variable that roots it.
// It must evaluate to a non-scalar to be indexable.
func lvalueOf(context value.Context, item value.Expr) value.Value {
	switch lhs := item.(type) {
	case variableExpr:
		lvalue := lhs.Eval(context)
		if lvalue.Rank() == 0 {
			break
		}
		return lvalue
	case *binary:
		if lhs.op == "[]" {
			return lvalueOf(context, lhs.left)
		}
	}
	value.Errorf("cannot index %s in assignment", item.ProgString())
	panic("not reached")
}

func indexesOf(context value.Context, item value.Expr) []int {
	switch v := item.(type) {
	case *binary:
		if v.op == "[]" {
			if _, ok := v.left.(variableExpr); ok {
				return indexesOf(context, v.right)
			}
			return append(indexesOf(context, v.left), indexesOf(context, v.right)...)
		}
	default:
		v = v.Eval(context)
		if i, ok := v.(value.Int); ok {
			return []int{int(i)}
		}
		value.Errorf("cannot index by %s in assignment", item.ProgString())
	}
	return nil
}
