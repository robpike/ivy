// Copyright 2014 Rob Pike. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value // import "robpike.io/ivy/value"

import "strings"

type valueType int

const (
	intType valueType = iota
	bigIntType
	bigRatType
	vectorType
	matrixType
	numType
)

var typeName = [...]string{"int", "big int", "rational", "vector", "matrix"}

func (t valueType) String() string {
	return typeName[t]
}

type unaryFn func(Value) Value

type unaryOp struct {
	elementwise bool // whether the operation applies elementwise to vectors and matrices
	fn          [numType]unaryFn
}

func Unary(opName string, v Value) Value {
	if len(opName) > 1 && strings.HasSuffix(opName, `/`) {
		return Reduce(opName[:len(opName)-1], v)
	}
	op := unaryOps[opName]
	if op == nil {
		Errorf("unary %s not implemented", opName)
	}
	which := whichType(v)
	fn := op.fn[which]
	if fn == nil {
		if op.elementwise {
			switch which {
			case vectorType:
				return unaryVectorOp(opName, v)
			case matrixType:
				return unaryMatrixOp(opName, v)
			}
		}
		Errorf("unary %s not implemented on type %s", opName, which)
	}
	return fn(v)
}

type binaryFn func(Value, Value) Value

type binaryOp struct {
	elementwise bool // whether the operation applies elementwise to vectors and matrices
	whichType   func(a, b valueType) valueType
	fn          [numType]binaryFn
}

func whichType(v Value) valueType {
	switch v.(type) {
	case Int:
		return intType
	case BigInt:
		return bigIntType
	case BigRat:
		return bigRatType
	case Vector:
		return vectorType
	case Matrix:
		return matrixType
	}
	panic("which type")
}

func Binary(u Value, opName string, v Value) Value {
	if strings.Contains(opName, ".") {
		return innerProduct(u, opName, v)
	}
	op := binaryOps[opName]
	if op == nil {
		Errorf("binary %s not implemented", opName)
	}
	which := op.whichType(whichType(u), whichType(v))
	u = u.ToType(which)
	v = v.ToType(which)
	fn := op.fn[which]
	if fn == nil {
		if op.elementwise {
			switch which {
			case vectorType:
				return binaryVectorOp(u, opName, v)
			case matrixType:
				return binaryMatrixOp(u, opName, v)
			}
		}
		Errorf("binary %s not implemented on type %s", opName, which)
	}
	return fn(u, v)
}

func outerProduct(u Value, opName string, v Value) Value {
	// Vectors only for now, but can promote from scalars.
	i := u.ToType(vectorType).(Vector)
	j := v.ToType(vectorType).(Vector)
	m := Matrix{
		shape: ValueSlice([]Value{Int(len(i)), Int(len(j))}),
		data:  ValueSlice(make(Vector, len(i)*len(j))),
	}
	index := 0
	for _, vi := range i {
		for _, vj := range j {
			m.data[index] = Binary(vi, opName, vj)
			index++
		}
	}
	return m // TODO: Shrink?
}

func innerProduct(u Value, opName string, v Value) Value {
	dot := strings.IndexByte(opName, '.')
	left := opName[:dot]
	right := opName[dot+1:]
	if left == "o" {
		return outerProduct(u, right, v)
	}
	// Vectors only for now, but can promote from scalars.
	i := u.ToType(vectorType).(Vector)
	j := v.ToType(vectorType).(Vector)
	i.sameLength(j)
	var x Value
	for k, e := range i {
		tmp := Binary(e, right, j[k])
		if k == 0 {
			x = tmp
		} else {
			x = Binary(x, left, tmp)
		}
	}
	return x
}

func Reduce(opName string, v Value) Value {
	switch v := v.(type) {
	case Int, BigInt, BigRat:
		return v
	case Vector:
		acc := v[0]
		for i := 1; i < v.Len(); i++ {
			acc = Binary(acc, opName, v[i])
		}
		return acc
	case Matrix:
		if len(v.shape) < 2 {
			Errorf("shape for matrix is degenerate: %s", v.shape)
		}
		stride := int(v.shape[len(v.shape)-1].(Int))
		shape := v.shape[:len(v.shape)-1]
		data := make(Vector, size(shape))
		index := 0
		for i := range data {
			acc := v.data[index]
			index++
			for i := 1; i < stride; i++ {
				acc = Binary(acc, opName, v.data[index])
				index++
			}
			data[i] = acc
		}
		if len(shape) == 1 { // TODO: Matrix.shrink()?
			return ValueSlice(data)
		}
		return Matrix{
			shape: shape,
			data:  data,
		}
	}
	Errorf("bad type for reduce")
	panic("not reached")
}

// unaryVectorOp applies op elementwise to i.
func unaryVectorOp(op string, i Value) Value {
	u := i.(Vector)
	n := make([]Value, u.Len())
	for k := range u {
		n[k] = Unary(op, u[k])
	}
	return ValueSlice(n)
}

// unaryMatrixOp applies op elementwise to i.
func unaryMatrixOp(op string, i Value) Value {
	u := i.(Matrix)
	n := make([]Value, u.data.Len())
	for k := range u.data {
		n[k] = Unary(op, u.data[k])
	}
	return Matrix{
		shape: u.shape,
		data:  ValueSlice(n),
	}
}

// binaryVectorOp applies op elementwise to i and j.
func binaryVectorOp(i Value, op string, j Value) Value {
	u, v := i.(Vector), j.(Vector)
	if len(u) == 1 {
		n := make([]Value, v.Len())
		for k := range v {
			n[k] = Binary(u[0], op, v[k])
		}
		return ValueSlice(n)
	}
	if len(v) == 1 {
		n := make([]Value, u.Len())
		for k := range u {
			n[k] = Binary(u[k], op, v[0])
		}
		return ValueSlice(n)
	}
	u.sameLength(v)
	n := make([]Value, u.Len())
	for k := range u {
		n[k] = Binary(u[k], op, v[k])
	}
	return ValueSlice(n)
}

// binaryMatrixOp applies op elementwise to i and j.
func binaryMatrixOp(i Value, op string, j Value) Value {
	u, v := i.(Matrix), j.(Matrix)
	shape := u.shape
	var n []Value
	// One or the other may be a scalar in disguise.
	switch {
	case isScalar(u):
		// Scalar op Matrix.
		shape = v.shape
		n = make([]Value, v.data.Len())
		for k := range v.data {
			n[k] = Binary(u.data[0], op, v.data[k])
		}
	case isScalar(v):
		// Matrix op Scalar.
		n = make([]Value, u.data.Len())
		for k := range u.data {
			n[k] = Binary(u.data[k], op, v.data[0])
		}
	case isVector(u, v.shape):
		// Vector op Matrix.
		shape = v.shape
		n = make([]Value, v.data.Len())
		dim := int(u.shape[0].(Int))
		index := 0
		for k := range v.data {
			n[k] = Binary(u.data[index], op, v.data[k])
			index++
			if index >= dim {
				index = 0
			}
		}
	case isVector(v, u.shape):
		// Vector op Matrix.
		n = make([]Value, u.data.Len())
		dim := int(v.shape[0].(Int))
		index := 0
		for k := range u.data {
			n[k] = Binary(v.data[index], op, u.data[k])
			index++
			if index >= dim {
				index = 0
			}
		}
	default:
		// Matrix op Matrix.
		u.sameShape(v)
		n = make([]Value, u.data.Len())
		for k := range u.data {
			n[k] = Binary(u.data[k], op, v.data[k])
		}
	}
	return Matrix{
		shape,
		ValueSlice(n),
	}
}

// isScalar reports whether u is a 1x1x1x... item, that is, a scalar promoted to matrix.
func isScalar(u Matrix) bool {
	for _, dim := range u.shape {
		if dim.(Int) != 1 {
			return false
		}
	}
	return true
}

// isVector reports whether u is an 1x1x...xn item where n is the last dimension
// of the shape, that is, an n-vector promoted to matrix.
func isVector(u Matrix, shape Vector) bool {
	if len(u.shape) == 0 || len(shape) == 0 || u.shape[0] != shape[len(shape)-1] {
		return false
	}
	for _, dim := range u.shape[1:] {
		if dim.(Int) != 1 {
			return false
		}
	}
	return true
}
