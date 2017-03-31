// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "strings"

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

var typeName = [...]string{"int", "char", "big int", "rational", "float", "vector", "matrix"}

func (t valueType) String() string {
	return typeName[t]
}

type unaryFn func(Context, Value) Value

type unaryOp struct {
	name        string
	elementwise bool // whether the operation applies elementwise to vectors and matrices
	fn          [numType]unaryFn
}

func (op *unaryOp) EvalUnary(c Context, v Value) Value {
	which := whichType(v)
	fn := op.fn[which]
	if fn == nil {
		if op.elementwise {
			switch which {
			case vectorType:
				return unaryVectorOp(c, op.name, v)
			case matrixType:
				return unaryMatrixOp(c, op.name, v)
			}
		}
		Errorf("unary %s not implemented on type %s", op.name, which)
	}
	return fn(c, v)
}

type binaryFn func(Context, Value, Value) Value

type binaryOp struct {
	name        string
	elementwise bool // whether the operation applies elementwise to vectors and matrices
	whichType   func(a, b valueType) valueType
	fn          [numType]binaryFn
}

func whichType(v Value) valueType {
	switch v.Inner().(type) {
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

func (op *binaryOp) EvalBinary(c Context, u, v Value) Value {
	which := op.whichType(whichType(u), whichType(v))
	conf := c.Config()
	u = u.toType(conf, which)
	v = v.toType(conf, which)
	fn := op.fn[which]
	if fn == nil {
		if op.elementwise {
			switch which {
			case vectorType:
				return binaryVectorOp(c, u, op.name, v)
			case matrixType:
				return binaryMatrixOp(c, u, op.name, v)
			}
		}
		Errorf("binary %s not implemented on type %s", op.name, which)
	}
	return fn(c, u, v)
}

// Product computes a compound product, such as an inner product
// "+.*" or outer product "o.*". The op is known to contain a
// period. The operands are all at least vectors, and for inner product
// they must both be vectors.
func Product(c Context, u Value, op string, v Value) Value {
	dot := strings.IndexByte(op, '.')
	left := op[:dot]
	right := op[dot+1:]
	which := atLeastVectorType(whichType(u), whichType(v))
	u = u.toType(c.Config(), which)
	v = v.toType(c.Config(), which)
	if left == "o" {
		return outerProduct(c, u, right, v)
	}
	return innerProduct(c, u, left, right, v)
}

// inner product computes an inner product such as "+.*".
// u and v are known to be the same type and at least Vectors.
func innerProduct(c Context, u Value, left, right string, v Value) Value {
	switch u := u.(type) {
	case Vector:
		v := v.(Vector)
		u.sameLength(v)
		var x Value
		for k, e := range u {
			tmp := c.EvalBinary(e, right, v[k])
			if k == 0 {
				x = tmp
			} else {
				x = c.EvalBinary(x, left, tmp)
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
			acc := c.EvalBinary(u.data[row*ucols], right, v.data[col])
			for j := 1; j < ucols; j++ {
				acc = c.EvalBinary(acc, left, c.EvalBinary(u.data[row*ucols+j], right, v.data[j*vcols+col]))
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

// outer product computes an outer product such as "o.*".
// u and v are known to be at least Vectors.
func outerProduct(c Context, u Value, op string, v Value) Value {
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
				m.data[index] = c.EvalBinary(vu, op, vv)
				index++
			}
		}
		return m // TODO: Shrink?
	case Matrix:
		v := v.(Matrix)
		m := Matrix{
			shape: NewVector(append(u.Shape(), v.Shape()...)),
			data:  NewVector(make(Vector, len(u.Data())*len(v.Data()))),
		}
		index := 0
		for _, vu := range u.Data() {
			for _, vv := range v.Data() {
				m.data[index] = c.EvalBinary(vu, op, vv)
				index++
			}
		}
		return m // TODO: Shrink?
	}
	Errorf("can't do outer product on %s", whichType(u))
	panic("not reached")
}

// Reduce computes a reduction such as +/. The slash has been removed.
func Reduce(c Context, op string, v Value) Value {
	// We must be right associative; that is the grammar.
	// -/1 2 3 == 1-2-3 is 1-(2-3) not (1-2)-3. Answer: 2.
	switch v := v.(type) {
	case Int, BigInt, BigRat:
		return v
	case Vector:
		if len(v) == 0 {
			return v
		}
		acc := v[len(v)-1]
		for i := len(v) - 2; i >= 0; i-- {
			acc = c.EvalBinary(v[i], op, acc)
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
				acc = c.EvalBinary(v.data[pos], op, acc)
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

// Scan computes a scan of the op; the \ has been removed.
// It gives the successive values of reducing op through v.
// We must be right associative; that is the grammar.
func Scan(c Context, op string, v Value) Value {
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
			values[i] = Reduce(c, op, v[:i+1])
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
				data[index+j] = Reduce(c, op, v.data[index:index+j+1])
			}
			index += stride
		}
		return NewMatrix(v.shape, data)
	}
	Errorf("can't do scan on %s", whichType(v))
	panic("not reached")
}

// unaryVectorOp applies op elementwise to i.
func unaryVectorOp(c Context, op string, i Value) Value {
	u := i.(Vector)
	n := make([]Value, len(u))
	for k := range u {
		n[k] = c.EvalUnary(op, u[k])
	}
	return NewVector(n)
}

// unaryMatrixOp applies op elementwise to i.
func unaryMatrixOp(c Context, op string, i Value) Value {
	u := i.(Matrix)
	n := make([]Value, len(u.data))
	for k := range u.data {
		n[k] = c.EvalUnary(op, u.data[k])
	}
	return NewMatrix(u.shape, NewVector(n))
}

// binaryVectorOp applies op elementwise to i and j.
func binaryVectorOp(c Context, i Value, op string, j Value) Value {
	u, v := i.(Vector), j.(Vector)
	if len(u) == 1 {
		n := make([]Value, len(v))
		for k := range v {
			n[k] = c.EvalBinary(u[0], op, v[k])
		}
		return NewVector(n)
	}
	if len(v) == 1 {
		n := make([]Value, len(u))
		for k := range u {
			n[k] = c.EvalBinary(u[k], op, v[0])
		}
		return NewVector(n)
	}
	u.sameLength(v)
	n := make([]Value, len(u))
	for k := range u {
		n[k] = c.EvalBinary(u[k], op, v[k])
	}
	return NewVector(n)
}

// binaryMatrixOp applies op elementwise to i and j.
func binaryMatrixOp(c Context, i Value, op string, j Value) Value {
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
			n[k] = c.EvalBinary(u.data[0], op, v.data[k])
		}
	case isScalar(v):
		// Matrix op Scalar.
		n = make([]Value, len(u.data))
		for k := range u.data {
			n[k] = c.EvalBinary(u.data[k], op, v.data[0])
		}
	case isVector(u, v.shape):
		// Vector op Matrix.
		shape = v.shape
		n = make([]Value, len(v.data))
		dim := int(u.shape[0].(Int))
		index := 0
		for k := range v.data {
			n[k] = c.EvalBinary(u.data[index], op, v.data[k])
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
			n[k] = c.EvalBinary(v.data[index], op, u.data[k])
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
			n[k] = c.EvalBinary(u.data[k], op, v.data[k])
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
