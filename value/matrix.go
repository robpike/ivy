// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"bytes"
	"fmt"
	"io"
	"math/bits"
	"sort"
	"strings"

	"robpike.io/ivy/config"
)

/*
    3 4 ⍴ ⍳12

 1  2  3  4
 5  6  7  8
 9 10 11 12
*/

type Matrix struct {
	shape []int
	data  Vector
}

// Shape returns the shape of the matrix.
func (m *Matrix) Shape() []int {
	return m.shape
}

func (m *Matrix) Rank() int {
	return len(m.shape)
}

func (m *Matrix) shrink() Value {
	return m
}

// Data returns the data of the matrix as a vector.
func (m *Matrix) Data() Vector {
	return m.data
}

func (m *Matrix) Copy() *Matrix {
	shape := make([]int, len(m.shape))
	data := make([]Value, len(m.data))
	copy(shape, m.shape)
	copy(data, m.data)
	return &Matrix{
		shape: shape,
		data:  data,
	}
}

// write2d prints the 2d matrix m into the buffer.
// value is a slice of already-printed values.
// The receiver provides only the shape of the matrix.
func (m *Matrix) write2d(b *bytes.Buffer, value []string, width int) {
	nrows := m.shape[0]
	ncols := m.shape[1]
	for row := 0; row < nrows; row++ {
		if row > 0 {
			b.WriteByte('\n')
		}
		index := row * ncols
		for col := 0; col < ncols; col++ {
			if col > 0 {
				b.WriteByte(' ')
			}
			s := value[index]
			pad := width - len(s)
			for ; pad >= 10; pad -= 10 {
				b.WriteString("          ")
			}
			for ; pad > 0; pad-- {
				b.WriteString(" ")
			}
			b.WriteString(s)
			index++
		}
	}
}

func (m *Matrix) fprintf(c Context, w io.Writer, format string) {
	rank := len(m.shape)
	if rank == 0 || len(m.data) == 0 {
		return
	}
	counters := make([]int, len(m.shape))
	verb := verbOf(format)
	printSpace := false
	for i, v := range m.data {
		if printSpace {
			fmt.Fprint(w, " ")
		}
		formatOne(c, w, format, verb, v)
		printSpace = true
		for k := rank - 1; k >= 0; k-- {
			// Litte-endian counter iterates the indexes.
			counters[k]++
			if counters[k] < m.shape[k] {
				break
			}
			// Each time a counter overflows, add a newline.
			// This puts 0 lines between rows, 1 between
			// each 2-d block, 2 between each 3-d block, etc.
			if i < len(m.data)-1 {
				w.Write([]byte{'\n'})
				printSpace = false
			}
			counters[k] = 0
		}
	}
}

func (m *Matrix) String() string {
	return "(" + m.Sprint(debugConf) + ")"
}

func (m *Matrix) Sprint(conf *config.Config) string {
	var b bytes.Buffer
	switch m.Rank() {
	case 0:
		Errorf("matrix is scalar")
	case 1:
		Errorf("matrix is vector")
	case 2:
		nrows := m.shape[0]
		ncols := m.shape[1]
		if nrows == 0 || ncols == 0 {
			return ""
		}
		// If it's all chars, print it without padding or quotes.
		if m.data.AllChars() {
			for i := 0; i < nrows; i++ {
				if i > 0 {
					b.WriteByte('\n')
				}
				fmt.Fprintf(&b, "%s", m.data[i*ncols:(i+1)*ncols].Sprint(conf))
			}
			break
		}
		// We print the elements into one big string,
		// slice that, and then format so they line up.
		// Will need some rethinking when decimal points
		// can appear.
		// Vector.String does what we want for the first part.
		strs := strings.Split(m.data.makeString(conf, true), " ")
		wid := 1
		for _, s := range strs {
			if wid < len(s) {
				wid = len(s)
			}
		}
		m.write2d(&b, strs, wid)
	case 3:
		// If it's all chars, print it without padding or quotes.
		if m.data.AllChars() {
			nelems := m.shape[0]
			ElemSize := m.ElemSize()
			index := int64(0)
			for i := 0; i < nelems; i++ {
				if i > 0 {
					b.WriteString("\n\n")
				}
				fmt.Fprintf(&b, "%s", NewMatrix(m.shape[1:], m.data[index:index+ElemSize]).Sprint(conf))
				index += ElemSize
			}
			break
		}
		// As for 2d: print the vector elements, compute the
		// global width, and use that to print each 2d submatrix.
		strs := strings.Split(m.data.Sprint(conf), " ")
		wid := 1
		for _, s := range strs {
			if wid < len(s) {
				wid = len(s)
			}
		}
		n2d := m.shape[0]    // number of 2d submatrices.
		size := m.ElemSize() // number of elems in each submatrix.
		start := int64(0)
		for i := 0; i < n2d; i++ {
			if i > 0 {
				b.WriteString("\n\n")
			}
			m := Matrix{
				shape: m.shape[1:],
				data:  m.data[start : start+size],
			}
			m.write2d(&b, strs[start:start+size], wid)
			start += size
		}
	default:
		return m.higherDim(conf, "[", 0)
	}
	return b.String()
}

func (m *Matrix) ProgString() string {
	// There is no such thing as a matrix in program listings.
	panic("matrix.ProgString - cannot happen")
}

func (m *Matrix) higherDim(conf *config.Config, prefix string, indentation int) string {
	if m.Rank() <= 3 {
		return indent(indentation, m.Sprint(conf))
	}
	dim := m.shape[0]
	rest := strings.Repeat(" *", m.Rank()-1)[1:]
	var b bytes.Buffer
	for i := 0; i < dim; i++ {
		inner := Matrix{
			shape: m.shape[1:],
			data:  m.data[int64(i)*m.ElemSize():],
		}
		if i > 0 {
			b.WriteString("\n\n")
		}
		innerPrefix := fmt.Sprintf("%s%d ", prefix, i+conf.Origin())
		b.WriteString(indent(indentation, "%s%s]:\n", innerPrefix, rest))
		b.WriteString(inner.higherDim(conf, innerPrefix, indentation+1))
	}
	return b.String()
}

// indent prints the args, indenting each line by the specified amount.
func indent(indentation int, format string, args ...interface{}) string {
	s := fmt.Sprintf(format, args...)
	if indentation == 0 {
		return s
	}
	var b bytes.Buffer
	lines := strings.SplitAfter(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for _, line := range lines {
		if len(line) > 0 {
			b.WriteString(spaces(indentation))
		}
		b.WriteString(line)
	}
	return b.String()
}

// spaces returns 2*n space characters.
func spaces(n int) string {
	if n > 10 {
		n = 10
	}
	return "                    "[:2*n]
}

// Size returns number of elements of the matrix.
// Given shape [a, b, c, ...] it is a*b*c*....
func (m *Matrix) Size() int64 {
	return int64(size(m.shape))
}

// ElemSize returns the size of each top-level element of the matrix.
// Given shape [a, b, c, ...] it is b*c*....
func (m *Matrix) ElemSize() int64 {
	return int64(size(m.shape[1:]))
}

func size(shape []int) int {
	size := 1
	for _, i := range shape {
		hi, lo := bits.Mul(uint(size), uint(i))
		if int(lo) < 0 || hi != 0 {
			Errorf("matrix too large")
		}
		size = int(lo)
	}
	return size
}

// NewMatrix makes a new matrix. The number of elements must fit in an Int.
func NewMatrix(shape []int, data []Value) *Matrix {
	// Check consistency and sanity.
	nelems := 0
	if len(shape) > 0 {
		// Can't use size function here: must avoid overflow.
		n := int64(shape[0])
		for i := 1; i < len(shape); i++ {
			n *= int64(shape[i])
			if n > maxInt {
				Errorf("matrix too large")
			}
		}
		nelems = int(n)
	}
	if nelems != len(data) {
		Errorf("inconsistent shape and data size for new matrix")
	}
	return &Matrix{
		shape: shape,
		data:  data,
	}
}

func (m *Matrix) Eval(Context) Value {
	return m
}

func (m *Matrix) Inner() Value {
	return m
}

func (m *Matrix) toType(op string, conf *config.Config, which valueType) Value {
	switch which {
	case matrixType:
		return m
	}
	Errorf("%s: cannot convert matrix to %s", op, which)
	return nil
}

func (x *Matrix) sameShape(y *Matrix) {
	if !sameShape(x.Shape(), y.Shape()) {
		Errorf("shape mismatch: %s != %s", NewIntVector(x.shape), NewIntVector(y.shape))
	}
}

func sameShape(x, y []int) bool {
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

// reshape implements binary rho
// A⍴B: Array of shape A with data B
func reshape(A, B Vector) Value {
	if len(B) == 0 {
		Errorf("reshape of empty vector")
	}
	if len(A) == 0 {
		return Vector{}
	}
	nelems := Int(1)
	shape := make([]int, len(A))
	for i := range A {
		n, ok := A[i].Inner().(Int)
		if !ok || n < 0 || maxInt < n {
			Errorf("bad shape for rho: %s is not a small integer", A[i])
		}
		nelems *= n
		if nelems > maxInt {
			Errorf("rho has too many elements")
		}
		shape[i] = int(n)
	}
	values := make([]Value, nelems)
	n := copy(values, B)
	// replicate as needed by doubling in values.
	for n < len(values) {
		n += copy(values[n:], values[:n])
	}
	if len(A) == 1 {
		return NewVector(values)
	}
	return NewMatrix(shape, NewVector(values))
}

// rotate returns a copy of v with elements rotated left by n.
// Rotation occurs on the rightmost axis.
func (m *Matrix) rotate(n int) Value {
	if m.Rank() == 0 {
		return &Matrix{}
	}
	elems := make([]Value, len(m.data))
	dim := m.shape[m.Rank()-1]
	n %= dim
	if n < 0 {
		n += dim
	}
	pfor(true, dim, len(m.data)/dim, func(lo, hi int) {
		for i := lo; i < hi; i++ {
			j := i * dim
			doRotate(elems[j:j+dim], m.data[j:j+dim], n)
		}
	})
	return NewMatrix(m.shape, elems)
}

// vrotate returns a copy of v with elements rotated down by n.
// Rotation occurs on the leftmost axis.
func (m *Matrix) vrotate(n int) Value {
	if m.Rank() == 0 {
		return &Matrix{}
	}
	if m.Rank() == 1 {
		return m
	}

	elems := make([]Value, len(m.data))
	dim := len(m.data) / m.shape[0]

	n *= dim
	n %= len(m.data)
	if n < 0 {
		n += len(m.data)
	}

	pfor(true, dim, len(m.data)/dim, func(lo, hi int) {
		for i := lo; i < hi; i++ {
			j := i * dim
			n := (n + j) % len(m.data)
			copy(elems[j:j+dim], m.data[n:n+dim])
		}
	})

	return NewMatrix(m.shape, elems)
}

// transpose returns (as a new matrix) the transposition of the argument.
func (m *Matrix) transpose(c Context) *Matrix {
	v := make(Vector, m.Rank())
	origin := c.Config().Origin()
	for i := range v {
		v[len(v)-1-i] = Int(i + origin)
	}
	return m.binaryTranspose(c, v)
}

// binaryTranspose returns the transposition of m specified by v,
// defined by (v transp m)[i] = m[i[v]] (i is in general an index vector).
// APL calls this operator the dyadic transpose.
func (m *Matrix) binaryTranspose(c Context, v Vector) *Matrix {
	origin := c.Config().Origin()
	if len(v) != m.Rank() {
		Errorf("transp: vector length %d != matrix rank %d", len(v), m.Rank())
	}

	// Extract old-to-new index mapping and determine rank.
	oldToNew := make([]int, len(v))
	rank := -1
	for i := range v {
		vi, ok := v[i].(Int)
		if !ok {
			Errorf("transp: non-int index %v", v[i])
		}
		if vi < Int(origin) || vi >= Int(origin+m.Rank()) {
			Errorf("transp: out-of-range index %v", vi)
		}
		vi -= Int(origin)
		oldToNew[i] = int(vi)
		if rank <= int(vi) {
			rank = int(vi) + 1
		}
	}

	// Determine shape of result.
	// Each dimension is the min of the old dimensions mapping to it.
	oldShape := m.Shape()
	shape := make([]int, rank)
	for i := range shape {
		shape[i] = -1
	}
	for oi, dim := range oldShape {
		if i := oldToNew[oi]; shape[i] == -1 || shape[i] > dim {
			shape[i] = dim
		}
	}
	sz := 1
	for i, dim := range shape {
		if dim == -1 {
			Errorf("transp: partial index: missing %v", i+origin)
		}
		sz *= dim
	}

	old := m.Data()
	data := make([]Value, sz)
	pfor(true, 1, len(data), func(lo, hi int) {
		// Compute starting index
		index := make([]int, rank)
		i := lo
		for j := rank - 1; j >= 0; j-- {
			if shape[j] > 0 {
				index[j] = i % shape[j]
				i /= shape[j]
			}
		}
		for i := lo; i < hi; i++ {
			// Compute old index for this new entry.
			oi := 0
			for j := range v {
				oi = oi*m.Shape()[j] + index[oldToNew[j]]
			}

			data[i] = old[oi]

			// Increment index.
			for j := rank - 1; j >= 0; j-- {
				if index[j]++; index[j] < shape[j] {
					break
				}
				index[j] = 0
			}
		}
	})

	return NewMatrix(shape, data)
}

// catenate returns the catenation x, y.
// It handles the following shape combinations:
//
//	(n ...), (...) -> (n+1 ...)  # list, elem
//	(...), (n ...) -> (n+1 ...)  # elem, list
//	(n ...), (m ...) -> (n+m ...)  # list, list
//	(1), (n ...) -> (n+1 ...)  # scalar (extended), list
//	(n ...), (1) -> (n+1 ...)  # list, scalar (extended)
//
func (x *Matrix) catenate(y *Matrix) *Matrix {
	if x.Rank() == 0 || y.Rank() == 0 {
		Errorf("empty matrix for ,")
	}
	var shape []int
	var data Vector
	switch {
	default:
		Errorf("catenate shape mismatch: %s != %s", NewIntVector(x.shape[1:]), NewIntVector(y.shape))

	case x.Rank() == y.Rank() && sameShape(x.shape[1:], y.shape[1:]):
		// list, list
		shape = make([]int, x.Rank())
		copy(shape, x.shape)
		shape[0] = x.shape[0] + y.shape[0]

	case x.Rank() == y.Rank()+1 && sameShape(x.shape[1:], y.shape):
		// list, elem
		shape = make([]int, x.Rank())
		copy(shape, x.shape)
		shape[0]++

	case x.Rank()+1 == y.Rank() && sameShape(x.shape, y.shape[1:]):
		// elem, list
		shape = make([]int, y.Rank())
		copy(shape, y.shape)
		shape[0]++

	case x.Rank() == 1 && x.shape[0] == 1 && y.Rank() > 1:
		// scalar extension, list
		shape = make([]int, y.Rank())
		copy(shape, y.shape)
		shape[0]++
		elem := y.ElemSize()
		a := x.Data()[0]
		data = make(Vector, elem+int64(len(y.Data())))
		for i := int64(0); i < elem; i++ {
			data[i] = a
		}
		copy(data[elem:], y.Data())

	case x.Rank() > 1 && y.Rank() == 1 && y.shape[0] == 1:
		// list, scalar extension
		shape = make([]int, x.Rank())
		copy(shape, x.shape)
		shape[0]++
		elem := x.ElemSize()
		b := y.Data()[0]
		data = make(Vector, elem+int64(len(x.Data())))
		copy(data, x.Data())
		ext := data[len(x.Data()):]
		for i := int64(0); i < elem; i++ {
			ext[i] = b
		}
	}
	if data == nil {
		data = make(Vector, len(x.Data())+len(y.Data()))
		copy(data, x.Data())
		copy(data[len(x.Data()):], y.Data())
	}
	return NewMatrix(shape, data)
}

// sel returns the selection of m according to v.
// The selection applies to the final axis.
func (m *Matrix) sel(c Context, v Vector) *Matrix {
	// All lhs values must be small integers.
	if !v.AllInts() {
		Errorf("sel: left operand must be small integers")
	}

	var count int64
	for _, x := range v {
		x := x.(Int)
		if x < 0 {
			count -= int64(x)
		} else {
			count += int64(x)
		}
	}
	if len(v) != 1 && len(v) != m.Shape()[len(m.Shape())-1] {
		Errorf("sel: bad length %d for shape %s", len(v), NewIntVector(m.Shape()))
	}
	if len(v) == 1 {
		count *= int64(m.Shape()[len(m.Shape())-1])
	}

	shape := make([]int, len(m.Shape()))
	copy(shape, m.Shape())
	shape[len(shape)-1] = int(count)

	for _, dim := range shape[:len(shape)-1] {
		count *= int64(dim)
	}
	if count > 1e8 {
		Errorf("sel: result too large: %d elements", count)
	}

	result := make(Vector, 0, count)
	for i, y := range m.Data() {
		c := v[i%len(v)].(Int)
		if c < 0 {
			c = -c
			y = Int(0)
		}
		for ; c > 0; c-- {
			result = append(result, y)
		}
	}

	return NewMatrix(shape, result)
}

// take returns v take m.
func (m *Matrix) take(c Context, v Vector) *Matrix {
	// Extend short vector to full rank using shape.
	if len(v) > m.Rank() {
		Errorf("take: bad length %d for shape %s", len(v), NewIntVector(m.Shape()))
	}
	if len(v) < m.Rank() {
		ext := make(Vector, m.Rank())
		copy(ext, v)
		for i := len(v); i < m.Rank(); i++ {
			ext[i] = Int(m.Shape()[i])
		}
		v = ext
	}

	// All lhs values must be small integers in range for m's shape.
	// Compute new shape.
	shape := make([]int, m.Rank())
	count := int64(1)
	for i, x := range v {
		y, ok := x.(Int)
		if !ok {
			Errorf("take: left operand must be small integers")
		}
		if y < 0 {
			y = -y
		}
		if y > Int(m.Shape()[i]) {
			Errorf("take: left operand %v out of range for %d in shape %v", x, m.Shape()[i], NewIntVector(m.Shape()))
		}
		shape[i] = int(y)
		count *= int64(y)
	}

	result := make(Vector, 0, count)
	result = appendTake(result, v, m.Data(), m.Shape())
	return NewMatrix(shape, result)
}

// TODO(rsc): Use pfor, but will probably require
// avoiding recursion and definitely avoiding append.
func appendTake(result, take, data Vector, dshape []int) Vector {
	if len(take) == 0 {
		return append(result, data...)
	}
	n := Int(len(data) / dshape[0])
	t := take[0].(Int)
	if t >= 0 {
		data = data[:t*n]
	} else {
		data = data[Int(len(data))-(-t)*n:]
	}
	for ; len(data) > 0; data = data[n:] {
		result = appendTake(result, take[1:], data[:n], dshape[1:])
	}
	return result
}

// drop returns v drop m.
func (m *Matrix) drop(c Context, v Vector) *Matrix {
	// Extend short vector to full rank using zeros.
	if len(v) > m.Rank() {
		Errorf("drop: bad length %d for shape %s", len(v), NewIntVector(m.Shape()))
	}
	if !v.AllInts() {
		Errorf("drop: left operand must be small integers")
	}
	if len(v) < m.Rank() {
		ext := make(Vector, m.Rank())
		copy(ext, v)
		for i := len(v); i < m.Rank(); i++ {
			ext[i] = Int(0)
		}
		v = ext
	}

	// All lhs values must be small integers in range for m's shape.
	// Convert to parameters for take.
	//	1 drop x = (1 - N) take x
	//	-1 drop x = (N - 1) take x
	take := make(Vector, len(v))
	for i, x := range v {
		x := x.(Int)
		if x < -Int(m.Shape()[i]) || x > Int(m.Shape()[i]) {
			Errorf("drop: left operand %v out of range for %d in shape %v", x, m.Shape()[i], NewIntVector(m.Shape()))
		}
		if x >= 0 {
			take[i] = x - Int(m.Shape()[i])
		} else {
			take[i] = Int(m.Shape()[i]) + x
		}
	}

	return m.take(c, take)
}

// grade returns as a Vector the indexes that sort the rows of m
// into increasing order.
func (m *Matrix) grade(c Context) Vector {
	x := make([]int, m.shape[0])
	for i := range x {
		x[i] = i
	}
	v := m.data
	stride := len(v) / m.shape[0]
	sort.Slice(x, func(i, j int) bool {
		i = x[i] * stride
		j = x[j] * stride
		for k := 0; k < stride; k++ {
			if toBool(c.EvalBinary(v[i+k], "==", v[j+k])) {
				continue
			}
			return toBool(c.EvalBinary(v[i+k], "<", v[j+k]))
		}
		return false
	})
	origin := c.Config().Origin()
	for i := range x {
		x[i] += origin
	}
	return NewIntVector(x)
}
