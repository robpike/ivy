// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"math/big"
	"runtime"
	"strings"
)

type valueType int

const (
	intType valueType = iota
	charType
	bigIntType
	bigRatType
	bigFloatType
	complexType
	vectorType
	matrixType
	numType
)

var typeName = [...]string{"int", "char", "big int", "rational", "float", "complex", "vector", "matrix"}

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
	case Complex:
		return complexType
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

// safeBinary reports whether the binary operator op is safe to parallelize.
func safeBinary(op string) bool {
	// ? uses the random number generator,
	// which maintains global state.
	return BinaryOps[op] != nil && op != "?"
}

// safeUnary reports whether the unary operator op is safe to parallelize.
func safeUnary(op string) bool {
	// ? uses the random number generator,
	// which maintains global state.
	return UnaryOps[op] != nil && op != "?"
}

// knownAssoc reports whether the binary op is known to be associative.
func knownAssoc(op string) bool {
	switch op {
	case "+", "*", "min", "max", "or", "and", "xor", "|", "&", "^":
		return true
	}
	return false
}

var pforMinWork = 100

func MaxParallelismForTesting() {
	pforMinWork = 1
}

// pfor is a conditionally parallel for loop from 0 to n.
// If ok is true and the work is big enough,
// pfor calls f(lo, hi) for ranges [lo, hi) that collectively tile [0, n)
// and for which (hi-lo)*size is at least roughly pforMinWork.
// Otherwise, pfor calls f(0, n).
func pfor(ok bool, size, n int, f func(lo, hi int)) {
	var p int
	if ok {
		p = runtime.GOMAXPROCS(-1)
		if p == 1 || n <= 1 || n*size < pforMinWork*2 {
			ok = false
		}
	}
	if !ok {
		f(0, n)
		return
	}
	p *= 4 // evens out lopsided work splits
	if q := n * size / pforMinWork; q < p {
		p = q
	}
	c := make(chan interface{}, p)
	for i := 0; i < p; i++ {
		lo, hi := i*n/p, (i+1)*n/p
		go func() {
			defer sendRecover(c)
			f(lo, hi)
		}()
	}
	var err interface{}
	for i := 0; i < p; i++ {
		if e := <-c; e != nil {
			err = e
		}
	}
	if err != nil {
		panic(err)
	}
}

func sendRecover(c chan<- interface{}) {
	c <- recover()
}

// inner product computes an inner product such as "+.*".
// u and v are known to be the same type and at least Vectors.
func innerProduct(c Context, u Value, left, right string, v Value) Value {
	switch u := u.(type) {
	case Vector:
		v := v.(Vector)
		u.sameLength(v)
		n := len(u)
		if n == 0 {
			Errorf("empty inner product")
		}
		x := c.EvalBinary(u[n-1], right, v[n-1])
		for k := n - 2; k >= 0; k-- {
			x = c.EvalBinary(c.EvalBinary(u[k], right, v[k]), left, x)
		}
		return x
	case *Matrix:
		// Say we're doing +.*
		// result[i,j] = +/(u[row i] * v[column j])
		// Number of columns of u must be the number of rows of v: (-1 take rho u) == (1 take rho v)
		// The result is has shape (-1 drop rho u), (1 drop rho v)
		v := v.(*Matrix)
		if u.Rank() < 1 || v.Rank() < 1 || u.shape[len(u.shape)-1] != v.shape[0] {
			Errorf("inner product: mismatched shapes %s and %s", NewIntVector(u.shape), NewIntVector(v.shape))
		}
		n := v.shape[0]
		vstride := len(v.data) / n
		data := make(Vector, len(u.data)/n*vstride)
		pfor(safeBinary(left) && safeBinary(right), 1, len(data), func(lo, hi int) {
			for x := lo; x < hi; x++ {
				i := x / vstride * n
				j := x % vstride
				acc := c.EvalBinary(u.data[i+n-1], right, v.data[j+(n-1)*vstride])
				for k := n - 2; k >= 0; k-- {
					acc = c.EvalBinary(c.EvalBinary(u.data[i+k], right, v.data[j+k*vstride]), left, acc)
				}
				data[x] = acc
			}
		})
		rank := len(u.shape) + len(v.shape) - 2
		if rank == 1 {
			return data
		}
		shape := make([]int, rank)
		copy(shape, u.shape[:len(u.shape)-1])
		copy(shape[len(u.shape)-1:], v.shape[1:])
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
		pfor(safeBinary(op), 1, len(m.data), func(lo, hi int) {
			for x := lo; x < hi; x++ {
				m.data[x] = c.EvalBinary(u[x/len(v)], op, v[x%len(v)])
			}
		})
		return &m // TODO: Shrink?
	case *Matrix:
		v := v.(*Matrix)
		m := Matrix{
			shape: append(u.Shape(), v.Shape()...),
			data:  NewVector(make(Vector, len(u.Data())*len(v.Data()))),
		}
		vdata := v.Data()
		udata := u.Data()
		pfor(safeBinary(op), 1, len(m.data), func(lo, hi int) {
			for x := lo; x < hi; x++ {
				m.data[x] = c.EvalBinary(udata[x/len(vdata)], op, vdata[x%len(vdata)])
			}
		})
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
	case Int, BigInt, BigRat, BigFloat, Complex:
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
		pfor(safeBinary(op), stride, len(data), func(lo, hi int) {
			for i := lo; i < hi; i++ {
				index := stride * i
				pos := index + stride - 1
				acc := v.data[pos]
				pos--
				for j := 1; j < stride; j++ {
					acc = c.EvalBinary(v.data[pos], op, acc)
					pos--
				}
				data[i] = acc
			}
		})
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
	case Int, BigInt, BigRat, BigFloat, Complex:
		return v
	case Vector:
		if len(v) == 0 {
			return v
		}
		values := make(Vector, len(v))
		// This is fundamentally O(n²) in the general case.
		// We make it O(n) for known associative ops.
		values[0] = v[0]
		if knownAssoc(op) {
			for i := 1; i < len(v); i++ {
				values[i] = c.EvalBinary(values[i-1], op, v[i])
			}
		} else {
			for i := 1; i < len(v); i++ {
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
		nrows := 1
		for i := 0; i < v.Rank()-1; i++ {
			// Guaranteed by NewMatrix not to overflow.
			nrows *= v.shape[i]
		}
		pfor(safeBinary(op), stride, nrows, func(lo, hi int) {
			for i := lo; i < hi; i++ {
				index := i * stride
				// This is fundamentally O(n²) in the general case.
				// We make it O(n) for known associative ops.
				data[index] = v.data[index]
				if knownAssoc(op) {
					for j := 1; j < stride; j++ {
						data[index+j] = c.EvalBinary(data[index+j-1], op, v.data[index+j])
					}
				} else {
					for j := 1; j < stride; j++ {
						data[index+j] = Reduce(c, op, v.data[index:index+j+1])
					}
				}
			}
		})
		return NewMatrix(v.shape, data)
	}
	Errorf("can't do scan on %s", whichType(v))
	panic("not reached")
}

// unaryVectorOp applies op elementwise to i.
func unaryVectorOp(c Context, op string, i Value) Value {
	u := i.(Vector)
	n := make([]Value, len(u))
	pfor(safeUnary(op), 1, len(n), func(lo, hi int) {
		for k := lo; k < hi; k++ {
			n[k] = c.EvalUnary(op, u[k])
		}
	})
	return NewVector(n)
}

// unaryMatrixOp applies op elementwise to i.
func unaryMatrixOp(c Context, op string, i Value) Value {
	u := i.(*Matrix)
	n := make([]Value, len(u.data))
	pfor(safeUnary(op), 1, len(n), func(lo, hi int) {
		for k := lo; k < hi; k++ {
			n[k] = c.EvalUnary(op, u.data[k])
		}
	})
	return NewMatrix(u.shape, NewVector(n))
}

// binaryVectorOp applies op elementwise to i and j.
func binaryVectorOp(c Context, i Value, op string, j Value) Value {
	u, v := i.(Vector), j.(Vector)
	if len(u) == 1 {
		n := make([]Value, len(v))
		pfor(safeBinary(op), 1, len(n), func(lo, hi int) {
			for k := lo; k < hi; k++ {
				n[k] = c.EvalBinary(u[0], op, v[k])
			}
		})
		return NewVector(n)
	}
	if len(v) == 1 {
		n := make([]Value, len(u))
		pfor(safeBinary(op), 1, len(n), func(lo, hi int) {
			for k := lo; k < hi; k++ {
				n[k] = c.EvalBinary(u[k], op, v[0])
			}
		})
		return NewVector(n)
	}
	u.sameLength(v)
	n := make([]Value, len(u))
	pfor(safeBinary(op), 1, len(n), func(lo, hi int) {
		for k := lo; k < hi; k++ {
			n[k] = c.EvalBinary(u[k], op, v[k])
		}
	})
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
		pfor(safeBinary(op), 1, len(n), func(lo, hi int) {
			for k := lo; k < hi; k++ {
				n[k] = c.EvalBinary(u.data[0], op, v.data[k])
			}
		})
	case isScalar(v):
		// Matrix op Scalar.
		n = make([]Value, len(u.data))
		pfor(safeBinary(op), 1, len(n), func(lo, hi int) {
			for k := lo; k < hi; k++ {
				n[k] = c.EvalBinary(u.data[k], op, v.data[0])
			}
		})
	case isVector(u, v.shape):
		// Vector op Matrix.
		shape = v.shape
		n = make([]Value, len(v.data))
		dim := u.shape[0]
		pfor(safeBinary(op), 1, len(n), func(lo, hi int) {
			for k := lo; k < hi; k++ {
				n[k] = c.EvalBinary(u.data[k%dim], op, v.data[k])
			}
		})
	case isVector(v, u.shape):
		// Matrix op Vector.
		n = make([]Value, len(u.data))
		dim := v.shape[0]
		pfor(safeBinary(op), 1, len(n), func(lo, hi int) {
			for k := lo; k < hi; k++ {
				n[k] = c.EvalBinary(u.data[k], op, v.data[k%dim])
			}
		})
	default:
		// Matrix op Matrix.
		u.sameShape(v)
		n = make([]Value, len(u.data))
		pfor(safeBinary(op), 1, len(n), func(lo, hi int) {
			for k := lo; k < hi; k++ {
				n[k] = c.EvalBinary(u.data[k], op, v.data[k])
			}
		})
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

// isZero reports whether u is a numeric zero.
func isZero(v Value) bool {
	switch v := v.(type) {
	case Int:
		return v == 0
	case BigInt:
		return v.Sign() == 0
	case BigRat:
		return v.Sign() == 0
	case BigFloat:
		return v.Sign() == 0
	case Complex:
		return isZero(v.real) && isZero(v.imag)
	}
	return false
}

// isNegative reports whether u is negative
func isNegative(v Value) bool {
	switch v := v.(type) {
	case Int:
		return v < 0
	case BigInt:
		return v.Sign() < 0
	case BigRat:
		return v.Sign() < 0
	case BigFloat:
		return v.Sign() < 0
	case Complex:
		return false
	}
	return false
}

// compare returns -1, 0, 1 according to whether v is less than,
// equal to, or greater than i.
func compare(v Value, i int) int {
	switch v := v.(type) {
	case Int:
		i := Int(i)
		switch {
		case v < i:
			return -1
		case v == i:
			return 0
		}
		return 1
	case BigInt:
		r := big.NewInt(int64(i))
		return -r.Sub(r, v.Int).Sign()
	case BigRat:
		r := big.NewRat(int64(i), 1)
		return -r.Sub(r, v.Rat).Sign()
	case BigFloat:
		r := big.NewFloat(float64(i))
		return -r.Sub(r, v.Float).Sign()
	case Complex:
		return -1
	}
	return -1
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
	case Complex:
		return !isZero(v)
	default:
		Errorf("invalid expression %s for conditional inside %q", v, fnName)
		return false
	}
}

// emod is a restricted form of Euclidean integer modulus.
// Used by encode, and only works for integers.
func emod(op string, c Context, a, b Value) Value {
	if z, ok := b.(Int); ok && z == 0 {
		return a
	}
	aa := a.toType(op, c.Config(), bigIntType)
	bb := b.toType(op, c.Config(), bigIntType)
	return binaryBigIntOp(aa, (*big.Int).Mod, bb)
}

// ediv is a restricted form of Euclidean integer division.
// Used by encode, and only works for integers.
func ediv(op string, c Context, a, b Value) Value {
	if z, ok := b.(Int); ok && z == 0 {
		return a
	}
	aa := a.toType(op, c.Config(), bigIntType)
	bb := b.toType(op, c.Config(), bigIntType)
	return binaryBigIntOp(aa, (*big.Int).Div, bb)
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
