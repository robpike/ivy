// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"errors"
	"math/big"
)

type BigInt struct {
	x big.Int
}

func SetBigIntString(s string) (BigInt, error) {
	var i BigInt
	_, ok := i.x.SetString(s, 0)
	if !ok {
		return BigInt{}, errors.New("integer parse error")
	}
	return i, nil
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

// shrink shrinks, if possible, a BigInt down to an Int.
func (i BigInt) shrink() Value {
	if i.x.BitLen() < intBits {
		return Int{x: i.x.Int64()}
	}
	return i
}
