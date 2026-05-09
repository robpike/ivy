// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"
	"math/big"
	"strings"
	"sync"
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
func (f BigFloat) Format() {}

func (f BigFloat) Rank() int {
	return 0
}

const fastFloatPrint = true

func (f BigFloat) String() string {
	return "(" + f.Sprint(debugContext) + ")"
}

func (f BigFloat) Sprint(c Context) string {
	conf := c.Config()
	verb, prec := byte('g'), 12 // prec is number of digits after the decimal.
	format := conf.Format()
	if format != "" {
		v, p, ok := conf.FloatFormat()
		if ok {
			verb, prec = v, p
		}
	}
	if base := conf.OutputBase(); base != 0 && base != 10 {
		return nonDecimalBaseFloatString(c, f.Float, base, prec, verb)
	}
	var mant big.Float
	exp := f.Float.MantExp(&mant)
	positive := 1
	if exp < 0 {
		positive = 0
		exp = -exp
	}
	// Printing huge floats can be very slow using
	// big.Float's native methods; see issue #11068.
	// For example 1e5000000 takes a minute of CPU time just
	// to print. The code below is instantaneous, by rescaling
	// first. It is however less feature-complete.
	// (Big ints are problematic too, but if you print 1e50000000
	// as an integer you probably won't be surprised it's slow.)
	if fastFloatPrint && exp > 10000 {
		// We always use %g to print the fraction, and it will
		// never have an exponent, but if the format is %E we
		// need to use a capital E.
		eChar := 'e'
		if verb == 'E' || verb == 'G' {
			eChar = 'E'
		}
		// Up to precision 10000, the result is off by 4 decimal digits.
		// Add at least 4×ln(10)/ln(2) bits of precision.
		fprec := addPrec(conf.FloatPrec(), 16)
		fexp := newFloatPrec(fprec).SetInt64(int64(exp))
		fexp.Mul(fexp, floatLog2)
		fexp.Quo(fexp, floatLog10)
		// We now have a floating-point base 10 exponent.
		// Break into the integer part and the fractional part.
		// The integer part is what we will show.
		// The 10**(fractional part) will be multiplied back in.
		iexp, _ := fexp.Int(nil)
		fraction := fexp.Sub(fexp, newFloatPrec(fprec).SetInt(iexp))
		// Now compute 10**(fractional part).
		// Fraction is in base 10. Move it to base e.
		fraction.Mul(fraction, floatLog10)
		scale := exponential(c, &big.Float{}, fraction)
		sign := ""
		if mant.Sign() < 0 {
			sign = "-"
			mant.Neg(&mant)
		}
		mant.SetPrec(fprec)
		if positive > 0 {
			mant.Mul(&mant, scale)
		} else {
			mant.Quo(&mant, scale)
		}
		i64exp := iexp.Int64()
		// If it has a leading 0 rescale.
		digits := mant.Text('g', prec)
		if digits[0] == '0' {
			mant.Mul(&mant, new(big.Float).SetUint64(10))
			if positive > 0 {
				i64exp--
			} else {
				i64exp++
			}
			digits = mant.Text('g', prec)
		}
		// Print with the E notation for numbers far enough from one.
		// This should always be the case (exp must be large to get here) but just
		// in case, we keep this check around and fallback to big.Float.Text.
		if i64exp < -4 || 11 < i64exp {
			return fmt.Sprintf("%s%s%c%c%d", sign, digits, eChar, "-+"[positive], i64exp)
		}
	}
	return f.Float.Text(verb, prec)
}

var (
	zs        = ""
	zerosLock sync.Mutex
)

// zeros returns a string of n zeros.
func zeros(n int) string {
	zerosLock.Lock()
	defer zerosLock.Unlock()
	if len(zs) < n {
		zs = strings.Repeat("0", n+10)
	}
	return zs[:n]
}

var binaryToOctal = map[string]byte{
	"000": '0',
	"001": '1',
	"010": '2',
	"011": '3',
	"100": '4',
	"101": '5',
	"110": '6',
	"111": '7',
}

// nonDecimalBaseFloatString formats a float in a non-decimal base.
// It always prints with mantissa in [½, 1) or 0, followed by
// a power of two exponent introduced by a 'p'.
// TODO: Implement 'f' and 'g' formats.
func nonDecimalBaseFloatString(c Context, f *big.Float, base, prec int, verb byte) string {
	switch base {
	case 2, 8, 16:
	default:
		c.Errorf("cannot print floats in base %d", base)
	}
	var mant big.Float
	exp := f.MantExp(&mant)
	// For now, we ignore the verb except to set the case of the exponent marker.
	// Also, for these bases we always show the exponent as a power of two;
	// otherwise there is a lot more work to do.
	expChar := 'p'
	if 'A' <= verb && verb <= 'Z' {
		expChar = 'P'
	}
	signChar := '+'
	switch f.Sign() {
	case 0:
		return fmt.Sprintf("+0.%s%c+00", zeros(prec), expChar)
	case -1:
		mant.Neg(&mant)
		signChar = '-'
	}
	// Convert mantissa to a hexadecimal 'p' format, which we can then rework
	// as bits. TODO: Would be much nicer if we MantExp gave is an integer for
	// the mantissa.
	str := mant.Text('p', int(c.Config().FloatPrec()))
	pLoc := strings.IndexByte(str, 'p')
	if !strings.HasPrefix(str, "0x.") || pLoc < 0 {
		c.Errorf("internal error formatting float in base %d; %q", base, str)
	}
	mStr := str[3:pLoc]
	var b strings.Builder
	s := mStr
	bufSize := 2 * prec // Headroom.
	// Safety first.
	if prec < 2 {
		bufSize = 12
	}
	if base == 8 {
		bufSize *= 3 // Headroom, plus we generate bits first, three per digit.
	}
	for i := 0; b.Len() < bufSize; i++ {
		c := byte(0)
		if i < len(s) {
			c = s[i]
			if c <= '9' {
				c -= '0'
			} else {
				c -= 'a' - 10
			}
		}
		switch base {
		case 2, 8:
			// Base 8 needs three at a time. Convert to binary first, fix below.
			for mask := byte(8); mask > 0; mask >>= 1 {
				if c&mask != 0 {
					b.WriteByte('1')
				} else {
					b.WriteByte('0')
				}
			}
		case 16:
			if c <= 9 {
				b.WriteByte('0' + c)
			} else {
				b.WriteByte('a' + c - 10)
			}
		}
	}
	s = b.String()
	// Base 8 consumes 3 bits at a time, so the loop above is unsound for octal.
	// That's why we process it as base 2 above, and then translate here.
	if base == 8 {
		b.Reset()
		for len(s) >= 3 {
			b.WriteByte(binaryToOctal[s[:3]])
			s = s[3:]
		}
		s = b.String()
	}
	mantissa := s[:prec]
	return fmt.Sprintf("%c0.%s%c%+.2d", signChar, mantissa, expChar, exp)
}

// inverse returns 1/f
func (f BigFloat) inverse(c Context) Value {
	if f.Sign() == 0 {
		c.Errorf("inverse of zero")
	}
	var one big.Float
	one.Set(floatOne) // Avoid big.Float.Copy, which appears to have a sharing bug.
	result := BigFloat{
		Float: one.Quo(&one, f.Float),
	}.shrink()
	return result
}

func (f BigFloat) ProgString() string {
	// There is no such thing as a float literal in program listings.
	panic("float.ProgString - cannot happen")
}

func (f BigFloat) Eval(Context) Value {
	return f
}

func (f BigFloat) Inner() Value {
	return f
}

func (f BigFloat) toType(op string, c Context, which valueType) Value {
	switch which {
	case bigFloatType:
		return f
	case complexType:
		return NewComplex(c, f, zero)
	case vectorType:
		return oneElemVector(f)
	case matrixType:
		return NewMatrix(c, []int{1}, NewVector(f))
	}
	c.Errorf("%s: cannot convert float to %s", op, which)
	return nil
}

// shrink shrinks, if possible, a BigFloat down to an integer type.
func (f BigFloat) shrink() Value {
	exp := f.MantExp(nil)
	if exp <= 100 && f.IsInt() { // Huge integers are not pretty. (Exp here is power of two.)
		i, _ := f.Int(nil) // Result guaranteed exact.
		return BigInt{i}.shrink()
	}
	return f
}
