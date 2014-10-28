// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"
	"math/big"
	"strings"
)

type Expr interface {
	String() string

	Eval() Value
}

type Value interface {
	String() string
	Eval() Value

	ToType(valueType) Value
}

type Error string

func (err Error) Error() string {
	return string(err)
}

func Errorf(format string, args ...interface{}) Error {
	return Error(fmt.Sprintf(format, args...))
}

type ParseState int

func ValueString(s string) (Value, error) {
	// Is it a rational? If so, it's tricky.
	if strings.ContainsRune(s, '/') {
		elems := strings.Split(s, "/")
		if len(elems) != 2 {
			panic("bad rat")
		}
		num, err := ValueString(elems[0])
		if err != nil {
			return nil, err
		}
		den, err := ValueString(elems[1])
		if err != nil {
			return nil, err
		}
		// Common simple case.
		if whichType(num) == intType && whichType(den) == intType {
			return bigRatTwoInt64s(num.(Int).x, den.(Int).x).shrink(), nil
		}
		// General mix-em-up.
		rden := den.ToType(bigRatType)
		if z := rden.(BigRat).x; z.Sign() == 0 {
			panic(Error("zero denominator in rational"))
		}
		return binaryBigRatOp(num.ToType(bigRatType), (*big.Rat).Quo, rden), nil
	}
	// Not a rational, but might be something like 1.3e-2 and therefore
	// become a rational.
	i, err := SetIntString(s)
	if err == nil {
		return i, nil
	}
	b, err := SetBigIntString(s)
	if err == nil {
		return b.shrink(), nil
	}
	r, err := SetBigRatString(s)
	if err == nil {
		return r.shrink(), nil
	}
	return nil, err
}

func valueInt64(x int64) Value {
	if minInt <= x && x <= maxInt {
		return Int{x: x}
	}
	return bigInt64(x)
}

func bigInt64(x int64) BigInt {
	return BigInt{x: big.NewInt(x)}
}

func bigRatInt64(x int64) BigRat {
	return bigRatTwoInt64s(x, 1)
}

func bigRatTwoInt64s(x, y int64) BigRat {
	if y == 0 {
		panic(Error("zero denominator in rational"))
	}
	return BigRat{x: big.NewRat(x, y)}
}
