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
	numType
)

var typeName = [...]string{"int", "big int", "rational", "vector"}

func (t valueType) String() string {
	return typeName[t]
}

type unaryFn func(Value) Value

type unaryOp struct {
	fn [numType]unaryFn
}

func Unary(opName string, v Value) Value {
	if strings.HasSuffix(opName, `\`) {
		return Reduce(opName[:len(opName)-1], v)
	}
	op := unaryOps[opName]
	if op == nil {
		panic(Errorf("unary %s not implemented", opName))
	}
	which := whichType(v)
	fn := op.fn[which]
	if fn == nil {
		panic(Errorf("unary %s not implemented on type %s", opName, which))
	}
	return fn(v)
}

type binaryFn func(Value, Value) Value

type binaryOp struct {
	whichType func(a, b valueType) valueType
	fn        [numType]binaryFn
}

type reduceOp struct {
	zero Value
	fn   unaryFn
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
	}
	panic("which type")
}

func Binary(v1 Value, opName string, v2 Value) Value {
	op := binaryOps[opName]
	if op == nil {
		panic(Errorf("binary %s not implemented", opName))
	}
	which := op.whichType(whichType(v1), whichType(v2))
	fn := op.fn[which]
	if fn == nil {
		panic(Errorf("binary %s not implemented on type %s", opName, which))
	}
	return fn(v1.ToType(which), v2.ToType(which))
}

func Reduce(opName string, v Value) Value {
	vec, ok := v.(Vector)
	if !ok {
		panic(Error("reduction operand is not a vector"))
	}
	acc := vec.x[0]
	for i := 1; i < vec.Len(); i++ {
		acc = Binary(acc, opName, vec.x[i]) // TODO!
	}
	return acc
}
