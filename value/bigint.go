// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"robpike.io/ivy/config"
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

func setBigIntString(conf *config.Config, s string) (BigInt, error) {
	i, ok := big.NewInt(0).SetString(s, conf.InputBase())
	if !ok {
		return BigInt{}, errors.New("integer parse error")
	}
	return BigInt{i}, nil
}

func (i BigInt) String() string {
	return "(" + i.Sprint(debugConf) + ")"
}

func (i BigInt) Sprint(conf *config.Config) string {
	bitLen := i.BitLen()
	format := conf.Format()
	var maxBits = (uint64(conf.MaxDigits()) * 33222) / 10000 // log 10 / log 2 is 3.32192809489
	if uint64(bitLen) > maxBits && maxBits != 0 {
		// Print in floating point.
		return BigFloat{newF(conf).SetInt(i.Int)}.Sprint(conf)
	}
	if format != "" {
		verb, prec, ok := conf.FloatFormat()
		if ok {
			return i.floatString(verb, prec)
		}
		return fmt.Sprintf(format, i.Int)
	}
	// Is this from a rational and we could use an int?
	if i.BitLen() < intBits {
		return Int(i.Int64()).Sprint(conf)
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

func (i BigInt) ProgString() string {
	return fmt.Sprintf("%d", i.Int)
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
			// Use e format.
			verb -= 2 // g becomes e.
			return trimEZeros(verb, i.floatString(verb, prec-1))
		}
		// Use f format, but this is just an integer.
		return fmt.Sprintf("%d", i.Int)
	default:
		Errorf("can't handle verb %c for big int", verb)
	}
	return ""
}

var (
	bigIntTen     = big.NewInt(10)
	bigIntMillion = big.NewInt(1e6)
	bigIntBillion = big.NewInt(1e9)
	MaxBigInt63   = big.NewInt(int64(^uint64(0) >> 1))
)

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

func (i BigInt) Inner() Value {
	return i
}

func (i BigInt) toType(conf *config.Config, which valueType) Value {
	switch which {
	case bigIntType:
		return i
	case bigRatType:
		r := big.NewRat(0, 1).SetInt(i.Int)
		return BigRat{r}
	case bigFloatType:
		f := new(big.Float).SetPrec(conf.FloatPrec()).SetInt(i.Int)
		return BigFloat{f}
	case vectorType:
		return NewVector([]Value{i})
	case matrixType:
		return NewMatrix([]Value{one}, []Value{i})
	}
	Errorf("cannot convert big int to %s", which)
	return nil
}

// trimEZeros takes an e or E format string and deletes
// trailing zeros and maybe the decimal from the string.
func trimEZeros(e byte, s string) string {
	eLoc := strings.IndexByte(s, e)
	if eLoc < 0 {
		return s
	}
	n := eLoc
	for s[n-1] == '0' {
		n--
	}
	if s[n-1] == '.' {
		n--
	}
	return s[:n] + s[eLoc:]
}

// shrink shrinks, if possible, a BigInt down to an Int.
func (i BigInt) shrink() Value {
	if i.BitLen() < intBits {
		return Int(i.Int64())
	}
	return i
}

func (i BigInt) BitLen() int64 {
	return int64(i.Int.BitLen())
}

// mustFit errors out if n is larger than the maximum number of bits allowed.
func mustFit(conf *config.Config, n int64) {
	max := conf.MaxBits()
	if max != 0 && n > int64(max) {
		Errorf("result too large (%d bits)", n)
	}
}
