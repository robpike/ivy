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
		v = newComplex(v, Int(0))
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
	var i Int
	switch u := u.(type) {
	case Int:
		i, v = intLog(u, v.(Int))
		if v.(Int) == 1 {
			return i
		}
	case BigInt:
		i, v = bigIntLog(u, v.(BigInt))
		if v.(BigInt).Cmp(bigIntOne.Int) == 0 {
			return i
		}
	}
	f := c.EvalBinary(logn(c, v), "/", logn(c, u))
	if i != 0 {
		f = c.EvalBinary(f, "+", i)
	}
	return f
}

// intLog returns the integer portion i of log base b of v
// along with the remaining portion v / b^i.
func intLog(b, v Int) (i, r Int) {
	if b <= 1 || v < 0 {
		return 0, v
	}
	iu, ru := uint64Log(uint64(b), uint64(v))
	return Int(iu), Int(ru)
}

// uint64Log is like intLog, but for uint64.
// Working in uint64 makes the overflow check easy.
func uint64Log(b, v uint64) (i, r uint64) {
	// Log by repeated squaring produces a single bit at a time, high to low.
	// The algorithm is the reverse of exponentiation by repeated squaring,
	if b <= 1 || v < b || v%b != 0 {
		return 0, v
	}
	// u doesn't fit in 32 bits, then b*b will both overflow the uint64
	// and be larger than v (if not for the overflow),
	// so only recurse when b does fit in 32 bits
	if uint64(uint32(b)) == b {
		i, v = uint64Log(b*b, v)
		i <<= 1
	}
	if v >= b {
		v /= b
		i |= 1
	}
	return i, v
}

// bigIntLog is like intLog, but for BigInt.
// Note that the integer log portion always fits in an Int:
// any non-trivial result is using base b ≥ 2,
// bounding the result by the number of bits in v.
func bigIntLog(b, v BigInt) (i Int, r BigInt) {
	if b.Cmp(bigIntOne.Int) <= 0 || v.Cmp(b.Int) < 0 {
		return 0, v
	}
	if z := new(big.Int).Mod(v.Int, b.Int); z.Cmp(bigIntZero.Int) != 0 {
		return 0, v
	}

	// Only compute b*b if it can be smaller than v.
	if 2*b.BitLen()-1 <= v.BitLen() {
		i, v = bigIntLog(BigInt{new(big.Int).Mul(b.Int, b.Int)}, v)
		i <<= 1
	}
	if v.Cmp(b.Int) >= 0 {
		v = BigInt{new(big.Int).Div(v.Int, b.Int)}
		i |= 1
	}
	return i, v
}

// floatLog computes natural log(x) using the Maclaurin series for log(1-x).
func floatLog(c Context, x *big.Float) *big.Float {
	if x.Sign() <= 0 {
		Errorf("log of non-positive value")
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

// Note: We return a Complex here, not a Value, so the caller
// might want to call shrink. This is so the binary ** has a Complex
// on both sides.
func complexLog(c Context, v Complex) Complex {
	abs := v.abs(c)
	phase := v.phase(c)
	return newComplex(logn(c, abs), phase)
}
