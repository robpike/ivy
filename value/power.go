// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"math/big"

	"robpike.io/ivy/config"
)

func power(c Context, u, v Value) Value {
	z := floatPower(c, floatSelf(c, u).(BigFloat), floatSelf(c, v).(BigFloat))
	return BigFloat{z}.shrink()
}

func exp(c Context, u Value) Value {
	z := exponential(c.Config(), floatSelf(c, u).(BigFloat).Float)
	return BigFloat{z}.shrink()
}

// floatPower computes bx to the power of bexp.
func floatPower(c Context, bx, bexp BigFloat) *big.Float {
	x := bx.Float
	fexp := newFloat(c).Set(bexp.Float)
	positive := true
	conf := c.Config()
	switch fexp.Sign() {
	case 0:
		return newFloat(c).SetInt64(1)
	case -1:
		if x.Sign() == 0 {
			Errorf("negative exponent of zero")
		}
		positive = false
		fexp = c.EvalUnary("-", bexp).toType(conf, bigFloatType).(BigFloat).Float
	}
	if x.Cmp(floatOne) == 0 || x.Sign() == 0 {
		return x
	}
	isInt := true
	exp, acc := fexp.Int64() // No point in doing *big.Ints now. TODO?
	if acc != big.Exact {
		isInt = false
	}
	// Integer part.
	z := integerPower(c, x, exp)
	// Fractional part..
	if !isInt {
		frac := fexp.Sub(fexp, newFloat(c).SetInt64(exp))
		// x**frac is e**(frac*log x)
		logx := floatLog(c, x)
		frac.Mul(frac, logx)
		z.Mul(z, exponential(c.Config(), frac))
	}
	if !positive {
		z.Quo(floatOne, z)
	}
	return z
}

// exponential computes exp(x) using the Taylor series. It converges quickly
// since we call it with only small values of x.
func exponential(conf *config.Config, x *big.Float) *big.Float {
	// The Taylor series for e**x, exp(x), is 1 + x + x²/2! + x³/3! ...

	xN := newF(conf).Set(x)
	term := newF(conf)
	n := newF(conf)
	nFactorial := newF(conf).SetUint64(1)
	z := newF(conf).SetInt64(1)

	for loop := newLoop(conf, "exponential", x, 4); ; {
		term.Set(xN)
		term.Quo(term, nFactorial)
		z.Add(z, term)

		if loop.done(z) {
			break
		}
		// Advance x**index (multiply by x).
		xN.Mul(xN, x)
		// Advance n, n!.
		nFactorial.Mul(nFactorial, n.SetUint64(loop.i+1))
	}

	return z

}

// integerPower returns x**exp where exp is an int64 of size <= intBits.
func integerPower(c Context, x *big.Float, exp int64) *big.Float {
	z := newFloat(c).SetInt64(1)
	y := newFloat(c).Set(x)
	// For each loop, we compute a xⁿ where n is a power of two.
	for exp > 0 {
		if exp&1 == 1 {
			// This bit contributes. Multiply it into the result.
			z.Mul(z, y)
		}
		y.Mul(y, y)
		exp >>= 1
	}
	return z
}
