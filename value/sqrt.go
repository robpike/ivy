// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

func sqrt(v Value) Value {
	return floatSqrt(floatSelf(v).(BigFloat))
}

// floatSqrt computes the square root of x using Newton's method.
// TODO: Use a better algorithm such as the one from math/sqrt.go.
func floatSqrt(bx BigFloat) Value {
	x := bx.Float
	two := newF().SetInt64(2)
	if x.IsNeg() {
		Errorf("square root of negative number")
	}
	if x.IsZero() {
		return zero
	}

	// Each iteration computes
	// 	z = z - (z²-x)/2z
	// delta holds the difference between the result
	// this iteration and the previous. The loop stops
	// when it hits zero.

	// z holds the result so far. A good starting point is to halve the exponent.
	// Experiments show we converge in only a handful of iterations.
	z := newF()
	exp := x.MantExp(z)
	z.SetMantExp(z, exp/2)

	// Intermediates, allocated once.
	zSquared := newF()
	num := newF()
	den := newF()

	loop := newLoop("sqrt", x, 1)
	for {
		zSquared = zSquared.Mul(z, z)
		num.Sub(zSquared, x)
		den.Mul(two, z)
		num.Quo(num, den)
		z.Sub(z, num)
		if loop.terminate(z) {
			break
		}
	}
	return BigFloat{z}.shrink()
}
