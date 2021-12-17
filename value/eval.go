// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
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
	whichType   func(a, b valueType) (valueType, valueType)
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
	case *Matrix:
		return matrixType
	}
	Errorf("unknown type %T in whichType", v)
	panic("which type")
}

func (op *binaryOp) EvalBinary(c Context, u, v Value) Value {
	if op.whichType == nil {
		// At the moment, "text" is the only operator that leaves
		// both arg types alone. Perhaps more will arrive.
		if op.name != "text" {
			Errorf("internal error: nil whichType")
		}
		return op.fn[0](c, u, v)
	}
	whichU, whichV := op.whichType(whichType(u), whichType(v))
	conf := c.Config()
	u = u.toType(op.name, conf, whichU)
	v = v.toType(op.name, conf, whichV)
	fn := op.fn[whichV]
	if fn == nil {
		if op.elementwise {
			switch whichV {
			case vectorType:
				return binaryVectorOp(c, u, op.name, v)
			case matrixType:
				return binaryMatrixOp(c, u, op.name, v)
			}
		}
		Errorf("binary %s not implemented on type %s", op.name, whichV)
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
	which, _ := atLeastVectorType(whichType(u), whichType(v))
	u = u.toType(op, c.Config(), which)
	v = v.toType(op, c.Config(), which)
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
	case *Matrix:
		// Say we're doing +.*
		// result[i,j] = +/(u[row i] * v[column j])
		// Number of columns of u must be the number of rows of v.
		// The result is has shape (urows, vcols).
		v := v.(*Matrix)
		if u.Rank() != 2 || v.Rank() != 2 {
			Errorf("can't do inner product on shape %s times %s", NewIntVector(u.shape), NewIntVector(v.shape))
		}
		urows := u.shape[0]
		ucols := u.shape[1]
		vrows := v.shape[0]
		vcols := v.shape[1]
		if vrows != ucols {
			Errorf("inner product; column count of left (%d) not equal to row count on right (%d)", ucols, vrows)
		}
		data := make(Vector, urows*vcols)
		shape := []int{urows, vcols}
		i := 0
		for urow := 0; urow < urows; urow++ {
			for vcol := 0; vcol < vcols; vcol++ {
				acc := c.EvalBinary(u.data[urow*ucols], right, v.data[vcol])
				for vrow := 1; vrow < vrows; vrow++ {
					acc = c.EvalBinary(acc, left, c.EvalBinary(u.data[urow*ucols+vrow], right, v.data[vrow*vcols+vcol]))
				}
				data[i] = acc
				i++
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
			shape: []int{len(u), len(v)},
			data:  NewVector(make(Vector, len(u)*len(v))),
		}
		index := 0
		for _, vu := range u {
			for _, vv := range v {
				m.data[index] = c.EvalBinary(vu, op, vv)
				index++
			}
		}
		return &m // TODO: Shrink?
	case *Matrix:
		v := v.(*Matrix)
		m := Matrix{
			shape: append(u.Shape(), v.Shape()...),
			data:  NewVector(make(Vector, len(u.Data())*len(v.Data()))),
		}
		index := 0
		for _, vu := range u.Data() {
			for _, vv := range v.Data() {
				m.data[index] = c.EvalBinary(vu, op, vv)
				index++
			}
		}
		return &m // TODO: Shrink?
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
	case *Matrix:
		if v.Rank() < 2 {
			Errorf("shape for matrix is degenerate: %s", NewIntVector(v.shape))
		}
		stride := v.shape[v.Rank()-1]
		if stride == 0 {
			Errorf("shape for matrix is degenerate: %s", NewIntVector(v.shape))
		}
		shape := v.shape[:v.Rank()-1]
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
		// TODO: For some operations this is n^2.
		values[0] = v[0]
		for i := 1; i < len(v); i++ {
			if fastOp := fastScanOp(op, i); fastOp != "" {
				values[i] = c.EvalBinary(values[i-1], fastOp, v[i])
			} else {
				values[i] = Reduce(c, op, v[:i+1])
			}
		}
		return NewVector(values)
	case *Matrix:
		if v.Rank() < 2 {
			Errorf("shape for matrix is degenerate: %s", NewIntVector(v.shape))
		}
		stride := v.shape[v.Rank()-1]
		if stride == 0 {
			Errorf("shape for matrix is degenerate: %s", NewIntVector(v.shape))
		}
		data := make(Vector, len(v.data))
		index := 0
		nrows := 1
		for i := 0; i < v.Rank()-1; i++ {
			// Guaranteed by NewMatrix not to overflow.
			nrows *= v.shape[i]
		}
		for i := 0; i < nrows; i++ {
			data[index] = v.data[index]
			// TODO: For some operations this is n^2.
			for j := 1; j < stride; j++ {
				if fastOp := fastScanOp(op, j); fastOp != "" {
					data[index+j] = c.EvalBinary(data[index+j-1], fastOp, v.data[index+j])
				} else {
					data[index+j] = Reduce(c, op, v.data[index:index+j+1])
				}
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
	u := i.(*Matrix)
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
	u, v := i.(*Matrix), j.(*Matrix)
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
		dim := u.shape[0]
		index := 0
		for k := range v.data {
			n[k] = c.EvalBinary(u.data[index], op, v.data[k])
			index++
			if index >= dim {
				index = 0
			}
		}
	case isVector(v, u.shape):
		// Matrix op Vector.
		n = make([]Value, len(u.data))
		dim := v.shape[0]
		index := 0
		for k := range u.data {
			n[k] = c.EvalBinary(u.data[k], op, v.data[index])
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
func isScalar(u *Matrix) bool {
	for _, dim := range u.shape {
		if dim != 1 {
			return false
		}
	}
	return true
}

// isVector reports whether u is an 1x1x...xn item where n is the last dimension
// of the shape, that is, an n-vector promoted to matrix.
func isVector(u *Matrix, shape []int) bool {
	if u.Rank() == 0 || len(shape) == 0 || u.shape[0] != shape[len(shape)-1] {
		return false
	}
	for _, dim := range u.shape[1:] {
		if dim != 1 {
			return false
		}
	}
	return true
}

// EvalFunctionBody evaluates the list of expressions inside a function,
// possibly with conditionals that generate an early return.
func EvalFunctionBody(context Context, fnName string, body []Expr) Value {
	var v Value
	for _, e := range body {
		if d, ok := e.(Decomposable); ok && d.Operator() == ":" {
			left, right := d.Operands()
			if isTrue(fnName, left.Eval(context)) {
				return right.Eval(context)
			}
			continue
		}
		v = e.Eval(context)
	}
	return v
}

// isTrue reports whether v represents boolean truth. If v is not
// a scalar, an error results.
func isTrue(fnName string, v Value) bool {
	switch i := v.(type) {
	case Char:
		return i != 0
	case Int:
		return i != 0
	case BigInt:
		return true // If it's a BigInt, it can't be 0 - that's an Int.
	case BigRat:
		return true // If it's a BigRat, it can't be 0 - that's an Int.
	case BigFloat:
		return i.Float.Sign() != 0
	default:
		Errorf("invalid expression %s for conditional inside %q", v, fnName)
		return false
	}
}

// fastScanOp returns the appropriate scan op, according to it, the iteration
// variable. When no fast scan operation is available, an empty string is
// returned. Fast scan op is available for all associative ops and for some
// inassociative ops (such as "-" and "/").
func fastScanOp(op string, it int) string {
	switch op {
	case "+", "-", "*", "/", ",", "|", "&", "^", "or", "and", "xor":
	default:
		// All the rest of the ops are not optimized, and need full reduction.
		return ""
	}
	if it%2 == 0 {
		switch op {
		case "-":
			return "+"
		case "/":
			return "*"
		}
	}
	return op
}
