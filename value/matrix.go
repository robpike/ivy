// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value // import "robpike.io/ivy/value"

import (
	"bytes"
	"fmt"
	"strings"
)

/*
    3 4 ⍴ 1 2 3 4

1 2 3 4
1 2 3 4
1 2 3 4
*/

type Matrix struct {
	shape Vector // Will always be Ints inside.
	data  Vector
}

// write2d prints the 2d matrix m into the buffer.
// value is a slice of already-printed values.
// The receiver provides only the shape of the matrix.
func (m Matrix) write2d(b *bytes.Buffer, value []string, width int) {
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

func (m Matrix) String() string {
	var b bytes.Buffer
	switch len(m.shape) {
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
		// We print the elements into one big string,
		// slice that, and then format so they line up.
		// Will need some rethinking when decimal points
		// can appear.
		// Vector.String does what we want for the first part.
		strs := strings.Split(m.data.String(), " ")
		wid := 1
		for _, s := range strs {
			if wid < len(s) {
				wid = len(s)
			}
		}
		m.write2d(&b, strs, wid)
	case 3:
		// As for 2d: print the vector elements, compute the
		// global width, and use that to print each 2d submatrix.
		strs := strings.Split(m.data.String(), " ")
		wid := 1
		for _, s := range strs {
			if wid < len(s) {
				wid = len(s)
			}
		}
		n2d := int(m.shape[0].(Int)) // number of 2d submatrices.
		size := m.elemSize()         // number of elems in each submatrix.
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
		return m.higherDim("[", 0)
	}
	return b.String()
}

func (m Matrix) higherDim(prefix string, indentation int) string {
	if len(m.shape) <= 3 {
		return indent(indentation, m.String())
	}
	dim := int(m.shape[0].(Int))
	rest := strings.Repeat(" *", len(m.shape)-1)[1:]
	var b bytes.Buffer
	for i := 0; i < dim; i++ {
		inner := Matrix{
			shape: m.shape[1:],
			data:  m.data[i*m.elemSize():],
		}
		if i > 0 {
			b.WriteString("\n")
		}
		innerPrefix := fmt.Sprintf("%s%d ", prefix, i+conf.Origin())
		b.WriteString(indent(indentation, "%s%s]:\n", innerPrefix, rest))
		b.WriteString(inner.higherDim(innerPrefix, indentation+1))
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

// elemSize returns number of elements of the submatrix forming the elements of the matrix.
// Given shape [a, b, c, ...] it is b*c*....
func (m Matrix) elemSize() int {
	return size(m.shape[1:])
}

// size returns number of elements of the matrix.
// Given shape [a, b, c, ...] it is b*c*....
func (m Matrix) size() int {
	return size(m.shape)
}

func size(shape []Value) int {
	size := 1
	for _, i := range shape {
		size *= int(i.(Int))
	}
	return size
}

// newMatrix makes a new matrix. The number of elems
// must fit in an Int.
func newMatrix(shape, data []Value) Matrix {
	// Check consistency and sanity.
	nelems := 0
	if len(shape) > 0 {
		for i := 0; i < len(shape); i++ {
			_, ok := shape[0].(Int)
			if !ok {
				Errorf("non-integral shape for new matrix")
			}
		}
		n := shape[0].(Int)
		for i := 1; i < len(shape); i++ {
			n *= shape[i].(Int)
			if n > maxInt {
				Errorf("matrix too large")
			}
		}
		nelems = int(n)
	}
	if nelems != len(data) {
		Errorf("inconsistent shape and data size for new matrix")
	}
	return Matrix{
		shape: shape,
		data:  data,
	}
}

func (m Matrix) Eval(Context) Value {
	return m
}

func (m Matrix) toType(which valueType) Value {
	switch which {
	case intType:
		panic("matrix to int")
	case bigIntType:
		panic("matrix to big int")
	case bigRatType:
		panic("matrix to big rat")
	case vectorType:
		panic("matrix to vector")
	case matrixType:
		return m
	}
	panic("BigInt.toType")
}

func (m Matrix) Shape() Vector {
	return m.shape
}

func (x Matrix) sameShape(y Matrix) {
	if len(x.shape) != len(y.shape) {
		Errorf("rank mismatch: %s != %s", x.shape, y.shape)
	}
	for i, d := range x.shape {
		if d != y.shape[i] {
			Errorf("rank mismatch: %s != %s", x.shape, y.shape)
		}
	}
}

// reshape implements unary rho
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
		n, ok := A[i].(Int)
		if !ok || n < 0 || maxInt < n {
			Errorf("bad shape")
		}
		nelems *= n
		if nelems > maxInt {
			Errorf("too many elements")
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
	return newMatrix(A, NewVector(values))
}
