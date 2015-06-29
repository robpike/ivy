// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

func power(u, v Value) Value {
	z := floatPower(floatSelf(u).(BigFloat), floatSelf(v).(BigFloat))
	return BigFloat{z}.shrink()
}

// floatPower computes bx to the power of bexp.
func floatPower(bx, bexp BigFloat) *big.Float {
	x := bx.Float
	fexp := bexp.Float
	positive := true
	switch fexp.Sign() {
	case 0:
		return newF().SetInt64(1)
	case -1:
		if x.Sign() == 0 {
			Errorf("negative exponent of zero")
		}
		positive = false
		fexp = Unary("-", bexp).toType(bigFloatType).(BigFloat).Float
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
	z := integerPower(x, exp)
	// Fractional part..
	if !isInt {
		frac := fexp.Sub(fexp, newF().SetInt64(exp))
		// x**frac is e**(frac*log x)
		logx := floatLog(x)
		frac.Mul(frac, logx)
		z.Mul(z, exponential(frac))
	}
	if !positive {
		z.Quo(floatOne, z)
	}
	return z
}

// exponential computes exp(x) using the Taylor series. It converges quickly
// since we call it with only small values of x.
func exponential(x *big.Float) *big.Float {
	// The Taylor series for e**x, exp(x), is 1 + x + x²/2! + x³/3! ...

	xN := newF().Set(x)
	term := newF()
	n := big.NewInt(1)
	nFactorial := big.NewInt(1)
	z := newF().SetInt64(1)

	loop := newLoop("exponential", x, 4)
	for i := 0; ; i++ {
		term.Set(xN)
		nf := newF().SetInt(nFactorial)
		term.Quo(term, nf)
		z.Add(z, term)

		if loop.terminate(z) {
			break
		}
		// Advance x**index (multiply by x).
		xN.Mul(xN, x)
		// Advance n, n!.
		n.Add(n, bigOne.Int)
		nFactorial.Mul(nFactorial, n)
	}

	return z

}

// integerPower returns x**exp where exp is an int64 of size <= intBits.
func integerPower(x *big.Float, exp int64) *big.Float {
	z := newF().SetInt64(1)
	y := newF().Set(x)
	// For each loop, we compute a x**n where n is a power of two.
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
