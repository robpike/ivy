// Copyright 2014 Rob Pike. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value // import "robpike.io/ivy/value"

import (
	"fmt"
	"strconv"
)

// Int is not only the simplest representation, it provides the operands that mix
// types upward. That is, BigInt.Add(Int) will be done by rewriting as Int.Add(BigInt).

type Int int64

const (
	intBits = 32
	minInt  = -(1 << (intBits - 1))
	maxInt  = 1<<(intBits-1) - 1
)

func setIntString(s string) (Int, error) {
	i, err := strconv.ParseInt(s, conf.InputBase(), intBits)
	return Int(i), err
}

func (i Int) String() string {
	format := conf.Format()
	if format != "" {
		return fmt.Sprintf(format, int64(i))
	}
	base := conf.OutputBase()
	if base == 0 {
		base = 10
	}
	return strconv.FormatInt(int64(i), base)
}

var buf []byte

func (i Int) Eval(*Context) Value {
	return i
}

func (i Int) toType(which valueType) Value {
	switch which {
	case intType:
		return i
	case bigIntType:
		return bigInt64(int64(i))
	case bigRatType:
		return bigRatInt64(int64(i))
	case vectorType:
		return NewVector([]Value{i})
	case matrixType:
		return newMatrix([]Value{one}, []Value{i})
	}
	panic("Int.toType")
}

func (i Int) ToBool() bool {
	return i != 0
}

func (i Int) maybeBig() Value {
	if minInt <= i && i <= maxInt {
		return i
	}
	return bigInt64(int64(i))
}
