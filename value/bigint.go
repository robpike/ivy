// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

type BigInt struct {
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

func (i BigInt) ToType(which valueType) Value {
	switch which {
	case intType:
		panic("bigint to int")
	case bigIntType:
		return i
	case vectorType:
		return ValueSlice([]Value{i})
	}
	panic("BigInt.ToType")
}

// reduce pulls, if possible, a BigInt down to an Int.
func (i BigInt) reduce() Value {
	if i.x.BitLen() < intBits {
		return Int{x: i.x.Int64()}
	}
	return i
}
