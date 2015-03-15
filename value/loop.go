// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

type loop struct {
	name          string
	i             int
	maxIterations int
	start         *big.Float // starting value.
	prevZ         *big.Float // Result from the previous iteration.
	delta         *big.Float // |Change| from previous iteration.
	prevDelta     *big.Float // Delta from the previous iteration.
}

func newLoop(name string, x *big.Float, maxIterations int) *loop {
	return &loop{
		name:          name,
		start:         newF().Set(x),
		maxIterations: maxIterations,
		prevZ:         newF(),
		delta:         newF().Set(x),
		prevDelta:     newF(),
	}
}

func (l *loop) terminate(z *big.Float) bool {
	l.delta.Sub(l.prevZ, z)
	if l.delta.IsZero() {
		return true
	}
	if l.delta.IsNeg() {
		// Convergence can oscillate when the calculation is nearly
		// done and we're running out of bits. This stops that.
		// Happens for argument 1e1000 at almost any precision.
		// TODO: This is a bad idea; delta can still be large. Test case: exponential(3).
		// TODO: Must be fixed!
		l.delta.Neg(l.delta)
	}
	if l.delta.Cmp(l.prevDelta).Eql() {
		// Convergence has stopped.
		return true
	}
	l.i++
	if l.i == l.maxIterations {
		Errorf("%s %s: did not converge after %d iterations; prev,last result %s,%s delta %s", l.name, l.start, l.maxIterations, BigFloat{z}, BigFloat{l.prevZ}, BigFloat{l.delta})
	}
	l.prevDelta.Set(l.delta)
	l.prevZ.Set(z)
	return false

}
