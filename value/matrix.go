// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

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

func (m Matrix) String() string {
	var b bytes.Buffer
	switch len(m.shape) {
	case 0:
		panic(Errorf("matrix is scalar"))
	case 1:
		panic(Errorf("matrix is vector"))
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
		for row := 0; row < nrows; row++ {
			if row > 0 {
				b.WriteByte('\n')
			}
			index := row * ncols
			for col := 0; col < ncols; col++ {
				if col > 0 {
					b.WriteByte(' ')
				}
				s := strs[index]
				pad := wid - len(s)
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
	case 3:
		n := int(m.shape[0].(Int))
		size := m.elemSize()
		start := 0
		for i := 0; i < n; i++ {
			if i > 0 {
				b.WriteString("\n\n")
			}
			m := Matrix{
				shape: m.shape[1:],
				data:  m.data[start : start+size],
			}
			b.WriteString(m.String())
			start += size
		}
	default:
		// TODO STUPID
		fmt.Printf("shape: %s; elems: %s\n", m.shape, m.data)
	}
	return b.String()
}

// elemSize returns the length of the submatrix forming the elements of the matrix.
// Given shape [a, b, c, ...] it is b*c*....
func (m Matrix) elemSize() int {
	size := 1
	for _, i := range m.shape[1:] {
		size *= int(i.(Int))
	}
	return size
}

func ValueMatrix(shape, data []Value) Matrix {
	return Matrix{
		shape: shape,
		data:  data,
	}
}

func (m Matrix) Eval() Value {
	return m
}

func (m Matrix) ToType(which valueType) Value {
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
	panic("BigInt.ToType")
}

func (m Matrix) Shape() Vector {
	return m.shape
}

func (x Matrix) sameShape(y Matrix) {
	if len(x.shape) != len(y.shape) {
		panic(Errorf("rank mismatch: %s != %s", x.shape, y.shape))
	}
	for i, d := range x.shape {
		if d != y.shape[i] {
			panic(Errorf("rank mismatch: %s != %s", x.shape, y.shape))
		}
	}
}

// reshape implements unary rho
// A⍴B: Array of shape A with data B
func reshape(A, B Vector) Value {
	if len(A) == 0 {
		panic(Error("bad index"))
	}
	nelems := Int(1)
	for i := range A {
		n, ok := A[i].(Int)
		if !ok || n <= 0 || maxInt < n { // TODO: 0 should be ok.
			panic(Error("bad index"))
		}
		nelems *= n
		if maxInt < nelems {
			panic(Error("too big"))
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
		return ValueSlice(values)
	}
	return ValueMatrix(A, ValueSlice(values))
}
