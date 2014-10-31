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
	*big.Int
}

// The fmt package looks for Formatter before Stringer, but we want
// to use Stringer only. big.Int and big.Rat implement Formatter,
// and we embed them in our BigInt and BigRat types. To make sure
// that our String gets called rather than the inner Format, we
// put a non-matching stub Format method into this interface.
// This is ugly but very simple and cheap.
func (i BigInt) Format() {}

func SetBigIntString(s string) (BigInt, error) {
	i, ok := big.NewInt(0).SetString(s, 0)
	if !ok {
		return BigInt{}, errors.New("integer parse error")
	}
	return BigInt{i}, nil
}

func (i BigInt) String() string {
	return fmt.Sprintf(conf.Format(), i.Int)
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
		r := big.NewRat(0, 1).SetInt(i.Int)
		return BigRat{r}
	case vectorType:
		return ValueSlice([]Value{i})
	case matrixType:
		return ValueMatrix([]Value{one, one}, []Value{i})
	}
	panic("BigInt.ToType")
}

// shrink shrinks, if possible, a BigInt down to an Int.
func (i BigInt) shrink() Value {
	if i.BitLen() < intBits {
		return Int(i.Int64())
	}
	return i
}
