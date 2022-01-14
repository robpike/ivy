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
	arg           *big.Float // original argument to function; only used for diagnostic.
	prevZ         *big.Float // Result from the previous iteration.
	delta         *big.Float // |Change| from previous iteration.
}

// newLoop returns a new loop checker. The arguments are the name
// of the function being evaluated, the argument to the function, and
// the maximum number of iterations to perform before giving up.
// The last number in terms of iterations per bit, so the caller can
// ignore the precision setting.
func newLoop(conf *config.Config, name string, x *big.Float, itersPerBit uint) *loop {
	return &loop{
		name:          name,
		arg:           newF(conf).Set(x),
		maxIterations: 10 + uint64(itersPerBit*conf.FloatPrec()),
		prevZ:         newF(conf),
		delta:         newF(conf),
	}
}

// done reports whether the loop is done. If it does not converge
// after the maximum number of iterations, it errors out.
// It will not return before doing at least 3 iterations. Some
// series (such as exp(-1) hit zero along the way.
func (l *loop) done(z *big.Float) bool {
	const minIterations = 3
	l.delta.Sub(l.prevZ, z)
	sign := l.delta.Sign()
	if sign == 0 && l.i >= minIterations {
		return true
	}
	if sign < 0 {
		l.delta.Neg(l.delta)
	}
	// Check if delta is no bigger than the smallest change in z that can be
	// represented with the given precision.
	var eps big.Float
	eps.SetMantExp(eps.SetUint64(1), z.MantExp(nil)-int(z.Prec()))
	if l.delta.Cmp(&eps) <= 0 && l.i >= minIterations {
		return true
	}
	l.i++
	if l.i == l.maxIterations {
		// Users should never see this.
		Errorf("%s %s: did not converge after %d iterations; prev,last result %s,%s delta %s", l.name, BigFloat{l.arg}, l.maxIterations, BigFloat{z}, BigFloat{l.prevZ}, BigFloat{l.delta})
	}
	l.prevZ.Set(z)
	return false
}
