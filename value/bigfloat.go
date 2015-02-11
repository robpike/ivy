// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"errors"
	"math/big"
)

type BigFloat struct {
	*big.Float
}

// The fmt package looks for Formatter before Stringer, but we want
// to use Stringer only. big.Float implements Formatter,
// and we embed it in our BigFloat type. To make sure
// that our String gets called rather than the inner Format, we
// put a non-matching stub Format method into this interface.
// This is ugly but very simple and cheap.
func (i BigFloat) Format() {}

func setBigFloatString(s string) (BigFloat, error) {
	f, ok := big.NewFloat(0, conf.FloatPrec(), big.ToNearestEven).SetString(s)
	if !ok {
		return BigFloat{}, errors.New("float parse error")
	}
	return BigFloat{f}, nil
}

func (f BigFloat) String() string {
	format := conf.Format()
	if format != "" {
		verb, prec, ok := conf.FloatFormat()
		if ok {
			return f.Float.Format(verb, prec)
		}
	}
	return f.Float.Format('g', 6)
}

func (f BigFloat) Eval(Context) Value {
	return f
}

func (f BigFloat) toType(which valueType) Value {
	switch which {
	case intType:
		panic("bigfloat to int")
	case bigIntType:
		panic("bigfloat to bigint")
	case bigRatType:
		panic("bigfloat to bigrat")
	case bigFloatType:
		return f
	case vectorType:
		return NewVector([]Value{f})
	case matrixType:
		return newMatrix([]Value{one}, []Value{f})
	}
	panic("BigFloat.toType")
}

// shrink shrinks, if possible, a BigFloat down to an integer type.
func (f BigFloat) shrink() Value {
	_, exp := f.MantExp()
	if exp <= 100 && f.IsInt() { // Huge integers are not pretty. (Exp here is power of two.)
		i, _ := f.Int() // Result guaranteed exact.
		return BigInt{i}.shrink()
	}
	return f
}
