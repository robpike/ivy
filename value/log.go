// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"math/big"
)

func logn(c Context, v Value) Value {
	negative := isNegative(v)
	if negative {
		// Promote to complex. The Complex type is never negative.
		v = NewComplex(c, v, zero)
	}
	if u, ok := v.(Complex); ok {
		if isNegative(u.real) {
			negative = true
		}
		if !isZero(u.imag) || negative {
			return complexLog(c, u).shrink()
		}
		v = u.real
	}
	return evalFloatFunc(c, v, floatLog)
}

func logBaseU(c Context, u, v Value) Value {
	// Handle the integer part exactly when the arguments are exact.
	switch u := u.(type) {
	case Int:
		log, exact := intLog(u, v.(Int))
		if exact {
			return Int(log)
		}
	case BigInt:
		log, exact := bigIntLog(u, v.(BigInt))
		if exact {
			return log
		}
	}
	return c.EvalBinary(logn(c, v), "/", logn(c, u))
}

// intLog returns the integer portion of the log base b of v, if it's exact.
// If inexact, it returns zero.
func intLog(bI, vI Int) (log uint64, exact bool) {
	b := uint64(bI)
	v := uint64(vI)
	if b <= 1 || v < b {
		return 0, false
	}
	log = 1 // The logarithm is at least 1 because v>=b.
	// Looping one division at a time is easy but slow.
	// Doesn't matter much here but can be important in bigIntLog.
	for v != b {
		quo := v / b
		rem := v % b
		if rem != 0 {
			return 0, false
		}
		log++
		v = quo
	}
	return log, true
}

// bigIntLog returns the integer portion of the log base b of v, if it's exact.
// If inexact, it returns zero.
func bigIntLog(b, v BigInt) (log Int, exact bool) {
	if b.Cmp(bigIntOne.Int) <= 0 || v.Cmp(b.Int) < 0 {
		return 0, false
	}
	log = Int(1) // The logarithm is at least 1 because v>=b.
	x := new(big.Int).Set(v.Int)
	quo := new(big.Int)
	rem := new(big.Int)
	// Looping one division at a time is easy but slow.
	// TODO: Scale up faster.
	for x.Cmp(b.Int) != 0 {
		quo.DivMod(x, b.Int, rem)
		if rem.Sign() != 0 {
			return 0, false
		}
		log++
		x.Set(quo)
	}
	return log, true
}

// floatLog computes natural log(x) using the Maclaurin series for log(1-x).
func floatLog(c Context, x *big.Float) *big.Float {
	if x.Sign() <= 0 {
		c.Errorf("log of non-positive value")
	}
	// Convergence is imperfect at 1, so get it right.
	if x.Cmp(floatOne) == 0 {
		return newFloat(c)
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
	mantissa := new(big.Float)
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

	for loop := newLoop(c, "log", x, 40); ; {
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

// Note: We return a Complex here, not a Value, so the caller
// might want to call shrink. This is so the binary ** has a Complex
// on both sides.
func complexLog(c Context, v Complex) Complex {
	abs := v.abs(c)
	phase := v.phase(c)
	return NewComplex(c, logn(c, abs), phase)
}
