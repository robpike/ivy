// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"bytes"
	"fmt"
)

type Vector struct {
	unimplemented
	x []Value
}

func SetVector(x []Value) Vector { //TODO NOT CALLED SET if parsers are set
	return Vector{
		x: x,
	}
}

func (v Vector) String() string {
	var b bytes.Buffer
	for i := range v.x {
		if i > 0 {
			fmt.Fprint(&b, " ")
		}
		fmt.Fprint(&b, v.x[i].String())
	}
	return b.String()
}

func (v Vector) Eval() Value {
	return v
}

func (v Vector) Len() int {
	return len(v.x)
}

func (v Vector) Add(x Value) Value {
	switch x := x.(type) {
	case Vector:
		if v.Len() != x.Len() {
			panic(Errorf("length mismatch: %d %d", v.Len(), x.Len()))
		}
		n := make([]Value, v.Len())
		for i := range v.x {
			n[i] = v.x[i].Add(x.x[i])
		}
		return SetVector(n)
	case BigInt, Int:
		xx := x.(Value)
		n := make([]Value, v.Len())
		for i := range v.x {
			n[i] = v.x[i].Add(xx)
		}
		return SetVector(n)
	}
	panic(Errorf("unimplemented Add(Vector, %T)", x))
}

func (v Vector) Append(x Value) Value {
	return SetVector(append(v.x, x))
}

func (v Vector) Sub(x Value) Value {
	switch x := x.(type) {
	case Vector:
		if v.Len() != x.Len() {
			panic(Errorf("length mismatch: %d %d", v.Len(), x.Len()))
		}
		n := make([]Value, v.Len())
		for i := range v.x {
			n[i] = v.x[i].Sub(x.x[i])
		}
		return SetVector(n)
	case BigInt, Int:
		xx := x.(Value)
		n := make([]Value, v.Len())
		for i := range v.x {
			n[i] = v.x[i].Sub(xx)
		}
		return SetVector(n)
	}
	panic(Errorf("unimplemented Sub(Vector, %T)", x))
}

func (v Vector) Mul(x Value) Value {
	switch x := x.(type) {
	case Vector:
		if v.Len() != x.Len() {
			panic(Errorf("length mismatch: %d %d", v.Len(), x.Len()))
		}
		n := make([]Value, v.Len())
		for i := range v.x {
			n[i] = v.x[i].Mul(x.x[i])
		}
		return SetVector(n)
	case BigInt, Int:
		xx := x.(Value)
		n := make([]Value, v.Len())
		for i := range v.x {
			n[i] = v.x[i].Mul(xx)
		}
		return SetVector(n)
	}
	panic(Errorf("unimplemented Mul(Vector, %T)", x))
}

// TODO: here and elsewhere, division needs to become rational.
func (v Vector) Div(x Value) Value {
	switch x := x.(type) {
	case Vector:
		if v.Len() != x.Len() {
			panic(Errorf("length mismatch: %d %d", v.Len(), x.Len()))
		}
		n := make([]Value, v.Len())
		for i := range v.x {
			n[i] = v.x[i].Div(x.x[i])
		}
		return SetVector(n)
	case BigInt, Int:
		xx := x.(Value)
		n := make([]Value, v.Len())
		for i := range v.x {
			n[i] = v.x[i].Div(xx)
		}
		return SetVector(n)
	}
	panic(Errorf("unimplemented Div(Vector, %T)", x))
}

func (v Vector) Neg() Value {
	values := make([]Value, v.Len())
	for i := range values {
		values[i] = v.x[i].Neg()
	}
	return SetVector(values)
}
