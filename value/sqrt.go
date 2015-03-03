// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

func sqrt(v Value) Value {
	return floatSqrt(floatSelf(v).(BigFloat))
}

// floatSqrt computes the square root of x using Newton's method.
// TODO: Use a better algorithm such as the one from math/sqrt.go.
func floatSqrt(bx BigFloat) Value {
	x := bx.Float
	two := newF().SetInt64(2)
	if x.Sign() < 0 {
		Errorf("square root of negative number")
	}

	// Each iteration computes
	// 	z = z - (z²-x)/2z
	// delta holds the difference between the result
	// this iteration and the previous. The loop stops
	// when it hits zero.

	// z holds the result so far. A good starting point is to halve the exponent.
	// Experiments show we converge in only a handful of iterations.
	z := newF()
	fr, exp := x.MantExp(&floatTmp)
	z.SetMantExp(fr, exp/2)

	// These are used to terminate iteration.
	prevZ := newF()        // Result from the previous iteration.
	delta := newF().Set(x) // |Change| from previous iteration.
	prevDelta := newF()    // Delta from the previous iteration.

	// Intermediates, allocated once.
	zSquared := newF()
	num := newF()
	den := newF()

	var i = 0
	const maxIterations = 100
	for i = 0; ; i++ {
		zSquared = zSquared.Mul(z, z)
		num.Sub(zSquared, x)
		den.Mul(two, z)
		num.Quo(num, den)
		z.Sub(z, num)
		if terminate(z, prevZ, delta, prevDelta) {
			break
		}
		if i == maxIterations {
			Errorf("sqrt %s did not converge after %d iterations; prev,last result %s,%s delta %s", bx, maxIterations, BigFloat{z}, BigFloat{prevZ}, BigFloat{delta})
		}
	}
	return BigFloat{z}.shrink()
}

func terminate(z, prevZ, delta, prevDelta *big.Float) bool {
	delta.Sub(prevZ, z)
	if delta.Sign() == 0 {
		return true
	}
	if delta.Sign() < 0 {
		// Convergence can oscillate when the calculation is nearly
		// done and we're running out of bits. This stops that.
		// Happens for argument 1e1000 at almost any precision.
		delta.Neg(delta)
	}
	if delta.Cmp(prevDelta) == 0 {
		// Convergence has stopped.
		return true
	}
	prevDelta.Set(delta)
	prevZ.Set(z)
	return false
}
