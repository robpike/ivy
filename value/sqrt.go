// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

func sqrt(c Context, v Value) Value {
	return evalFloatFunc(c, v, floatSqrt)
}

func evalFloatFunc(c Context, v Value, fn func(Context, *big.Float) *big.Float) Value {
	return BigFloat{(fn(c, floatSelf(c, v).(BigFloat).Float))}.shrink()
}

// floatSqrt computes the square root of x using Newton's method.
// TODO: Use a better algorithm such as the one from math/sqrt.go.
func floatSqrt(c Context, x *big.Float) *big.Float {
	switch x.Sign() {
	case -1:
		Errorf("square root of negative number")
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
