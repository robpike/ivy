// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

type BigInt struct {
	unimplemented
	x big.Int
}

func SetBigIntString(s string) (BigInt, ParseState) {
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
		return z.reduce()
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
		return z.reduce()
	case Int:
		return x.Sub(i).Neg()
	}
	panic(Errorf("unimplemented Sub(BigInt, %T)", x))
}

func (i BigInt) Mul(x Value) Value {
	switch x := x.(type) {
	case BigInt:
		var z BigInt
		z.x.Mul(&i.x, &x.x)
		return z.reduce()
	case Int:
		return x.Mul(i)
	}
	panic(Errorf("unimplemented Mul(BigInt, %T)", x))
}

func (i BigInt) Div(x Value) Value {
	switch x := x.(type) {
	case BigInt:
		var z BigInt
		z.x.Div(&i.x, &x.x)
		return z.reduce()
	case Int:
		return i.Div(BigInt64(x.x))
	}
	panic(Errorf("unimplemented Div(BigInt, %T)", x))
}

func (i BigInt) Pow(x Value) Value {
	switch x := x.(type) {
	case BigInt:
		var z BigInt
		z.x.Exp(&i.x, &x.x, nil)
		return z.reduce()
	case Int:
		return i.Pow(BigInt64(x.x))
	}
	panic(Errorf("unimplemented Div(BigInt, %T)", x))
}

func (i BigInt) And(x Value) Value {
	switch x := x.(type) {
	case BigInt:
		var z BigInt
		z.x.And(&i.x, &x.x)
		return z.reduce()
	case Int:
		return x.And(i)
	}
	panic(Errorf("unimplemented And(BigInt, %T)", x))
}

func (i BigInt) Or(x Value) Value {
	switch x := x.(type) {
	case BigInt:
		var z BigInt
		z.x.Or(&i.x, &x.x)
		return z.reduce()
	case Int:
		return x.Or(i)
	}
	panic(Errorf("unimplemented Or(BigInt, %T)", x))
}

func (i BigInt) Xor(x Value) Value {
	switch x := x.(type) {
	case BigInt:
		var z BigInt
		z.x.Xor(&i.x, &x.x)
		return z.reduce()
	case Int:
		return x.Xor(i)
	}
	panic(Errorf("unimplemented Xor(BigInt, %T)", x))
}

func (i BigInt) Lsh(x Value) Value {
	var z BigInt
	z.x.Lsh(&i.x, shiftCount(x))
	return z.reduce()
}

func (i BigInt) Rsh(x Value) Value {
	var z BigInt
	z.x.Rsh(&i.x, shiftCount(x))
	return z.reduce()
}

func (i BigInt) Neg() Value {
	var z BigInt
	z.x.Neg(&i.x)
	return z.reduce()
}
