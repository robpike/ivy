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
	i, ok := big.NewInt(0).SetString(s, conf.InputBase())
	if !ok {
		return BigInt{}, errors.New("integer parse error")
	}
	return BigInt{i}, nil
}

func (i BigInt) String() string {
	format := conf.Format()
	if format != "" {
		verb, prec, ok := conf.FloatFormat()
		if ok {
			return i.floatString(verb, prec)
		}
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

func (i BigInt) floatString(verb byte, prec int) string {
	switch verb {
	case 'f', 'F':
		str := fmt.Sprintf("%d", i.Int)
		if prec > 0 {
			str += "." + zeros(prec)
		}
		return str
	case 'e', 'E':
		// The exponent will alway be >= 0.
		sign := ""
		var x big.Int
		x.Set(i.Int)
		if x.Sign() < 0 {
			sign = "-"
			x.Neg(&x)
		}
		return eFormat(verb, prec, sign, x.String(), eExponent(&x))
	case 'g', 'G':
		// Exponent is always positive so it's easy.
		var x big.Int
		x.Set(i.Int)
		if eExponent(&x) >= prec {
			// Use e format:
			return i.floatString(verb-2, prec-1)
		}
		// use f format, but this is just an integer,
		return fmt.Sprintf("%d", i.Int)
	default:
		Errorf("can't handle verb %c for big int", verb)
	}
	return ""
}

var bigIntTen = big.NewInt(10)
var bigIntBillion = big.NewInt(1e9)

// eExponent returns the exponent to use to display i in 1.23e+04 format.
func eExponent(x *big.Int) int {
	if x.Sign() < 0 {
		x.Neg(x)
	}
	e := 0
	for x.Cmp(bigIntBillion) >= 0 {
		e += 9
		x.Quo(x, bigIntBillion)
	}
	for x.Cmp(bigIntTen) >= 0 {
		e++
		x.Quo(x, bigIntTen)
	}
	return e
}

func (i BigInt) Eval(Context) Value {
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
