// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"
	"math/big"
	"strings"
)

type BigRat struct {
	*big.Rat
}

// The input is known to be in floating-point syntax.
// If there's a slash, the parsing is done in Parse().
// Bases allowed: 0, 2, 8, 10, or 16.
func setBigRatFromFloatString(c Context, s string) (br BigRat, err error) {
	// Be safe: Verify that it is floating-point, because otherwise
	// we need to honor arbitrary ibase.
	if !strings.ContainsAny(s, ".eEpP") {
		// Most likely a number like "08".
		c.Errorf("bad number syntax: %s", s)
		return BigRat{}, fmt.Errorf("bad number syntax: %q", s)
	}
	ibase := 0
	if c != nil { // Happens during const.go initialization, fixing would create import cycle.
		ibase = c.Config().InputBase()
	}
	if strings.Contains(s, "p") || strings.Contains(s, "P") {
		ibase = 16 // Force hexadecimal; overrides ibase.
	}
	// Rat.SetString uses prefix to indicate input base.
	var ok bool
	switch ibase {
	case 0, 10:
		// Base 10, the default.
		ok = true
	case 2:
		s, ok = basify(s, "0b", 2)
	case 8:
		s, ok = basify(s, "0o", 8)
	case 16:
		s, ok = basify(s, "0x", 16)
	}
	if !ok {
		return BigRat{}, fmt.Errorf("cannot input floating-point number in base %d", ibase)
	}
	r, ok := big.NewRat(0, 1).SetString(s)
	if !ok {
		return BigRat{}, fmt.Errorf("floating-point number syntax: %q", s)
	}
	return BigRat{r}, nil
}

// basify updates, if necessary, the string to add the prefix required for the base
// when parsed by big.Rat.SetString. The returned boolean will be false for
// unsupported bases.
func basify(s, prefix string, base int) (string, bool) {
	switch base {
	case 0, 10:
		return s, true
	case 2, 8, 16:
		if !strings.HasPrefix(s, prefix) {
			s = prefix + s
		}
		return s, true
	}
	return "", false
}

func (r BigRat) String() string {
	return "(" + r.Sprint(debugContext) + ")"
}

func (r BigRat) Rank() int {
	return 0
}

func (r BigRat) Sprint(c Context) string {
	conf := c.Config()
	format := conf.Format()
	if format != "" {
		verb, prec, ok := conf.FloatFormat()
		if ok {
			return r.floatString(c, verb, prec)
		}
		return fmt.Sprintf(conf.RatFormat(), r.Num(), r.Denom())
	}
	num := BigInt{r.Num()}
	den := BigInt{r.Denom()}
	return fmt.Sprintf("%s/%s", num.Sprint(c), den.Sprint(c))
}

func (r BigRat) ProgString() string {
	return fmt.Sprintf("%s/%s", r.Num(), r.Denom())
}

func (r BigRat) floatString(c Context, verb byte, prec int) string {
	switch verb {
	case 'f', 'F':
		return r.Rat.FloatString(prec)
	case 'e', 'E':
		// The exponent will always be >= 0.
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
			return trimEZeros(verb, r.floatString(c, verb, prec-1))
		}
		// Use f format.
		// If it's got zeros right of the decimal, they count as digits in the precision.
		// If it's got digits left of the decimal, they count as digits in the precision.
		// Both are handled by adjusting prec by exp.
		str := r.floatString(c, verb-1, prec-exp-1) // -1 for the one digit left of the decimal.
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
		c.Errorf("can't handle verb %c for rational", verb)
	}
	return ""
}

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

// inverse returns 1/r
func (r BigRat) inverse(c Context) Value {
	if r.Sign() == 0 {
		c.Errorf("inverse of zero")
	}
	return BigRat{
		Rat: big.NewRat(0, 1).SetFrac(r.Denom(), r.Num()),
	}.shrink()
}

func (r BigRat) Eval(Context) Value {
	return r
}

func (r BigRat) Inner() Value {
	return r
}

func (r BigRat) toType(op string, c Context, which valueType) Value {
	switch which {
	case bigRatType:
		return r
	case bigFloatType:
		f := new(big.Float).SetPrec(c.Config().FloatPrec()).SetRat(r.Rat)
		return BigFloat{f}
	case complexType:
		return NewComplex(c, r, zero)
	case vectorType:
		return oneElemVector(r)
	case matrixType:
		return NewMatrix(c, []int{1, 1}, NewVector(r))
	}
	c.Errorf("%s: cannot convert rational to %s", op, which)
	return nil
}

// shrink pulls, if possible, a BigRat down to a BigInt or Int.
func (r BigRat) shrink() Value {
	if !r.IsInt() {
		return r
	}
	return BigInt{r.Num()}.shrink()
}
