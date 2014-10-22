// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"
	"log"
	"strconv"
)

// Int is not only the simplest representation, it provides the operands that mix
// types upward. That is, BigInt.Add(Int) will be done by rewriting as Int.Add(BigInt).

type Int struct {
	unimplemented
	x int64
}

const (
	intBits = 32
	minInt  = -(1 << (intBits - 1))
	maxInt  = 1<<(intBits-1) - 1
)

func SetIntString(s string) (Int, ParseState) {
	i, err := strconv.ParseInt(s, 0, intBits)
	if err == nil {
		return Int{x: i}, Valid
	}
	if err, ok := err.(*strconv.NumError); ok && err.Err == strconv.ErrRange {
		return Int{}, Retry
	}
	log.Print(err)
	return Int{}, Fail
}

func (i Int) String() string {
	return fmt.Sprint(i.x)
}

func (i Int) Eval() Value {
	return i
}

func (i Int) Add(x Value) Value {
	switch x := x.(type) {
	case Int:
		return ValueInt64(i.x + x.x)
	case BigInt:
		return BigInt64(i.x).Add(x)
	case Vector:
		n := make([]Value, x.Len())
		for j := range x.x {
			n[j] = i.Add(x.x[j])
		}
		return ValueSlice(n)
	}
	panic(Errorf("unimplemented Add(Int, %T)", x))
}

func (i Int) Sub(x Value) Value {
	switch x := x.(type) {
	case Int:
		return ValueInt64(i.x - x.x)
	case BigInt:
		return BigInt64(i.x).Sub(x)
	case Vector:
		n := make([]Value, x.Len())
		for j := range x.x {
			n[j] = i.Sub(x.x[j])
		}
		return ValueSlice(n)
	}
	panic(Errorf("unimplemented Sub(Int, %T)", x))
}

func (i Int) Mul(x Value) Value {
	switch x := x.(type) {
	case Int:
		return ValueInt64(i.x * x.x)
	case BigInt:
		return BigInt64(i.x).Mul(x)
	case Vector:
		n := make([]Value, x.Len())
		for j := range x.x {
			n[j] = i.Mul(x.x[j])
		}
		return ValueSlice(n)
	}
	panic(Errorf("unimplemented Mul(Int, %T)", x))
}

func (i Int) Div(x Value) Value {
	switch x := x.(type) {
	case Int:
		if x.x == 0 {
			panic(Error("division by zero"))
		}
		return ValueInt64(i.x / x.x)
	case BigInt:
		return BigInt64(i.x).Div(x)
	case Vector:
		n := make([]Value, x.Len())
		for j := range x.x {
			n[j] = i.Div(x.x[j])
		}
		return ValueSlice(n)
	}
	panic(Errorf("unimplemented Div(Int, %T)", x))
}

func (i Int) Pow(x Value) Value {
	switch x := x.(type) {
	case Int:
		if x.x < 0 {
			panic(Errorf("unimplemented negative exponent %s", x))
		}
		// Let math/big figure out the algorithm.
		return BigInt64(i.x).Pow(x)
	case BigInt:
		if x.x.Sign() < 0 {
			panic(Errorf("unimplemented negative exponent %s", x))
		}
		return BigInt64(i.x).Pow(x)
	case Vector:
		n := make([]Value, x.Len())
		for j := range x.x {
			n[j] = i.Pow(x.x[j])
		}
		return ValueSlice(n)
	}
	panic(Errorf("unimplemented Pow(Int, %T)", x))
}

func (i Int) And(x Value) Value {
	switch x := x.(type) {
	case Int:
		return ValueInt64(i.x & x.x)
	case BigInt:
		return BigInt64(i.x).And(x)
	case Vector:
		n := make([]Value, x.Len())
		for j := range x.x {
			n[j] = i.And(x.x[j])
		}
		return ValueSlice(n)
	}
	panic(Errorf("unimplemented And(Int, %T)", x))
}

func (i Int) Or(x Value) Value {
	switch x := x.(type) {
	case Int:
		return ValueInt64(i.x | x.x)
	case BigInt:
		return BigInt64(i.x).Or(x)
	case Vector:
		n := make([]Value, x.Len())
		for j := range x.x {
			n[j] = i.Or(x.x[j])
		}
		return ValueSlice(n)
	}
	panic(Errorf("unimplemented Or(Int, %T)", x))
}

func (i Int) Xor(x Value) Value {
	switch x := x.(type) {
	case Int:
		return ValueInt64(i.x ^ x.x)
	case BigInt:
		return BigInt64(i.x).Xor(x)
	case Vector:
		n := make([]Value, x.Len())
		for j := range x.x {
			n[j] = i.Xor(x.x[j])
		}
		return ValueSlice(n)
	}
	panic(Errorf("unimplemented Xor(Int, %T)", x))
}

func shiftCount(x Value) uint {
	count, ok := x.(Int)
	if !ok || count.x < 0 || count.x >= intBits {
		panic(Errorf("illegal shift count %d", count.x))
	}
	return uint(count.x)
}

func (i Int) Lsh(x Value) Value {
	return ValueInt64(i.x << shiftCount(x))
}

func (i Int) Rsh(x Value) Value {
	return ValueInt64(i.x >> shiftCount(x))
}

func (i Int) Neg() Value {
	if i.x == minInt {
		var z BigInt
		z.x.SetInt64(-i.x)
		return z
	}
	return Int{x: -i.x}

}

func (i Int) Iota() Value {
	if i.x <= 0 {
		panic(Errorf("bad iota %d)", i.x))
	}
	v := make([]Value, i.x)
	for j := range v {
		v[j] = Int{x: int64(j) + 1} // TODO: assumes small
	}
	return ValueSlice(v)
}
