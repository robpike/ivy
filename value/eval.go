// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"
	"iter"
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

// TraceUnary prints a trace line for a unary operator.
func TraceUnary(c Context, level int, op string, v Value) {
	if c.Config().Tracing(level) {
		fmt.Fprintf(c.Config().ErrOutput(), "\t%s> %s %s\n", c.TraceIndent(), op, v)
	}
}

// TraceBinary prints a trace line for a binary operator.
func TraceBinary(c Context, level int, u Value, op string, v Value) {
	if c.Config().Tracing(level) {
		fmt.Fprintf(c.Config().ErrOutput(), "\t%s> %s %s %s\n", c.TraceIndent(), u, op, v)
	}
}

func (op *unaryOp) EvalUnary(c Context, v Value) Value {
	which := whichType(c, v)
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
		c.Errorf("unary %s not implemented on type %s", op.name, which)
	}
	if c.Config().Tracing(2) {
		fmt.Printf("\t%s> %s %s\n", c.TraceIndent(), op.name, v)
	}
	return fn(c, v)
}

type binaryFn func(Context, Value, Value) Value

type binaryOp struct {
	name        string
	elementwise bool // whether the operation applies elementwise to vectors and matrices
	whichType   func(c Context, a, b valueType) (valueType, valueType)
	fn          [numType]binaryFn
}

func whichType(c Context, v Value) valueType {
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
	case *Vector:
		return vectorType
	case *Matrix:
		return matrixType
	}
	c.Errorf("internal error: unknown type %T in whichType", v)
	panic("which type")
}

func (op *binaryOp) EvalBinary(c Context, u, v Value) Value {
	whichU, whichV := op.whichType(c, whichType(c, u), whichType(c, v))
	conf := c.Config()
	u = u.toType(op.name, c, whichU)
	v = v.toType(op.name, c, whichV)
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
		c.Errorf("binary %s not implemented on type %s", op.name, whichV)
	}
	if conf.Tracing(2) {
		fmt.Printf("\t%s> %s %s %s\n", c.TraceIndent(), u, op.name, v)
	}
	return fn(c, u, v)
}

// EvalCharEqual handles == and != in a special case:
// If comparing a scalar against a Char, avoid the conversion.
// The logic of type promotion in EvalBinary otherwise interferes with comparison
// because it tries to force scalar types to be the same, and char doesn't convert to
// any other type.
func EvalCharEqual(c Context, u Value, isEqualOp bool, v Value) (Value, bool) {
	uType, vType := whichType(c, u), whichType(c, v)
	if uType != vType && uType < vectorType && vType < vectorType {
		// Two different scalar types. If either is char, we know the answer now.
		if uType == charType || vType == charType {
			if isEqualOp {
				return zero, true
			}
			return one, true
		}
	}
	return nil, false
}

// Product computes a compound product, such as an inner product
// "+.*" or outer product "o.*". The op is known to contain a
// period. The operands are all at least vectors, and for inner product
// they must both be vectors.
func Product(c Context, u Value, op string, v Value) Value {
	dot := strings.IndexByte(op, '.')
	left := op[:dot]
	right := op[dot+1:]
	which, _ := atLeastVectorType(c, whichType(c, u), whichType(c, v))
	u = u.toType(op, c, which)
	v = v.toType(op, c, which)
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
	case *Vector:
		v := v.(*Vector)
		u.sameLength(c, v)
		n := u.Len()
		if n == 0 {
			c.Errorf("empty inner product")
		}
		x := c.EvalBinary(u.At(n-1), right, v.At(n-1))
		for k := n - 2; k >= 0; k-- {
			x = c.EvalBinary(c.EvalBinary(u.At(k), right, v.At(k)), left, x)
		}
		return x
	case *Matrix:
		// Say we're doing +.*
		// result[i,j] = +/(u[row i] * v[column j])
		// Number of columns of u must be the number of rows of v: (-1 take rho u) == (1 take rho v)
		// The result has shape (-1 drop rho u), (1 drop rho v)
		v := v.(*Matrix)
		if u.Rank() < 1 || v.Rank() < 1 || u.shape[len(u.shape)-1] != v.shape[0] {
			c.Errorf("inner product: mismatched shapes %s and %s", NewIntVector(u.shape...), NewIntVector(v.shape...))
		}
		n := v.shape[0]
		vstride := v.data.Len() / n
		data := newVectorEditor(u.data.Len()/n*vstride, nil)
		pfor(safeBinary(left) && safeBinary(right), 1, data.Len(), func(lo, hi int) {
			for x := lo; x < hi; x++ {
				i := x / vstride * n
				j := x % vstride
				acc := c.EvalBinary(u.data.At(i+n-1), right, v.data.At(j+(n-1)*vstride))
				for k := n - 2; k >= 0; k-- {
					acc = c.EvalBinary(c.EvalBinary(u.data.At(i+k), right, v.data.At(j+k*vstride)), left, acc)
				}
				data.Set(x, acc)
			}
		})
		rank := len(u.shape) + len(v.shape) - 2
		if rank == 1 {
			return data.Publish()
		}
		shape := make([]int, rank)
		copy(shape, u.shape[:len(u.shape)-1])
		copy(shape[len(u.shape)-1:], v.shape[1:])
		return NewMatrix(c, shape, data.Publish())
	}
	c.Errorf("can't do inner product on %s", whichType(c, u))
	panic("not reached")
}

// outer product computes an outer product such as "o.*".
// u and v are known to be at least Vectors.
func outerProduct(c Context, u Value, op string, v Value) Value {
	switch u := u.(type) {
	case *Vector:
		v := v.(*Vector)
		data := newVectorEditor(u.Len()*v.Len(), nil)
		pfor(safeBinary(op), 1, data.Len(), func(lo, hi int) {
			for x := lo; x < hi; x++ {
				data.Set(x, c.EvalBinary(u.At(x/v.Len()), op, v.At(x%v.Len())))
			}
		})
		return NewMatrix(c, []int{u.Len(), v.Len()}, data.Publish())
	case *Matrix:
		v := v.(*Matrix)
		udata := u.Data()
		vdata := v.Data()
		data := newVectorEditor(udata.Len()*vdata.Len(), nil)
		pfor(safeBinary(op), 1, data.Len(), func(lo, hi int) {
			for x := lo; x < hi; x++ {
				data.Set(x, c.EvalBinary(udata.At(x/vdata.Len()), op, vdata.At(x%vdata.Len())))
			}
		})
		return NewMatrix(c, append(u.Shape(), v.Shape()...), data.Publish())
	}
	c.Errorf("can't do outer product on %s", whichType(c, u))
	panic("not reached")
}

// Reduce computes a reduction such as +/. The slash has been removed.
func Reduce(c Context, op string, v Value) Value {
	// We must be right associative; that is the grammar.
	// -/1 2 3 == 1-2-3 is 1-(2-3) not (1-2)-3. Answer: 2.
	switch v := v.(type) {
	case Int, BigInt, BigRat, BigFloat, Complex:
		return v
	case *Vector:
		if v.Len() == 0 {
			return v
		}
		acc := v.At(v.Len() - 1)
		for i := v.Len() - 2; i >= 0; i-- {
			acc = c.EvalBinary(v.At(i), op, acc)
		}
		return acc
	case *Matrix:
		if v.Rank() < 2 {
			degenerate(c, v)
		}
		stride := v.shape[v.Rank()-1]
		if stride == 0 {
			degenerate(c, v)
		}
		shape := v.shape[:v.Rank()-1]
		data := newVectorEditor(size(c, shape), nil)
		pfor(safeBinary(op), stride, data.Len(), func(lo, hi int) {
			for i := lo; i < hi; i++ {
				index := stride * i
				pos := index + stride - 1
				acc := v.data.At(pos)
				pos--
				for j := 1; j < stride; j++ {
					acc = c.EvalBinary(v.data.At(pos), op, acc)
					pos--
				}
				data.Set(i, acc)
			}
		})
		if len(shape) == 1 {
			return data.Publish()
		}
		return NewMatrix(c, shape, data.Publish())
	}
	c.Errorf("can't do reduce on %s", whichType(c, v))
	panic("not reached")
}

func degenerate(c Context, m *Matrix) {
	c.Errorf("shape for matrix is degenerate: %s", NewIntVector(m.shape...))
}

// ReduceFirst computes a reduction such as +/% along
// the first axis. The slash-percent has been removed.
func ReduceFirst(c Context, op string, v Value) Value {
	// We must be right associative; that is the grammar.
	// -/1 2 3 == 1-2-3 is 1-(2-3) not (1-2)-3. Answer: 2.
	m, ok := v.(*Matrix)
	if !ok {
		// Same as regular reduce.
		return Reduce(c, op, v)
	}
	if v.Rank() < 2 {
		degenerate(c, m)
	}
	if m.shape[0] == 0 {
		degenerate(c, m)
	}
	stride := size(c, m.shape[1:m.Rank()])
	if stride == 0 {
		degenerate(c, m)
	}
	shape := m.shape[1:m.Rank()]
	data := newVectorEditor(size(c, shape), nil)
	pfor(safeBinary(op), stride, data.Len(), func(lo, hi int) {
		for i := lo; i < hi; i++ {
			pos := i + m.data.Len() - stride
			acc := m.data.At(pos)
			for j := pos - stride; j >= 0; j -= stride {
				acc = c.EvalBinary(m.data.At(j), op, acc)
			}
			data.Set(i, acc)
		}
	})
	if len(shape) == 1 { // TODO: Matrix.shrink()?
		return data.Publish()
	}
	return NewMatrix(c, shape, data.Publish())
}

// Scan computes a scan of the op; the \ has been removed.
// It gives the successive values of reducing op through v.
// We must be right associative; that is the grammar.
func Scan(c Context, op string, v Value) Value {
	switch v := v.(type) {
	case Int, BigInt, BigRat, BigFloat, Complex:
		return v
	case *Vector:
		if v.Len() == 0 {
			return v
		}
		values := newVectorEditor(v.Len(), nil)
		// This is fundamentally O(n²) in the general case.
		// We make it O(n) for known associative ops.
		values.Set(0, v.At(0))
		if knownAssoc(op) {
			for i := 1; i < v.Len(); i++ {
				values.Set(i, c.EvalBinary(values.At(i-1), op, v.At(i)))
			}
		} else {
			for i := 1; i < v.Len(); i++ {
				values.Set(i, Reduce(c, op, NewVectorSeq(v.Slice(0, i+1))))
			}
		}
		return values.Publish()
	case *Matrix:
		if v.Rank() < 2 {
			degenerate(c, v)
		}
		stride := v.shape[v.Rank()-1]
		if stride == 0 {
			degenerate(c, v)
		}
		data := newVectorEditor(v.data.Len(), nil)
		nrows := size(c, v.shape[:len(v.shape)-1])
		pfor(safeBinary(op), stride, nrows, func(lo, hi int) {
			for i := lo; i < hi; i++ {
				index := i * stride
				// This is fundamentally O(n²) in the general case.
				// We make it O(n) for known associative ops.
				data.Set(index, v.data.At(index))
				if knownAssoc(op) {
					for j := 1; j < stride; j++ {
						data.Set(index+j, c.EvalBinary(data.At(index+j-1), op, v.data.At(index+j)))
					}
				} else {
					for j := 1; j < stride; j++ {
						data.Set(index+j, Reduce(c, op, NewVectorSeq(v.data.Slice(index, index+j+1))))
					}
				}
			}
		})
		return NewMatrix(c, v.shape, data.Publish())
	}
	c.Errorf("can't do scan on %s", whichType(c, v))
	panic("not reached")
}

// ScanFirst computes a scan of the op along the first axis.
// The backslash-percent has been removed.
// It gives the successive values of reducing op through v.
// We must be right associative; that is the grammar.
func ScanFirst(c Context, op string, v Value) Value {
	m, ok := v.(*Matrix)
	if !ok {
		// Same as regular reduce.
		return Scan(c, op, v)
	}
	if m.Rank() < 2 {
		degenerate(c, m)
	}
	stride := m.shape[len(m.shape)-1]
	if stride == 0 {
		degenerate(c, m)
	}
	// Simple but effective algorithm: Transpose twice. Better than one might
	// think because transposition is O(size of matrix) and it also lines up
	// the scan in memory order.
	// TODO: Is it worth doing the ugly non-transpose bookkeeping?
	m = Scan(c, op, m.transpose(c)).(*Matrix)
	return m.transpose(c)

}

// dataShape returns the data shape of v.
// The data shape of a scalar is []int{}, to distinguish from a vector of length 1.
func dataShape(v Value) []int {
	switch v := v.(type) {
	case *Vector:
		return []int{v.Len()}
	case *Matrix:
		return v.Shape()
	}
	return []int{}
}

// eachOne returns an iterator that yields v once.
func eachOne(v Value) iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(v)
	}
}

// eachVector returns an iterator that yields each element of v.
func eachVector(v *Vector) iter.Seq[Value] {
	return func(yield func(Value) bool) {
		for _, x := range v.All() {
			if !yield(x) {
				break
			}
		}
	}
}

// eachMatrix returns an iterator that yields subparts of m,
// iterating over the first dim dimensions of m.
// If dim == len(m.shape), the iterator yields each value in m.
// If dim == len(m.shape)-1, the iterator yields each innermost row of m.
// Otherwise the iterator yields each submatrix obtained by indexing
// the first dim dimensions of m.
func eachMatrix(c Context, m *Matrix, dim int) iter.Seq[Value] {
	if dim == len(m.shape) {
		return eachVector(m.data)
	}
	size := m.data.Len()
	if size > 0 {
		for d := 0; d < dim; d++ {
			size /= m.shape[d]
		}
	}
	return func(yield func(Value) bool) {
		for i := 0; i < m.data.Len(); i += size {
			var v Value
			if dim == len(m.shape)-1 {
				v = NewVectorSeq(m.data.Slice(i, i+size))
			} else {
				v = NewMatrix(c, m.shape[dim:], NewVectorSeq(m.data.Slice(i, i+size)))
			}
			if !yield(v) {
				break
			}
		}
	}
}

// eachAny returns an iterator that yields subparts of v
// iterating over the first dim dimensions of v.
// The caller has checked that dim is in range for v.
func eachValue(c Context, v Value, dim int) iter.Seq[Value] {
	if dim == 0 {
		return eachOne(v)
	}
	switch v := v.(type) {
	default:
		return eachOne(v)
	case QuietValue:
		return eachValue(c, v.Value, dim)
	case *Vector:
		if dim != 1 {
			panic("impossible eachValue")
		}
		return eachVector(v)
	case *Matrix:
		if dim > len(v.shape) {
			panic("impossible eachValue")
		}
		return eachMatrix(c, v, dim)
	}
}

// BinaryEach computes the result of applying op to lv and rv,
// applying the "each" expansion depending on how many times
// @ appears on the left and right ends of op.
func BinaryEach(c Context, lv Value, op string, rv Value) Value {
	// Count as many @s as possible for the left side.
	// Must strip at least one if present, but don't have to strip all,
	// in case we are doing @ of vector of vectors.
	l := lv.toType(op, c, matrixType).(*Matrix)
	lmax := len(l.shape)
	ld := 0
	for ld < len(op) && ld < lmax && op[ld] == '@' {
		ld++
	}
	if ld == 0 && op[0] == '@' {
		c.Errorf("%s: left side is scalar", op)
	}
	lhs := eachValue(c, lv, ld)

	// Count as many @s as possible for the right side.
	// Must strip at least one if present, but don't have to strip all,
	// in case we are doing @ of vector of vectors.
	r := rv.toType(op, c, matrixType).(*Matrix)
	rmax := len(r.shape)
	rd := 0
	for rd < len(op) && rd < rmax && op[len(op)-1-rd] == '@' {
		rd++
	}
	if rd == 0 && op[len(op)-1] == '@' {
		c.Errorf("%s: right side is scalar", op)
	}
	rhs := eachValue(c, rv, rd)

	innerOp := op[ld : len(op)-rd]
	data := newVectorEditor(0, nil)

	for x := range lhs {
		for y := range rhs {
			data.Append(c.EvalBinary(x, innerOp, y))
		}
	}

	if ld+rd == 1 {
		return data.Publish()
	}
	shape := append(append([]int{}, l.shape[:ld]...), r.shape[:rd]...)
	return NewMatrix(c, shape, data.Publish())
}

// Each computes the result of running op on each element of v.
// The trailing @ has been removed.
func Each(c Context, op string, v Value) Value {
	m := v.toType(op, c, matrixType).(*Matrix)
	max := len(m.shape)
	d := 0
	for d < len(op) && d < max && op[len(op)-1-d] == '@' {
		d++
	}
	if d == 0 {
		c.Errorf("%s: arg is scalar", op)
	}

	data := newVectorEditor(0, nil)
	for x := range eachValue(c, v, d) {
		data.Append(c.EvalUnary(op[:len(op)-d], x))
	}

	if d == 1 {
		return data.Publish()
	}
	return NewMatrix(c, m.shape[:d], data.Publish())
}

// unaryVectorOp applies op elementwise to i.
func unaryVectorOp(c Context, op string, i Value) Value {
	u := i.(*Vector)
	n := newVectorEditor(u.Len(), nil)
	pfor(safeUnary(op), 1, n.Len(), func(lo, hi int) {
		for k := lo; k < hi; k++ {
			n.Set(k, c.EvalUnary(op, u.At(k)))
		}
	})
	return n.Publish()
}

// unaryMatrixOp applies op elementwise to i.
func unaryMatrixOp(c Context, op string, i Value) Value {
	u := i.(*Matrix)
	n := newVectorEditor(u.data.Len(), nil)
	pfor(safeUnary(op), 1, n.Len(), func(lo, hi int) {
		for k := lo; k < hi; k++ {
			n.Set(k, c.EvalUnary(op, u.data.At(k)))
		}
	})
	return NewMatrix(c, u.shape, n.Publish())
}

// binaryVectorOp applies op elementwise to i and j.
func binaryVectorOp(c Context, i Value, op string, j Value) Value {
	u, v := i.(*Vector), j.(*Vector)
	if u.Len() == 1 {
		n := newVectorEditor(v.Len(), nil)
		pfor(safeBinary(op), 1, n.Len(), func(lo, hi int) {
			for k := lo; k < hi; k++ {
				n.Set(k, c.EvalBinary(u.At(0), op, v.At(k)))
			}
		})
		return n.Publish()
	}
	if v.Len() == 1 {
		n := newVectorEditor(u.Len(), nil)
		pfor(safeBinary(op), 1, n.Len(), func(lo, hi int) {
			for k := lo; k < hi; k++ {
				n.Set(k, c.EvalBinary(u.At(k), op, v.At(0)))
			}
		})
		return n.Publish()
	}
	u.sameLength(c, v)
	n := newVectorEditor(u.Len(), nil)
	pfor(safeBinary(op), 1, n.Len(), func(lo, hi int) {
		for k := lo; k < hi; k++ {
			n.Set(k, c.EvalBinary(u.At(k), op, v.At(k)))
		}
	})
	return n.Publish()
}

// binaryMatrixOp applies op elementwise to i and j.
func binaryMatrixOp(c Context, i Value, op string, j Value) Value {
	u, v := i.(*Matrix), j.(*Matrix)
	shape := u.shape
	var n *vectorEditor

	// One or the other may be a scalar in disguise.
	switch {
	case isScalar(u):
		// Scalar op Matrix.
		shape = v.shape
		n = newVectorEditor(v.data.Len(), nil)
		pfor(safeBinary(op), 1, n.Len(), func(lo, hi int) {
			for k := lo; k < hi; k++ {
				n.Set(k, c.EvalBinary(u.data.At(0), op, v.data.At(k)))
			}
		})
	case isScalar(v):
		// Matrix op Scalar.
		n = newVectorEditor(u.data.Len(), nil)
		pfor(safeBinary(op), 1, n.Len(), func(lo, hi int) {
			for k := lo; k < hi; k++ {
				n.Set(k, c.EvalBinary(u.data.At(k), op, v.data.At(0)))
			}
		})
	case isVector(u, v.shape):
		// Vector op Matrix.
		shape = v.shape
		n = newVectorEditor(v.data.Len(), nil)
		dim := u.shape[0]
		pfor(safeBinary(op), 1, n.Len(), func(lo, hi int) {
			for k := lo; k < hi; k++ {
				n.Set(k, c.EvalBinary(u.data.At(k%dim), op, v.data.At(k)))
			}
		})
	case isVector(v, u.shape):
		// Matrix op Vector.
		n = newVectorEditor(u.data.Len(), nil)
		dim := v.shape[0]
		pfor(safeBinary(op), 1, n.Len(), func(lo, hi int) {
			for k := lo; k < hi; k++ {
				n.Set(k, c.EvalBinary(u.data.At(k), op, v.data.At(k%dim)))
			}
		})
	default:
		// Matrix op Matrix.
		u.sameShape(c, v)
		n = newVectorEditor(u.data.Len(), nil)
		pfor(safeBinary(op), 1, n.Len(), func(lo, hi int) {
			for k := lo; k < hi; k++ {
				n.Set(k, c.EvalBinary(u.data.At(k), op, v.data.At(k)))
			}
		})
	}
	return NewMatrix(c, shape, n.Publish())
}

// IsScalarType reports whether u is an actual scalar, an int or float etc.
func IsScalarType(c Context, v Value) bool {
	return whichType(c, v) < vectorType
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
	case Char:
		return v == '\x00'
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
		return v.Int.Cmp(big.NewInt(int64(i)))
	case BigRat:
		return v.Rat.Cmp(big.NewRat(int64(i), 1))
	case BigFloat:
		return v.Float.Cmp(big.NewFloat(float64(i)))
	case Complex:
		return -1
	}
	return -1
}

// isTrue reports whether v represents boolean truth. If v is not
// ultimately a scalar or empty, an error results.
func isTrue(c Context, fnName string, v Value) bool {
	switch i := v.(type) {
	case Char:
		return i != '\x00'
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
	case QuietValue:
		return isTrue(c, fnName, i.Value)
	case *Vector:
		switch i.Len() {
		case 0:
			return false
		case 1:
			return isTrue(c, fnName, i.At(0))
		}
	case *Matrix:
		switch i.data.Len() {
		case 0:
			return false
		case 1:
			return isTrue(c, fnName, i.data.At(0))
		}
	}
	c.Errorf("invalid expression %s for conditional inside %q", v, fnName)
	return false
}

// sgn is a wrapper for calling "sgn v".
func sgn(c Context, v Value) int {
	return int(c.EvalUnary("sgn", v).(Int))
}

// inverse returns 1/v for any scalar value, errors otherwise.
func inverse(c Context, v Value) Value {
	switch v := v.(type) {
	case Int:
		return v.inverse(c)
	case BigInt:
		return v.inverse(c)
	case BigRat:
		return v.inverse(c)
	case BigFloat:
		return v.inverse(c)
	case Complex:
		return v.inverse(c)
	}
	c.Errorf("inverse of non-scalar %s", v)
	return zero
}

func mod(c Context, a, b Value) Value {
	_, rem := QuoRem("mod", c, a, b)
	return rem
}

// QuoRem uses Euclidean division to return the quotient and remainder for a/b.
// The quotient will be an integer, possibly negative; the remainder is always positive
// and may be fractional. Returned values satisfy the identity that
//
//	quo = a div b  such that
//	rem = a - b*quo  with 0 <= rem < |y|
//
// See comment for math/big.Int.DivMod for details.
// Exported for testing.
func QuoRem(op string, c Context, a, b Value) (div, rem Value) {
	if z, ok := b.shrink().(Int); ok && z == 0 { // If zero, it must be shrinkable to Int.
		return zero, a
	}
	aT := whichType(c, a)
	bT := whichType(c, b)
	negX, negY := false, false
	// The calculations all do the division on the absolute values,
	// then restore sign and adjust if necessary afterwards.
	switch typ, _ := binaryArithType(c, aT, bT); typ {
	case intType:
		x := int(a.(Int))
		y := int(b.(Int))
		if x < 0 {
			x = -x
			negX = true
		}
		if y < 0 {
			y = -y
			negY = true
		}
		quo := x / y
		rem := x % y
		if negX && rem != 0 {
			rem = y - rem
			quo++
		}
		if negX != negY {
			quo = -quo
		}
		return Int(quo), Int(rem)
	case bigIntType:
		x := a.toType(op, c, bigIntType).(BigInt)
		y := b.toType(op, c, bigIntType).(BigInt)
		rem := big.NewInt(0)
		quo := big.NewInt(0)
		// This is the one case we don't need to work hard.
		quo.DivMod(x.Int, y.Int, rem)
		return BigInt{quo}.shrink(), BigInt{rem}.shrink()
	case bigRatType:
		x := a.toType(op, c, bigRatType).(BigRat).Rat
		y := b.toType(op, c, bigRatType).(BigRat).Rat
		if x.Sign() < 0 {
			x = x.Set(x) // Copy x.
			x.Neg(x)
			negX = true
		}
		if y.Sign() < 0 {
			y = y.Set(y) // Copy y.
			y.Neg(y)
			negY = true
		}
		quo := big.NewRat(1, 1).Quo(x, y)
		num := big.NewInt(0).Set(quo.Num())
		iquo := num.Quo(num, quo.Denom()) // Truncation of quotient to an integer.
		// quo is the division, iquo is its integer truncation. Remainder is (quo-iquo)*y.
		rem := quo.Sub(quo, big.NewRat(1, 1).SetInt(iquo))
		rem.Mul(rem, y)
		if negX && rem.Sign() != 0 {
			rem.Sub(y, rem)
			iquo.Add(iquo, bigIntOne.Int)
		}
		if negX != negY {
			iquo.Neg(iquo)
		}
		return BigInt{iquo}.shrink(), BigRat{rem}.shrink()
	case bigFloatType:
		x := a.toType(op, c, bigFloatType).(BigFloat).Float
		y := b.toType(op, c, bigFloatType).(BigFloat).Float
		if x.Sign() < 0 {
			x = x.Copy(x)
			x.Neg(x)
			negX = true
		}
		if y.Sign() < 0 {
			y = y.Copy(y)
			y.Neg(y)
			negY = true
		}
		quo := big.NewFloat(0).Quo(x, y)
		iquo, _ := quo.Int(nil)
		// quo is the division, iquo is its integer truncation. Remainder is (quo-iquo)*y.
		rem := quo.Sub(quo, big.NewFloat(0).SetInt(iquo))
		rem.Mul(rem, y)
		if negX && rem.Sign() != 0 {
			rem.Sub(y, rem)
			iquo.Add(iquo, bigIntOne.Int)
		}
		if negX != negY {
			iquo.Neg(iquo)
		}
		return BigInt{iquo}.shrink(), BigFloat{rem}.shrink()
	default:
		c.Errorf("%s undefined for type %s", op, typ)
	}
	return zero, a
}

// EvalFunctionBody evaluates the list of expressions inside a function,
// with no default value.
func EvalFunctionBody(context Context, fnName string, body StatementList, hasRet bool) (v Value) {
	if hasRet {
		// The runtime has n^2 behavior handling repanics. Avoid that if possible.
		defer func() {
			// Catch any early returns.
			err := recover()
			if err == nil {
				return
			}
			r, ok := err.(*RetExpr)
			if !ok {
				panic(err)
			}
			v = r.Value
		}()
	}
	v, _ = evalStatementList(context, fnName, nil, body)
	return v
}

// EvalBlock evaluates the list of expressions inside a block, with
// the empty expression as the default value.
func EvalBlock(context Context, fnName string, body StatementList) Value {
	v, _ := evalStatementList(context, fnName, empty, body)
	return v
}

// evalStatementList evaluates an expression list with a specified
// default value. A colon expression will give an early return value,
// signaled by earlyExit. RetExpr is handled in EvalFunctionBody
// as it must step to the top level of the function.
func evalStatementList(context Context, fnName string, v Value, body StatementList) (val Value, earlyExit bool) {
	for _, e := range body {
		if stmt, ok := e.(*Statement); ok {
			e = stmt.Parse(context)
		}
		switch expr := e.(type) {
		case *ColonExpr:
			if isTrue(context, fnName, expr.Cond.Eval(context)) {
				return expr.Value.Eval(context), true // Early exit value for block.
			}
			continue
		}
		v = e.Eval(context)
	}
	return v, false
}

// flatten returns a simple vector containing the scalar elements of v.
func flatten(v Value) iter.Seq2[int, Value] {
	return func(yield func(int, Value) bool) {
		flattenTo(0, v, yield)
	}
}

// flattenTo flattens the values contained in v,
// calling yield(off, x0), yield(off+1, x1), ... for successive values.
// It returns the new next offset to use
// and whether the iteration should continue at all.
func flattenTo(off int, v Value, yield func(int, Value) bool) (newOff int, cont bool) {
	switch v := v.(type) {
	case *Matrix:
		return flattenTo(off, v.data, yield)
	case *Vector:
		for _, elem := range v.All() {
			newOff, cont := flattenTo(off, elem, yield)
			if !cont {
				return 0, false
			}
			off = newOff
		}
		return off, true
	default:
		if !yield(off, v) {
			return 0, false
		}
		return off + 1, true
	}
}

// box returns its argument wrapped into a one-element vector.
func box(c Context, v Value) Value {
	return NewVector(v)
}
