// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

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
		panic(Errorf("unary %s not implemented", opName))
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
		panic(Errorf("unary %s not implemented on type %s", opName, which))
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
	if dot := strings.IndexByte(opName, '.'); dot > 0 {
		left := opName[:dot]
		right := opName[dot+1:]
		return InnerProduct(u, left, right, v)
	}
	op := binaryOps[opName]
	if op == nil {
		panic(Errorf("binary %s not implemented", opName))
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
		panic(Errorf("binary %s not implemented on type %s", opName, which))
	}
	return fn(u, v)
}

func InnerProduct(u Value, left, right string, v Value) Value {
	// Vectors only for now.
	i, ok := u.(Vector)
	if !ok {
		panic(Errorf("inner product not implemented on type %s", whichType(u)))
	}
	j, ok := v.(Vector)
	if !ok {
		panic(Errorf("inner product not implemented on type %s", whichType(v)))
	}
	i.sameLength(j)
	var x Value = zero
	switch left {
	case "*", "/": // TODO: what are the correct operators here? Should we complain?
		x = one
	}
	for k, e := range i {
		tmp := Binary(e, right, j[k])
		x = Binary(x, left, tmp)
	}
	return x
}

func Reduce(opName string, v Value) Value {
	vec, ok := v.(Vector)
	if !ok {
		panic(Error("reduction operand is not a vector"))
	}
	acc := vec[0]
	for i := 1; i < vec.Len(); i++ {
		acc = Binary(acc, opName, vec[i]) // TODO!
	}
	return acc
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
	case len(u.shape) == 1 && u.shape[0].(Int) == 1:
		// Scalar op Matrix.
		shape = v.shape
		n = make([]Value, v.data.Len())
		for k := range v.data {
			n[k] = Binary(u.data[0], op, v.data[k])
		}
	case len(v.shape) == 1 && v.shape[0].(Int) == 1:
		// Matrix op Scalar.
		n = make([]Value, u.data.Len())
		for k := range u.data {
			n[k] = Binary(u.data[k], op, v.data[0])
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
