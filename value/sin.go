// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

func sin(c Context, v Value) Value {
	return evalFloatFunc(c, v, floatSin)
}

func cos(c Context, v Value) Value {
	return evalFloatFunc(c, v, floatCos)
}

func tan(c Context, v Value) Value {
	x := floatSelf(c, v).(BigFloat).Float
	if x.IsInf() {
		Errorf("tangent of infinity")
	}
	negate := false
	if x.Sign() < 0 {
		x.Neg(x)
		negate = true
	}
	twoPiReduce(c, x)
	num := floatSin(c, x)
	den := floatCos(c, x)
	if den.Sign() == 0 {
		Errorf("tangent is infinite")
	}
	num.Quo(num, den)
	if negate {
		num.Neg(num)
	}
	return BigFloat{num}.shrink()
}

// floatSin computes sin(x) using argument reduction and a Taylor series.
func floatSin(c Context, x *big.Float) *big.Float {
	if x.IsInf() {
		Errorf("sine of infinity")
	}
	negate := false
	if x.Sign() < 0 {
		x.Neg(x)
		negate = true
	}
	twoPiReduce(c, x)

	// sin(x) = x - x³/3! + x⁵/5! - ...
	// First term to compute in loop will be -x³/3!
	factorial := newFloat(c).SetInt64(6)

	result := sincos("sin", c, 3, x, newFloat(c).Set(x), 3, factorial)

	if negate {
		result.Neg(result)
	}

	return result
}

// floatCos computes cos(x) using argument reduction and a Taylor series.
func floatCos(c Context, x *big.Float) *big.Float {
	if x.IsInf() {
		Errorf("cosine of infinity")
	}
	twoPiReduce(c, x)

	// cos(x) = 1 - x²/2! + x⁴/4! - ...
	// First term to compute in loop will be -x²/2!.
	factorial := newFloat(c).Set(floatTwo)

	return sincos("cos", c, 2, x, newFloat(c).SetInt64(1), 2, factorial)
}

// sincos iterates a sin or cos Taylor series.
func sincos(name string, c Context, index int, x *big.Float, z *big.Float, exp uint64, factorial *big.Float) *big.Float {
	term := newFloat(c).Set(floatOne)
	for j := 0; j < index; j++ {
		term.Mul(term, x)
	}
	xN := newFloat(c).Set(term)
	x2 := newFloat(c).Mul(x, x)
	n := newFloat(c)

	for loop := newLoop(c.Config(), name, x, 4); ; {
		// Invariant: factorial holds -1ⁿ*exponent!.
		factorial.Neg(factorial)
		term.Quo(term, factorial)
		z.Add(z, term)

		if loop.done(z) {
			break
		}
		// Advance x**index (multiply by x²).
		term.Mul(xN, x2)
		xN.Set(term)
		// Advance factorial.
		factorial.Mul(factorial, n.SetUint64(exp+1))
		factorial.Mul(factorial, n.SetUint64(exp+2))
		exp += 2
	}
	return z
}

// twoPiReduce guarantees x < 2π; x is known to be >= 0 coming in.
func twoPiReduce(c Context, x *big.Float) {
	// TODO: Is there an easy better algorithm?
	twoPi := newFloat(c).Set(floatTwo)
	twoPi.Mul(twoPi, floatPi)
	// Do something clever(er) if it's large.
	if x.Cmp(newFloat(c).SetInt64(1000)) > 0 {
		multiples := make([]*big.Float, 0, 100)
		sixteen := newFloat(c).SetInt64(16)
		multiple := newFloat(c).Set(twoPi)
		for {
			multiple.Mul(multiple, sixteen)
			if x.Cmp(multiple) < 0 {
				break
			}
			multiples = append(multiples, newFloat(c).Set(multiple))
		}
		// From the right, subtract big multiples.
		for i := len(multiples) - 1; i >= 0; i-- {
			multiple := multiples[i]
			for x.Cmp(multiple) >= 0 {
				x.Sub(x, multiple)
			}
		}
	}
	for x.Cmp(twoPi) >= 0 {
		x.Sub(x, twoPi)
	}
}
