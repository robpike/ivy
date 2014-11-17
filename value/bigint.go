// Copyright 2014 Rob Pike. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value // import "robpike.io/ivy/value"

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

func setBigIntString(s string) (BigInt, error) {
	base := conf.InputBase()
	// Tricky but good enough for now.
	switch base {
	case 0, 10: // Fine as is.
	case 8:
		s = "0" + s
	case 16:
		s = "0x" + s
	default:
		Errorf("can't handle base %d parsing big int %s", base, s)
	}
	i, ok := big.NewInt(0).SetString(s, 0)
	if !ok {
		return BigInt{}, errors.New("integer parse error")
	}
	return BigInt{i}, nil
}

func (i BigInt) String() string {
	format := conf.Format()
	if format != "" {
		return fmt.Sprintf(format, i.Int)
	}
	// Is this from a rational and we could use an int?
	if i.BitLen() < intBits {
		return Int(i.Int64()).String()
	}
	switch conf.OutputBase() {
	case 0, 10:
		return fmt.Sprintf("%d", i.Int)
	case 2:
		return fmt.Sprintf("%b", i.Int)
	case 8:
		return fmt.Sprintf("%o", i.Int)
	case 16:
		return fmt.Sprintf("%x", i.Int)
	}
	Errorf("can't print number in base %d (yet)", conf.OutputBase())
	return ""
}

func (i BigInt) Eval() Value {
	return i
}

func (i BigInt) toType(which valueType) Value {
	switch which {
	case intType:
		panic("bigint to int")
	case bigIntType:
		return i
	case bigRatType:
		r := big.NewRat(0, 1).SetInt(i.Int)
		return BigRat{r}
	case vectorType:
		return NewVector([]Value{i})
	case matrixType:
		return newMatrix([]Value{one}, []Value{i})
	}
	panic("BigInt.toType")
}

// shrink shrinks, if possible, a BigInt down to an Int.
func (i BigInt) shrink() Value {
	if i.BitLen() < intBits {
		return Int(i.Int64())
	}
	return i
}
