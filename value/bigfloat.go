// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"errors"
	"fmt"
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
	f, ok := new(big.Float).SetPrec(conf.FloatPrec()).SetString(s)
	if !ok {
		return BigFloat{}, errors.New("float parse error")
	}
	return BigFloat{f}, nil
}

const fastFloatPrint = true

func (f BigFloat) String() string {
	var mant big.Float
	exp := f.Float.MantExp(&mant)
	positive := 1
	if exp < 0 {
		positive = 0
		exp = -exp
	}
	verb, prec := byte('g'), 12
	format := conf.Format()
	if format != "" {
		v, p, ok := conf.FloatFormat()
		if ok {
			verb, prec = v, p
		}
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
		fexp := newF().SetInt64(int64(exp))
		fexp.Mul(fexp, floatLog2)
		fexp.Quo(fexp, floatLog10)
		// We now have a floating-point base 10 exponent.
		// Break into the integer part and the fractional part.
		// The integer part is what we will show.
		// The 10**(fractional part) will be multiplied back in.
		iexp, _ := fexp.Int(nil)
		fraction := fexp.Sub(fexp, newF().SetInt(iexp))
		// Now compute 10**(fractional part).
		// Fraction is in base 10. Move it to base e.
		fraction.Mul(fraction, floatLog10)
		scale := exponential(fraction)
		if positive > 0 {
			mant.Mul(&mant, scale)
		} else {
			mant.Quo(&mant, scale)
		}
		ten := newF().SetInt64(10)
		i64exp := iexp.Int64()
		// For numbers not too far from one, print without the E notation.
		// Shouldn't happen (exp must be large to get here) but just
		// in case, we keep this around.
		if -4 <= i64exp && i64exp <= 11 {
			if i64exp > 0 {
				for i := 0; i < int(i64exp); i++ {
					mant.Mul(&mant, ten)
				}
			} else {
				for i := 0; i < int(-i64exp); i++ {
					mant.Quo(&mant, ten)
				}
			}
			fmt.Sprintf("%g\n", &mant)
		} else {
			sign := ""
			if mant.Sign() < 0 {
				sign = "-"
				mant.Neg(&mant)
			}
			// If it has a leading zero, rescale.
			digits := mant.Text('g', prec)
			for digits[0] == '0' {
				mant.Mul(&mant, ten)
				if positive > 0 {
					i64exp--
				} else {
					i64exp++
				}
				digits = mant.Text('g', prec)
			}
			return fmt.Sprintf("%s%s%c%c%d", sign, digits, eChar, "-+"[positive], i64exp)
		}
	}
	return f.Float.Text(verb, prec)
}

func (f BigFloat) ProgString() string {
	// There is no such thing as a float literal in program listings.
	panic("float.ProgString - cannot happen")
}

func (f BigFloat) Eval(Context) Value {
	return f
}

func (f BigFloat) toType(which valueType) Value {
	switch which {
	case bigFloatType:
		return f
	case vectorType:
		return NewVector([]Value{f})
	case matrixType:
		return NewMatrix([]Value{one}, []Value{f})
	}
	Errorf("cannot convert float to %s", which)
	return nil
}

var floatTmp big.Float // For use in shrink only!. TODO: Delete this and use nil when possible.

// shrink shrinks, if possible, a BigFloat down to an integer type.
func (f BigFloat) shrink() Value {
	exp := f.MantExp(&floatTmp)
	if exp <= 100 && f.IsInt() { // Huge integers are not pretty. (Exp here is power of two.)
		i, _ := f.Int(nil) // Result guaranteed exact.
		return BigInt{i}.shrink()
	}
	return f
}
