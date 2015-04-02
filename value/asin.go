// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

func asin(v Value) Value {
	return BigFloat{floatAsin("asin", floatSelf(v).(BigFloat).Float)}.shrink()
}

func acos(v Value) Value {
	return BigFloat{floatAcos(floatSelf(v).(BigFloat).Float)}.shrink()
}

func atan(v Value) Value {
	return BigFloat{floatAtan(floatSelf(v).(BigFloat).Float)}.shrink()
}

// floatAsin computes asin(x) using a Taylor series.
func floatAsin(name string, x *big.Float) *big.Float {
	// Must be in range.
	one := newF().SetInt64(1)
	minusOne := newF().SetInt64(-1)
	if x.Cmp(minusOne) < 0 || 0 < x.Cmp(one) {
		Errorf("asin argument out of range")
	}
	// The series converges very slowly near 1.
	// At least we can pick off 1 and -1 since they're values that are
	// likely to be tried by an interactive user.
	if x.Cmp(one) == 0 {
		z := newF().Set(floatPi)
		return z.Quo(z, newF().SetInt64(2))
	}
	if x.Cmp(minusOne) == 0 {
		z := newF().Set(floatPi)
		z.Quo(z, newF().SetInt64(2))
		return z.Neg(z)
	}

	// asin(x) = x + (1/2) x³/3 + (1*3/2*4) x⁵/5 + (1*3*5/2*4*6) x⁷/7 + ...
	// First term to compute in loop will be x

	n := newF().Set(one)
	term := newF()
	coef := newF().Set(one)
	xN := newF().Set(x)
	xSquared := newF().Set(x)
	xSquared.Mul(x, x)
	z := newF()

	loop := newLoop(name, x, 10000)
	// n goes up by two each loop.
	for {
		term.Set(coef)
		term.Mul(term, xN)
		term.Quo(term, n)
		z.Add(z, term)

		if loop.terminate(z) {
			break
		}
		// Advance.
		// coef *= n/(n+1)
		coef.Mul(coef, n)
		n.Add(n, one)
		coef.Quo(coef, n)
		n.Add(n, one)
		// xN *= x², becoming x**n.
		xN.Mul(xN, xSquared)
	}

	return z
}

// floatAcos computes acos(x) using a Taylor series.
func floatAcos(x *big.Float) *big.Float {
	// acos(x) = π/2 - asin(x)
	z := newF().Set(floatPi)
	z.Quo(z, newF().SetInt64(2))
	return z.Sub(z, floatAsin("acos", x))
}

// floatAtan computes atan(x) using a Taylor series. There are two series,
// one for |x| < 1 and one for larger values.
func floatAtan(x *big.Float) *big.Float {
	one := newF().SetInt64(1)
	minusOne := newF().SetInt64(-1)
	if x.Cmp(minusOne) < 0 || 0 < x.Cmp(one) {
		return floatAtanLarge(x)
	}
	// The series converge very slowly near 1. atan 1.00001 takes over a million
	// iterations. At least we can pick off 1 and -1 since they're values that are
	// likely to be tried by an interactive user.
	if x.Cmp(one) == 0 {
		z := newF().Set(floatPi)
		return z.Quo(z, newF().SetInt64(4))
	}
	if x.Cmp(minusOne) == 0 {
		z := newF().Set(floatPi)
		z.Quo(z, newF().SetInt64(4))
		return z.Neg(z)
	}

	// This is the series for small values |x| <  1.
	// asin(x) = x - x³/3 + x⁵/5 - x⁷/7 + ...
	// First term to compute in loop will be x

	n := newF().Set(one)
	two := newF().SetInt64(2)
	term := newF()
	xN := newF().Set(x)
	xSquared := newF().Set(x)
	xSquared.Mul(x, x)
	z := newF()
	plus := true

	loop := newLoop("atan", x, 10000)
	// n goes up by two each loop.
	for {
		term.Set(xN)
		term.Quo(term, n)
		if plus {
			z.Add(z, term)
		} else {
			z.Sub(z, term)
		}
		plus = !plus

		if loop.terminate(z) {
			break
		}
		// Advance.
		n.Add(n, two)
		// xN *= x², becoming x**(n+2).
		xN.Mul(xN, xSquared)
	}

	return z
}

// floatAtan computes atan(x)  for large x using a Taylor series.
func floatAtanLarge(x *big.Float) *big.Float {
	// This is the series for larger values |x| >=  1.
	// For x > 0, atan(x) = +π/2 - 1/x + 1/3x³ -1/5x⁵ + 1/7x⁷ - ...
	// For x < 0, atan(x) = -π/2 - 1/x + 1/3x³ -1/5x⁵ + 1/7x⁷ - ...
	// First term to compute in loop will be -1/x

	one := newF().SetInt64(1)
	n := newF().Set(one)
	two := newF().SetInt64(2)
	term := newF()
	xN := newF().Set(x)
	xSquared := newF().Set(x)
	xSquared.Mul(x, x)
	z := newF().Set(floatPi)
	z.Quo(z, newF().SetInt64(2))
	if x.Sign() < 0 {
		z.Neg(z)
	}
	plus := false

	loop := newLoop("atan", x, 1e6)
	// n goes up by two each loop.
	for {
		term.Set(xN)
		term.Mul(term, n)
		term.Quo(one, term)
		if plus {
			z.Add(z, term)
		} else {
			z.Sub(z, term)
		}
		plus = !plus

		if loop.terminate(z) {
			break
		}
		// Advance.
		n.Add(n, two)
		// xN *= x², becoming x**(n+2).
		xN.Mul(xN, xSquared)
	}

	return z
}
