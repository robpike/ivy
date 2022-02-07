// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"math/big"
)

func sqrt(c Context, v Value) Value {
	if u, ok := v.(Complex); ok {
		if !isZero(u.imag) {
			return complexSqrt(c, u)
		}
		v = u.real
	}
	if isNegative(v) {
		return newComplex(Int(0), evalFloatFunc(c, c.EvalUnary("-", v), floatSqrt))
	}
	return evalFloatFunc(c, v, floatSqrt)
}

// complexSqrt returns sqrt(v) where v is Complex.
func complexSqrt(c Context, v Complex) Complex {
	// First turn v into (a + bi) where a and b are big.Floats.
	a := floatSelf(c, v.real).Float
	b := floatSelf(c, v.imag).Float
	a2 := newFloat(c).Mul(a, a)
	b2 := newFloat(c).Mul(b, b)
	mag := floatSqrt(c, a2.Add(a2, b2))
	// The real part is sqrt(mag+a)/2.
	r := newFloat(c).Add(mag, a)
	r = floatSqrt(c, r.Quo(r, floatTwo))
	// The imaginary part is sgn(b)*sqrt(mag-a)/2
	i := newFloat(c).Sub(mag, a)
	i.Quo(i, floatTwo)
	i = floatSqrt(c, i)
	if b.Sign() < 0 {
		i.Neg(i)
	}
	// As with normal square roots, we only return the positive root.
	return newComplex(BigFloat{r}.shrink(), BigFloat{i}.shrink())
}

func evalFloatFunc(c Context, v Value, fn func(Context, *big.Float) *big.Float) Value {
	return BigFloat{(fn(c, floatSelf(c, v).Float))}.shrink()
}

// floatSqrt computes the square root of x using Newton's method.
// TODO: Use a better algorithm such as the one from math/sqrt.go.
func floatSqrt(c Context, x *big.Float) *big.Float {
	switch x.Sign() {
	case -1:
		Errorf("square root of negative number") // Should never happen but be safe.
	case 0:
		return newFloat(c)
	}

	// Each iteration computes
	// 	z = z - (z²-x)/2z
	// z holds the result so far. A good starting point is to halve the exponent.
	// Experiments show we converge in only a handful of iterations.
	z := newFloat(c)
	exp := x.MantExp(z)
	z.SetMantExp(z, exp/2)

	// Intermediates, allocated once.
	zSquared := newFloat(c)
	num := newFloat(c)
	den := newFloat(c)

	for loop := newLoop(c.Config(), "sqrt", x, 1); ; {
		zSquared.Mul(z, z)
		num.Sub(zSquared, x)
		den.Mul(floatTwo, z)
		num.Quo(num, den)
		z.Sub(z, num)
		if loop.done(z) {
			break
		}
	}
	return z
}
