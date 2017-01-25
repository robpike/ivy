// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

func logn(c Context, v Value) Value {
	return evalFloatFunc(c, v, floatLog)
}

func logBaseU(c Context, u, v Value) Value {
	return c.EvalBinary(logn(c, v), "/", logn(c, u))
}

// floatLog computes natural log(x) using the Maclaurin series for log(1-x).
func floatLog(c Context, x *big.Float) *big.Float {
	if x.Sign() <= 0 {
		Errorf("log of non-positive value")
	}
	// The series wants x < 1, and log 1/x == -log x, so exploit that.
	invert := false
	x = newFloat(c).Set(x) // Don't modify argument!
	if x.Cmp(floatOne) > 0 {
		invert = true
		x.Quo(floatOne, x)
	}

	// x = mantissa * 2**exp, and 0.5 <= mantissa < 1.
	// So log(x) is log(mantissa)+exp*log(2), and 1-x will be
	// between 0 and 0.5, so the series for 1-x will converge well.
	// (The series converges slowly in general.)
	mantissa := newFloat(c)
	exp2 := x.MantExp(mantissa)
	exp := newFloat(c).SetInt64(int64(exp2))
	exp.Mul(exp, floatLog2)
	if invert {
		exp.Neg(exp)
	}

	// y = 1-x (whereupon x = 1-y and we use that in the series).
	y := newFloat(c).SetInt64(1)
	y.Sub(y, mantissa)

	// The Maclaurin series for log(1-y) == log(x) is: -y - y²/2 - y³/3 ...

	yN := newFloat(c).Set(y)
	term := newFloat(c)
	n := newFloat(c).Set(floatOne)
	z := newFloat(c)

	// This is the slowest-converging series, so we add a factor of ten to the cutoff.
	// Only necessary when FloatPrec is at or beyond constPrecisionInBits.

	for loop := newLoop(c.Config(), "log", x, 40); ; {
		term.Quo(yN, n.SetUint64(loop.i+1))
		z.Sub(z, term)
		if loop.done(z) {
			break
		}
		// Advance y**index (multiply by y).
		yN.Mul(yN, y)
	}

	if invert {
		z.Neg(z)
	}
	z.Add(z, exp)

	return z
}
