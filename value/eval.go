// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"
	"os"
	"strings"
)

type valueType int

const (
	intType valueType = iota
	charType
	bigIntType
	bigRatType
	bigFloatType
	vectorType
	matrixType
	numType
)

var sigChan chan os.Signal

func init() {
	sigChan = make(chan os.Signal, 100)
	// signal.Notify(sigChan, syscall.SIGINT) // TODO: Find a finer-grained way to handle this.
}

func CheckInterrupt() {
	select {
	case <-sigChan:
		Errorf("interrupted")
	default:
	}
}

func DrainInterrupt() {
	for {
		select {
		case <-sigChan:
			fmt.Fprintln(os.Stderr, "interrupted")
		default:
			return
		}
	}
}

var typeName = [...]string{"int", "char", "big int", "rational", "float", "vector", "matrix"}

func (t valueType) String() string {
	return typeName[t]
}

type unaryFn func(Value) Value

type unaryOp struct {
	elementwise bool // whether the operation applies elementwise to vectors and matrices
	fn          [numType]unaryFn
}

func Unary(opName string, v Value) Value {
	CheckInterrupt()
	if len(opName) > 1 {
		if strings.HasSuffix(opName, `/`) {
			return reduce(opName[:len(opName)-1], v)
		}
		if strings.HasSuffix(opName, `\`) {
			return scan(opName[:len(opName)-1], v)
		}
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
	case Char:
		return charType
	case BigInt:
		return bigIntType
	case BigRat:
		return bigRatType
	case BigFloat:
		return bigFloatType
	case Vector:
		return vectorType
	case Matrix:
		return matrixType
	}
	Errorf("unknown type %T in whichType", v)
	panic("which type")
}

func Binary(u Value, opName string, v Value) Value {
	CheckInterrupt()
	if strings.Contains(opName, ".") {
		return product(u, opName, v)
	}
	op := binaryOps[opName]
	if op == nil {
		Errorf("binary %s not implemented", opName)
	}
	which := op.whichType(whichType(u), whichType(v))
	u = u.toType(which)
	v = v.toType(which)
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

func product(u Value, opName string, v Value) Value {
	dot := strings.IndexByte(opName, '.')
	left := opName[:dot]
	right := opName[dot+1:]
	which := atLeastVectorType(whichType(u), whichType(v))
	u = u.toType(which)
	v = v.toType(which)
	if left == "o" {
		return outerProduct(u, right, v)
	}
	return innerProduct(u, left, right, v)
}

// u and v are known to be the same type and at least Vectors.
func innerProduct(u Value, left, right string, v Value) Value {
	switch u := u.(type) {
	case Vector:
		v := v.(Vector)
		u.sameLength(v)
		var x Value
		for k, e := range u {
			tmp := Binary(e, right, v[k])
			if k == 0 {
				x = tmp
			} else {
				x = Binary(x, left, tmp)
			}
		}
		return x
	case Matrix:
		// Say we're doing +.*
		// result[i,j] = +/(u[row i] * v[column j])
		// The result is a square matrix with each dimension the number of columns of the lhs.
		v := v.(Matrix)
		if len(u.shape) != 2 || len(v.shape) != 2 {
			Errorf("can't do inner product on shape %s times %s", u.shape, v.shape)
		}
		urows := int(u.shape[0].(Int))
		ucols := int(u.shape[1].(Int))
		vrows := int(v.shape[0].(Int))
		vcols := int(v.shape[1].(Int))
		if vrows != ucols || vcols != urows {
			Errorf("shape mismatch for inner product %s times %s", u.shape, v.shape)
		}
		data := make(Vector, urows*urows)
		shape := NewVector([]Value{u.shape[0], u.shape[0]})
		row, col := 0, 0
		for i := range data {
			acc := Binary(u.data[row*ucols], right, v.data[col])
			for j := 1; j < ucols; j++ {
				acc = Binary(acc, left, Binary(u.data[row*ucols+j], right, v.data[j*vcols+col]))
			}
			data[i] = acc
			col++
			if col >= urows {
				row++
				col = 0
			}
		}
		return NewMatrix(shape, data)
	}
	Errorf("can't do inner product on %s", whichType(u))
	panic("not reached")
}

// u and v are known to be at least Vectors.
func outerProduct(u Value, opName string, v Value) Value {
	switch u := u.(type) {
	case Vector:
		v := v.(Vector)
		m := Matrix{
			shape: NewVector([]Value{Int(len(u)), Int(len(v))}),
			data:  NewVector(make(Vector, len(u)*len(v))),
		}
		index := 0
		for _, vu := range u {
			for _, vv := range v {
				m.data[index] = Binary(vu, opName, vv)
				index++
			}
		}
		return m // TODO: Shrink?
	}
	Errorf("can't do outer product on %s", whichType(u))
	panic("not reached")
}

// We must be right associative; that is the grammar.
// -/1 2 3 == 1-2-3 is 1-(2-3) not (1-2)-3. Answer: 2.
func reduce(opName string, v Value) Value {
	switch v := v.(type) {
	case Int, BigInt, BigRat:
		return v
	case Vector:
		if len(v) == 0 {
			return v
		}
		acc := v[len(v)-1]
		for i := len(v) - 2; i >= 0; i-- {
			acc = Binary(v[i], opName, acc)
		}
		return acc
	case Matrix:
		if len(v.shape) < 2 {
			Errorf("shape for matrix is degenerate: %s", v.shape)
		}
		stride := int(v.shape[len(v.shape)-1].(Int))
		if stride == 0 {
			Errorf("shape for matrix is degenerate: %s", v.shape)
		}
		shape := v.shape[:len(v.shape)-1]
		data := make(Vector, size(shape))
		index := 0
		for i := range data {
			pos := index + stride - 1
			acc := v.data[pos]
			pos--
			for i := 1; i < stride; i++ {
				acc = Binary(v.data[pos], opName, acc)
				pos--
			}
			data[i] = acc
			index += stride
		}
		if len(shape) == 1 { // TODO: Matrix.shrink()?
			return NewVector(data)
		}
		return NewMatrix(shape, data)
	}
	Errorf("can't do reduce on %s", whichType(v))
	panic("not reached")
}

// scan gives the successive values of reducing op through v.
// We must be right associative; that is the grammar.
func scan(opName string, v Value) Value {
	switch v := v.(type) {
	case Int, BigInt, BigRat:
		return v
	case Vector:
		if len(v) == 0 {
			return v
		}
		values := make(Vector, len(v))
		acc := v[0]
		values[0] = acc
		// TODO: This is n^2.
		for i := 1; i < len(v); i++ {
			values[i] = reduce(opName, v[:i+1])
		}
		return NewVector(values)
	case Matrix:
		if len(v.shape) < 2 {
			Errorf("shape for matrix is degenerate: %s", v.shape)
		}
		stride := int(v.shape[len(v.shape)-1].(Int))
		if stride == 0 {
			Errorf("shape for matrix is degenerate: %s", v.shape)
		}
		data := make(Vector, len(v.data))
		index := 0
		nrows := 1
		for i := 0; i < len(v.shape)-1; i++ {
			// Guaranteed by NewMatrix not to overflow.
			nrows *= int(v.shape[i].(Int))
		}
		for i := 0; i < nrows; i++ {
			acc := v.data[index]
			data[index] = acc
			// TODO: This is n^2.
			for j := 1; j < stride; j++ {
				data[index+j] = reduce(opName, v.data[index:index+j+1])
			}
			index += stride
		}
		return NewMatrix(v.shape, data)
	}
	Errorf("can't do scan on %s", whichType(v))
	panic("not reached")
}

// unaryVectorOp applies op elementwise to i.
func unaryVectorOp(op string, i Value) Value {
	u := i.(Vector)
	n := make([]Value, len(u))
	for k := range u {
		n[k] = Unary(op, u[k])
	}
	return NewVector(n)
}

// unaryMatrixOp applies op elementwise to i.
func unaryMatrixOp(op string, i Value) Value {
	u := i.(Matrix)
	n := make([]Value, len(u.data))
	for k := range u.data {
		n[k] = Unary(op, u.data[k])
	}
	return NewMatrix(u.shape, NewVector(n))
}

// binaryVectorOp applies op elementwise to i and j.
func binaryVectorOp(i Value, op string, j Value) Value {
	u, v := i.(Vector), j.(Vector)
	if len(u) == 1 {
		n := make([]Value, len(v))
		for k := range v {
			n[k] = Binary(u[0], op, v[k])
		}
		return NewVector(n)
	}
	if len(v) == 1 {
		n := make([]Value, len(u))
		for k := range u {
			n[k] = Binary(u[k], op, v[0])
		}
		return NewVector(n)
	}
	u.sameLength(v)
	n := make([]Value, len(u))
	for k := range u {
		n[k] = Binary(u[k], op, v[k])
	}
	return NewVector(n)
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
		n = make([]Value, len(v.data))
		for k := range v.data {
			n[k] = Binary(u.data[0], op, v.data[k])
		}
	case isScalar(v):
		// Matrix op Scalar.
		n = make([]Value, len(u.data))
		for k := range u.data {
			n[k] = Binary(u.data[k], op, v.data[0])
		}
	case isVector(u, v.shape):
		// Vector op Matrix.
		shape = v.shape
		n = make([]Value, len(v.data))
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
		n = make([]Value, len(u.data))
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
		n = make([]Value, len(u.data))
		for k := range u.data {
			n[k] = Binary(u.data[k], op, v.data[k])
		}
	}
	return NewMatrix(shape, NewVector(n))
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
