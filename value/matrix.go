// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"bytes"
	"fmt"
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
	shape Vector // Will always be Ints inside.
	data  Vector
}

// Shape returns the shape of the matrix.
func (m *Matrix) Shape() Vector {
	return m.shape
}

func (m *Matrix) Rank() int {
	return len(m.shape)
}

// Data returns the data of the matrix as a vector.
func (m *Matrix) Data() Vector {
	return m.data
}

func (m *Matrix) Copy() *Matrix {
	shape := make([]Value, len(m.shape))
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
	nrows := int(m.shape[0].(Int))
	ncols := int(m.shape[1].(Int))
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
		nrows := int(m.shape[0].(Int))
		ncols := int(m.shape[1].(Int))
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
			nelems := int(m.shape[0].(Int))
			ElemSize := m.ElemSize()
			index := 0
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
		n2d := int(m.shape[0].(Int)) // number of 2d submatrices.
		size := m.ElemSize()         // number of elems in each submatrix.
		start := 0
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
	dim := int(m.shape[0].(Int))
	rest := strings.Repeat(" *", m.Rank()-1)[1:]
	var b bytes.Buffer
	for i := 0; i < dim; i++ {
		inner := Matrix{
			shape: m.shape[1:],
			data:  m.data[i*m.ElemSize():],
		}
		if i > 0 {
			b.WriteString("\n")
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
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for _, line := range lines {
		if len(line) > 0 {
			b.WriteString(spaces(indentation))
		}
		b.WriteString(line)
		b.WriteByte('\n')
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
func (m *Matrix) Size() int {
	return size(m.shape)
}

// ElemSize returns the size of each top-level element of the matrix.
// Given shape [a, b, c, ...] it is b*c*....
func (m *Matrix) ElemSize() int {
	return size(m.shape[1:])
}

func size(shape []Value) int {
	size := 1
	for _, i := range shape {
		size *= int(i.(Int))
	}
	return size
}

// NewMatrix makes a new matrix. The number of elements must fit in an Int.
func NewMatrix(shape, data []Value) *Matrix {
	// Check consistency and sanity.
	nelems := 0
	if len(shape) > 0 {
		// While we're here, demote the shape vector to its Inner values.
		// Thus after NewMatrix shape is always guaranteed to be only Ints.
		nshape := make([]Value, len(shape))
		for i := 0; i < len(shape); i++ {
			nshape[i] = shape[i].Inner()
			_, ok := nshape[i].(Int)
			if !ok {
				Errorf("non-integral shape for new matrix")
			}
		}
		// Can't use size function here: must avoid overflow.
		n := nshape[0].(Int)
		for i := 1; i < len(shape); i++ {
			n *= nshape[i].(Int)
			if n > maxInt {
				Errorf("matrix too large")
			}
		}
		nelems = int(n)
		shape = nshape
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

func (m *Matrix) toType(conf *config.Config, which valueType) Value {
	switch which {
	case matrixType:
		return m
	}
	Errorf("cannot convert matrix to %s", which)
	return nil
}

func (x *Matrix) sameShape(y *Matrix) {
	if x.Rank() != y.Rank() {
		Errorf("rank mismatch: %s != %s", x.shape, y.shape)
	}
	for i, d := range x.shape {
		if d != y.shape[i] {
			Errorf("rank mismatch: %s != %s", x.shape, y.shape)
		}
	}
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
	for i := range A {
		n, ok := A[i].Inner().(Int)
		if !ok || n < 0 || maxInt < n {
			Errorf("bad shape for rho: %s is not a small integer", A[i])
		}
		nelems *= n
		if nelems > maxInt {
			Errorf("rho has too many elements")
		}
	}
	values := make([]Value, nelems)
	j := 0
	for i := range values {
		if j >= len(B) {
			j = 0
		}
		values[i] = B[j]
		j++
	}
	if len(A) == 1 {
		return NewVector(values)
	}
	return NewMatrix(A, NewVector(values))
}

// rotate returns a copy of v with elements rotated left by n.
// Rotation occurs on the rightmost axis.
func (m *Matrix) rotate(n int) Value {
	if m.Rank() == 0 {
		return &Matrix{}
	}
	elems := make([]Value, len(m.data))
	dim := int(m.shape[m.Rank()-1].(Int))
	n %= dim
	if n < 0 {
		n += dim
	}
	for i := 0; i < len(m.data); i += dim {
		doRotate(elems[i:i+dim], m.data[i:i+dim], n)
	}
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
	dim := len(m.data) / int(m.shape[0].(Int))

	n *= dim
	n %= len(m.data)
	if n < 0 {
		n += len(m.data)
	}

	for i := 0; i < len(m.data); i += dim {
		copy(elems[i:i+dim], m.data[n:n+dim])
		n += dim
		if n >= len(m.data) {
			n = 0
		}
	}

	return NewMatrix(m.shape, elems)
}

// transpose returns (as a new matrix) the transposition of the argument.
func (m *Matrix) transpose() *Matrix {
	rank := m.Rank()
	if rank < 2 {
		// Shouldn't happen but easy.
		return m.Copy()
	}
	// Shapes as integers not values are easier here.
	// The reversed shape of m, that is, shape of m transposed.
	shape := make([]int64, rank)
	shapeVector := make([]Value, rank) // For NewMatrix.
	for i := range shape {
		v := m.Shape()[i]
		shape[rank-1-i] = int64(v.(Int))
		shapeVector[rank-1-i] = v
	}
	data := m.Data()
	result := make([]Value, len(data))
	sz := len(data) * rank
	counters := make([]int64, rank)
	for i, elem := 0, 0; i < sz; i += rank {
		j := offset(shape, counters)
		result[j] = data[elem]
		elem++
		for k := 0; k < rank; k++ {
			// Big-endian counter transposes the indexes.
			counters[k]++
			if counters[k] < shape[k] {
				break
			}
			counters[k] = 0
		}
	}
	return NewMatrix(shapeVector, result)
}

// offset returns, given a matrix's shape, the index within the slice holding the
// data of the element indexed in the full matrix by the successive indexes.
func offset(shape, indexes []int64) int64 {
	j := int64(0)
	sz := int64(1)
	for i := int64(len(indexes)) - 1; i >= 0; i-- {
		j += indexes[i] * sz
		sz *= shape[i]
	}
	return j
}
