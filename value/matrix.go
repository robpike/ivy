// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"bytes"
	"fmt"
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
		panic(Errorf("TODO: no matrix dimensions"))
	case 1:
		panic(Errorf("matrix is vector"))
	case 2:
		nrows := int(m.shape[0].(Int))
		ncols := int(m.shape[1].(Int))
		for row := 0; row < nrows; row++ {
			index := row * ncols
			for col := 0; col < ncols; col++ {
				if col > 0 {
					fmt.Fprint(&b, " ")
				}
				fmt.Fprint(&b, m.data[index])
				index++
			}
			fmt.Fprint(&b, "\n")
		}
	default:
		// TODO STUPID
		fmt.Fprintln(&b, "shape: ", m.shape)
		fmt.Fprintln(&b, "elems: ", m.data)
	}
	return b.String()
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
		panic("matrix to big vector")
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
		panic(Errorf("rank mismatch: %s %s", x.shape, y.shape))
	}
	for i, d := range x.shape {
		if d != y.shape[i] {
			panic(Errorf("rank mismatch: %s %s", x.shape, y.shape))
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
