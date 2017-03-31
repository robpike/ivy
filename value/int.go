// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"
	"strconv"

	"robpike.io/ivy/config"
)

// Int is not only the simplest representation, it provides the operands that mix
// types upward. That is, BigInt.Add(Int) will be done by rewriting as Int.Add(BigInt).

type Int int64

const (
	// We use an int32 size, so multiplications will fit in int64
	// and can be scaled afterwards.
	intBits = 32
	minInt  = -(1 << (intBits - 1))
	maxInt  = 1<<(intBits-1) - 1
)

func setIntString(conf *config.Config, s string) (Int, error) {
	i, err := strconv.ParseInt(s, conf.InputBase(), intBits)
	return Int(i), err
}

func (i Int) String() string {
	return "(" + i.Sprint(debugConf) + ")"
}

func (i Int) Sprint(conf *config.Config) string {
	format := conf.Format()
	if format != "" {
		verb, prec, ok := conf.FloatFormat()
		if ok {
			return i.floatString(verb, prec)
		}
		return fmt.Sprintf(format, int64(i))
	}
	base := conf.OutputBase()
	if base == 0 {
		base = 10
	}
	return strconv.FormatInt(int64(i), base)
}

func (i Int) ProgString() string {
	return strconv.FormatInt(int64(i), 10)
}

func (i Int) floatString(verb byte, prec int) string {
	switch verb {
	case 'f', 'F':
		str := strconv.FormatInt(int64(i), 10)
		if prec > 0 {
			str += "." + zeros(prec)
		}
		return str
	case 'e', 'E':
		sign := ""
		if i < 0 {
			sign = "-"
			i = -i
		}
		return eFormat(verb, prec, sign, strconv.FormatInt(int64(i), 10), i.eExponent())
	case 'g', 'G':
		// Exponent is always positive so it's easy.
		if i.eExponent() >= prec {
			// Use e format.
			return i.floatString(verb-2, prec-1)
		}
		// Use f format, but this is just an integer.
		return fmt.Sprintf("%d", int64(i))
	default:
		Errorf("can't handle verb %c for int", verb)
	}
	return ""
}

// eExponent returns the exponent to use to display i in 1.23e+04 format.
func (i Int) eExponent() int {
	if i < 0 {
		i = -i
	}
	// The exponent will alway be >= 0.
	exp := 0
	x := i
	for x >= 10 {
		exp++
		x /= 10
	}
	return exp
}

// eFormat returns the %e/%E form of the number represented by the
// string str, which is a decimal integer, scaled by 10**exp.
func eFormat(verb byte, prec int, sign, str string, exp int) string {
	if len(str)-1 < prec {
		// Zero pad.
		str += zeros(prec - len(str) + 1)
	} else {
		// Truncate.
		// TODO: rounding
		str = str[:1+prec]
	}
	period := "."
	if prec == 0 {
		period = ""
	}
	return fmt.Sprintf("%s%s%s%s%c%+.2d", sign, str[0:1], period, str[1:], verb, exp)
}

var manyZeros = "0000000000"

func zeros(prec int) string {
	for len(manyZeros) < prec {
		manyZeros += manyZeros
	}
	return manyZeros[:prec]
}

var buf []byte

func (i Int) Eval(Context) Value {
	return i
}

func (i Int) Inner() Value {
	return i
}

func (i Int) toType(conf *config.Config, which valueType) Value {
	switch which {
	case intType:
		return i
	case bigIntType:
		return bigInt64(int64(i))
	case bigRatType:
		return bigRatInt64(int64(i))
	case bigFloatType:
		return bigFloatInt64(conf, int64(i))
	case vectorType:
		return NewVector([]Value{i})
	case matrixType:
		return NewMatrix([]Value{one}, []Value{i})
	}
	Errorf("cannot convert int to %s", which)
	return nil
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
