// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"math/big"

	"robpike.io/ivy/config"
)

type loop struct {
	name          string     // The name of the function we are evaluating.
	i             uint64     // Loop count.
	maxIterations uint64     // When to give up.
	stallCount    int        // Iterations since |delta| changed.
	start         *big.Float // starting value.
	prevZ         *big.Float // Result from the previous iteration.
	delta         *big.Float // |Change| from previous iteration.
	prevDelta     *big.Float // Delta from the previous iteration.
}

// newLoop returns a new loop checker. The arguments are the name
// of the function being evaluated, the argument to the function, and
// the maximum number of iterations to perform before giving up.
// The last number in terms of iterations per bit, so the caller can
// ignore the precision setting.
func newLoop(conf *config.Config, name string, x *big.Float, itersPerBit uint) *loop {
	return &loop{
		name:          name,
		start:         newF(conf).Set(x),
		maxIterations: 10 + uint64(itersPerBit*conf.FloatPrec()),
		prevZ:         newF(conf),
		delta:         newF(conf).Set(x),
		prevDelta:     newF(conf),
	}
}

// done reports whether the loop is done. If it does not converge
// after the maximum number of iterations, it errors out.
func (l *loop) done(z *big.Float) bool {
	l.delta.Sub(l.prevZ, z)
	if l.delta.Sign() == 0 {
		return true
	}
	if l.delta.Sign() < 0 {
		// Convergence can oscillate when the calculation is nearly
		// done and we're running out of bits. This stops that.
		// See next comment.
		l.delta.Neg(l.delta)
	}
	if l.delta.Cmp(l.prevDelta) == 0 {
		// In freaky cases (like e**3) we can hit the same large positive
		// and then  large negative value (4.5, -4.5) so we count a few times
		// to see that it really has stalled. Avoids having to do hard math,
		// but it means we may iterate a few extra times. Usually, though,
		// iteration is stopped by the zero check above, so this is fine.
		l.stallCount++
		if l.stallCount > 3 {
			// Convergence has stopped.
			return true
		}
	} else {
		l.stallCount = 0
	}
	l.i++
	if l.i == l.maxIterations {
		// Users should never see this.
		Errorf("%s %s: did not converge after %d iterations; prev,last result %s,%s delta %s", l.name, l.start, l.maxIterations, BigFloat{z}, BigFloat{l.prevZ}, BigFloat{l.delta})
	}
	l.prevDelta.Set(l.delta)
	l.prevZ.Set(z)
	return false

}
