// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

func sin(v Value) Value {
	return BigFloat{floatSin(floatSelf(v).(BigFloat).Float)}.shrink()
}

func cos(v Value) Value {
	return BigFloat{floatCos(floatSelf(v).(BigFloat).Float)}.shrink()
}

func tan(v Value) Value {
	x := floatSelf(v).(BigFloat).Float
	twoPiReduce(x)
	num := floatSin(x)
	den := floatCos(x)
	if den.Sign() == 0 {
		Errorf("tangent is infinite")
	}
	num.Quo(num, den)
	return BigFloat{num}.shrink()
}

// floatSin computes sin(x) using argument reduction and a Taylor series.
// TODO: Use a better algorithm?
func floatSin(x *big.Float) *big.Float {
	negate := false
	if x.Sign() < 0 {
		x.Neg(x)
		negate = true
	}
	twoPiReduce(x)

	// sin(x) = x - x³/3! + x⁵/5! - ...
	// First term to compute in loop will be -x³/3!
	exponent := newF().SetInt64(3)
	factorial := newF().SetInt64(6)

	result := sincos("sin", 3, x, newF().Set(x), exponent, factorial)

	if negate {
		result.Neg(result)
	}

	return result
}

// floatCos computes sin(x) using argument reduction and a Taylor series.
// TODO: Use a better algorithm?
func floatCos(x *big.Float) *big.Float {
	twoPiReduce(x)

	// cos(x) = 1 - x²/2! + x⁴/4! - ...
	// First term to compute in loop will be -x²/2!.
	exponent := newF().SetInt64(2)
	factorial := newF().SetInt64(2)

	return sincos("cos", 2, x, newF().SetInt64(1), exponent, factorial)
}

// sincos iterates a sin or cos Taylor series.
func sincos(name string, index int, x, z, exponent, factorial *big.Float) *big.Float {
	one := newF().SetInt64(1)
	plus := false
	term := newF()
	term.Set(one)
	for j := 0; j < index; j++ {
		term.Mul(term, x)
	}
	xN := newF().Set(term)
	x2 := newF().Mul(x, x)

	loop := newLoop(name, x, 1000)
	for {
		// Invariant: factorial holds exponent!.
		term.Quo(term, factorial)
		if plus {
			z.Add(z, term)
		} else {
			z.Sub(z, term)
		}
		plus = !plus

		if loop.terminate(z) {
			break
		}
		// Advance x**index (multiply by x²).
		term.Mul(xN, x2)
		xN.Set(term)
		// Advance exponent and factorial.
		exponent.Add(exponent, one)
		factorial.Mul(factorial, exponent)
		exponent.Add(exponent, one)
		factorial.Mul(factorial, exponent)
	}
	return z
}

// twoPiReduce guarantees x < 2𝛑; x is known to be >= coming in.
func twoPiReduce(x *big.Float) {
	// Stupid algorithm. TODO.
	twoPi := newF().SetInt64(2)
	twoPi.Mul(twoPi, floatPi)
	for x.Cmp(twoPi).Geq() {
		x.Sub(x, twoPi)
	}
}
