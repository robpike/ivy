// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

func asin(c Context, v Value) Value {
	return evalFloatFunc(c, v, floatAsin)
}

func acos(c Context, v Value) Value {
	return evalFloatFunc(c, v, floatAcos)
}

func atan(c Context, v Value) Value {
	return evalFloatFunc(c, v, floatAtan)
}

// floatAsin computes asin(x) using the formula asin(x) = atan(x/sqrt(1-x²)).
func floatAsin(c Context, x *big.Float) *big.Float {
	// The asin Taylor series converges very slowly near ±1, but our
	// atan implementation converges well for all values, so we use
	// the formula above to compute asin. But be careful when |x|=1.
	if x.Cmp(floatOne) == 0 {
		z := newFloat(c).Set(floatPi)
		return z.Quo(z, floatTwo)
	}
	if x.Cmp(floatMinusOne) == 0 {
		z := newFloat(c).Set(floatPi)
		z.Quo(z, floatTwo)
		return z.Neg(z)
	}
	z := newFloat(c)
	z.Mul(x, x)
	z.Sub(floatOne, z)
	z = floatSqrt(c, z)
	z.Quo(x, z)
	return floatAtan(c, z)
}

// floatAcos computes acos(x) as π/2 - asin(x).
func floatAcos(c Context, x *big.Float) *big.Float {
	// acos(x) = π/2 - asin(x)
	z := newFloat(c).Set(floatPi)
	z.Quo(z, newFloat(c).SetInt64(2))
	return z.Sub(z, floatAsin(c, x))
}

// floatAtan computes atan(x) using a Taylor series. There are two series,
// one for |x| < 1 and one for larger values.
func floatAtan(c Context, x *big.Float) *big.Float {
	// atan(-x) == -atan(x). Do this up top to simplify the Euler crossover calculation.
	if x.Sign() < 0 {
		z := newFloat(c).Set(x)
		z = floatAtan(c, z.Neg(z))
		return z.Neg(z)
	}

	// The series converge very slowly near 1. atan 1.00001 takes over a million
	// iterations at the default precision. But there is hope, an Euler identity:
	//	atan(x) = atan(y) + atan((x-y)/(1+xy))
	// Note that y is a free variable. If x is near 1, we can use this formula
	// to push the computation to values that converge faster. Because
	//	tan(π/8) = √2 - 1, or equivalently atan(√2 - 1) == π/8
	// we choose y = √2 - 1 and then we only need to calculate one atan:
	//	atan(x) = π/8 + atan((x-y)/(1+xy))
	// Where do we cross over? This version converges significantly faster
	// even at 0.5, but we must be careful that (x-y)/(1+xy) never approaches 1.
	// At x = 0.5, (x-y)/(1+xy) is 0.07; at x=1 it is 0.414214; at x=1.5 it is
	// 0.66, which is as big as we dare go. With 256 bits of precision and a
	// crossover at 0.5, here are the number of iterations done by
	//	atan .1*iota 20
	// 0.1 39, 0.2 55, 0.3 73, 0.4 96, 0.5 126, 0.6 47, 0.7 59, 0.8 71, 0.9 85, 1.0 99, 1.1 116, 1.2 38, 1.3 44, 1.4 50, 1.5 213, 1.6 183, 1.7 163, 1.8 147, 1.9 135, 2.0 125
	tmp := newFloat(c).Set(floatOne)
	tmp.Sub(tmp, x)
	tmp.Abs(tmp)
	if tmp.Cmp(newFloat(c).SetFloat64(0.5)) < 0 {
		z := newFloat(c).Set(floatPi)
		z.Quo(z, newFloat(c).SetInt64(8))
		y := floatSqrt(c, floatTwo)
		y.Sub(y, floatOne)
		num := newFloat(c).Set(x)
		num.Sub(num, y)
		den := newFloat(c).Set(x)
		den = den.Mul(den, y)
		den = den.Add(den, floatOne)
		z = z.Add(z, floatAtan(c, num.Quo(num, den)))
		return z
	}

	if x.Cmp(floatOne) > 0 {
		return floatAtanLarge(c, x)
	}

	// This is the series for small values |x| <  1.
	// asin(x) = x - x³/3 + x⁵/5 - x⁷/7 + ...
	// First term to compute in loop will be x

	n := newFloat(c)
	term := newFloat(c)
	xN := newFloat(c).Set(x)
	xSquared := newFloat(c).Set(x)
	xSquared.Mul(x, x)
	z := newFloat(c)

	// n goes up by two each loop.
	for loop := newLoop(c.Config(), "atan", x, 4); ; {
		term.Set(xN)
		term.Quo(term, n.SetUint64(2*loop.i+1))
		z.Add(z, term)
		xN.Neg(xN)

		if loop.done(z) {
			break
		}
		// xN *= x², becoming x**(n+2).
		xN.Mul(xN, xSquared)
	}

	return z
}

// floatAtanLarge computes atan(x)  for large x using a Taylor series.
// x is known to be > 1.
func floatAtanLarge(c Context, x *big.Float) *big.Float {
	// This is the series for larger values |x| >=  1.
	// For x > 0, atan(x) = +π/2 - 1/x + 1/3x³ -1/5x⁵ + 1/7x⁷ - ...
	// First term to compute in loop will be -1/x

	n := newFloat(c)
	term := newFloat(c)
	xN := newFloat(c).Set(x)
	xSquared := newFloat(c).Set(x)
	xSquared.Mul(x, x)
	z := newFloat(c).Set(floatPi)
	z.Quo(z, floatTwo)

	// n goes up by two each loop.
	for loop := newLoop(c.Config(), "atan", x, 4); ; {
		xN.Neg(xN)
		term.Set(xN)
		term.Mul(term, n.SetUint64(2*loop.i+1))
		term.Quo(floatOne, term)
		z.Add(z, term)

		if loop.done(z) {
			break
		}
		// xN *= x², becoming x**(n+2).
		xN.Mul(xN, xSquared)
	}

	return z
}
