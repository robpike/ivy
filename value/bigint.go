// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"errors"
	"fmt"
	"math/big"
)

type BigInt struct {
	x *big.Int
}

func SetBigIntString(s string) (BigInt, error) {
	i, ok := big.NewInt(0).SetString(s, 0)
	if !ok {
		return BigInt{}, errors.New("integer parse error")
	}
	return BigInt{x: i}, nil
}

func (i BigInt) String() string {
	return fmt.Sprintf(format, i.x)
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
	case bigRatType:
		r := big.NewRat(0, 1).SetInt(i.x)
		return BigRat{x: r}
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
