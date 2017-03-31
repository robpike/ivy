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

type BigRat struct {
	*big.Rat
}

// The input is known to be in floating-point syntax.
// If there's a slash, the parsing is done in Parse().
func setBigRatFromFloatString(_ *config.Config, s string) (br BigRat, err error) {
	// Be safe: Verify that it is floating-point, because otherwise
	// we need to honor ibase.
	if !strings.ContainsAny(s, ".eE") {
		// Most likely a number like "08".
		Errorf("bad number syntax: %s", s)
	}
	var ok bool
	r, ok := big.NewRat(0, 1).SetString(s)
	if !ok {
		return BigRat{}, errors.New("floating-point number syntax")
	}
	return BigRat{r}, nil
}

func (r BigRat) String() string {
	return "(" + r.Sprint(debugConf) + ")"
}

func (r BigRat) Sprint(conf *config.Config) string {
	format := conf.Format()
	if format != "" {
		verb, prec, ok := conf.FloatFormat()
		if ok {
			return r.floatString(verb, prec)
		}
		return fmt.Sprintf(conf.RatFormat(), r.Num(), r.Denom())
	}
	num := BigInt{r.Num()}
	den := BigInt{r.Denom()}
	return fmt.Sprintf("%s/%s", num.Sprint(conf), den.Sprint(conf))
}

func (r BigRat) ProgString() string {
	return fmt.Sprintf("%s/%s", r.Num(), r.Denom())
}

func (r BigRat) floatString(verb byte, prec int) string {
	switch verb {
	case 'f', 'F':
		return r.Rat.FloatString(prec)
	case 'e', 'E':
		// The exponent will alway be >= 0.
		sign := ""
		var x, t big.Rat
		x.Set(r.Rat)
		if x.Sign() < 0 {
			sign = "-"
			x.Neg(&x)
		}
		t.Set(&x)
		exp := ratExponent(&x)
		ratScale(&t, exp)
		str := t.FloatString(prec + 1) // +1 because first digit might be zero.
		// Drop the decimal.
		if str[0] == '0' {
			str = str[2:]
			exp--
		} else if len(str) > 1 && str[1] == '.' {
			str = str[0:1] + str[2:]
		}
		return eFormat(verb, prec, sign, str, exp)
	case 'g', 'G':
		var x big.Rat
		x.Set(r.Rat)
		exp := ratExponent(&x)
		// Exponent could be positive or negative
		if exp < -4 || prec <= exp {
			// Use e format.
			verb -= 2 // g becomes e.
			return trimEZeros(verb, r.floatString(verb, prec-1))
		}
		// Use f format.
		// If it's got zeros right of the decimal, they count as digits in the precision.
		// If it's got digits left of the decimal, they count as digits in the precision.
		// Both are handled by adjusting prec by exp.
		str := r.floatString(verb-1, prec-exp-1) // -1 for the one digit left of the decimal.
		// Trim trailing decimals.
		point := strings.IndexByte(str, '.')
		if point > 0 {
			n := len(str)
			for str[n-1] == '0' {
				n--
			}
			str = str[:n]
			if str[n-1] == '.' {
				str = str[:n-1]
			}
		}
		return str
	default:
		Errorf("can't handle verb %c for rational", verb)
	}
	return ""
}

var bigRatOne = big.NewRat(1, 1)
var bigRatTen = big.NewRat(10, 1)
var bigRatBillion = big.NewRat(1e9, 1)

// ratExponent returns the power of ten that x would display in scientific notation.
func ratExponent(x *big.Rat) int {
	if x.Sign() < 0 {
		x.Neg(x)
	}
	e := 0
	invert := false
	if x.Num().Cmp(x.Denom()) < 0 {
		invert = true
		x.Inv(x)
		e++
	}
	for x.Cmp(bigRatBillion) >= 0 {
		e += 9
		x.Quo(x, bigRatBillion)
	}
	for x.Cmp(bigRatTen) > 0 {
		e++
		x.Quo(x, bigRatTen)
	}
	if invert {
		return -e
	}
	return e
}

// ratScale multiplies x by 10**exp.
func ratScale(x *big.Rat, exp int) {
	if exp < 0 {
		x.Inv(x)
		ratScale(x, -exp)
		x.Inv(x)
		return
	}
	for exp >= 9 {
		x.Quo(x, bigRatBillion)
		exp -= 9
	}
	for exp >= 1 {
		x.Quo(x, bigRatTen)
		exp--
	}
}

func (r BigRat) Eval(Context) Value {
	return r
}

func (r BigRat) Inner() Value {
	return r
}

func (r BigRat) toType(conf *config.Config, which valueType) Value {
	switch which {
	case bigRatType:
		return r
	case bigFloatType:
		f := new(big.Float).SetPrec(conf.FloatPrec()).SetRat(r.Rat)
		return BigFloat{f}
	case vectorType:
		return NewVector([]Value{r})
	case matrixType:
		return NewMatrix([]Value{one, one}, []Value{r})
	}
	Errorf("cannot convert rational to %s", which)
	return nil
}

// shrink pulls, if possible, a BigRat down to a BigInt or Int.
func (r BigRat) shrink() Value {
	if !r.IsInt() {
		return r
	}
	return BigInt{r.Num()}.shrink()
}
