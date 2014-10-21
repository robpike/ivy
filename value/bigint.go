// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

type BigInt struct {
	unimplemented
	x big.Int
}

func SetBigInt(s string) (BigInt, ParseState) {
	var i BigInt
	_, ok := i.x.SetString(s, 0)
	if !ok {
		return BigInt{}, Fail
	}
	return i, Valid
}

func (i BigInt) String() string {
	return i.x.String()
}

func (i BigInt) Eval() Value {
	return i
}

// reduce pulls, if possible, a BigInt down to an Int.
func (i BigInt) reduce() Value {
	if i.x.BitLen() < intBits {
		return Int{x: i.x.Int64()}
	}
	return i
}

func (i BigInt) Add(x Value) Value {
	switch x := x.(type) {
	case BigInt:
		var z BigInt
		z.x.Add(&i.x, &x.x)
		return z
	case Int:
		return x.Add(i)
	}
	panic(Errorf("unimplemented Add(BigInt, %T)", x))
}

func (i BigInt) Sub(x Value) Value {
	switch x := x.(type) {
	case BigInt:
		var z BigInt
		z.x.Sub(&i.x, &x.x)
		return z
	case Int:
		return x.Sub(i)
	}
	panic(Errorf("unimplemented Sub(BigInt, %T)", x))
}

func (i BigInt) Mul(x Value) Value {
	switch x := x.(type) {
	case BigInt:
		var z BigInt
		z.x.Mul(&i.x, &x.x)
		return z
	case Int:
		return x.Mul(i)
	}
	panic(Errorf("unimplemented Mul(BigInt, %T)", x))
}

func (i BigInt) Neg() Value {
	var z BigInt
	z.x.Neg(&i.x)
	return z.reduce()
}
