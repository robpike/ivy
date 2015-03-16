// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

func power(u, v Value) Value {
	return floatPower(floatSelf(u).(BigFloat), floatSelf(v).(BigFloat))
}

// floatPower computes bx to the power of bexp.
func floatPower(bx, bexp BigFloat) Value {
	x := bx.Float
	fexp := bexp.Float
	positive := true
	switch fexp.Sign() {
	case 0:
		return one
	case -1:
		if x.Sign() == 0 {
			Errorf("negative exponent of zero")
		}
		positive = false
		fexp = Unary("-", bexp).toType(bigFloatType).(BigFloat).Float
	}
	isInt := true
	exp, acc := fexp.Int64() // No point in doing *big.Ints now. TODO?
	if acc == big.Above || exp > 1e6 {
		Errorf("exponent too large")
	}
	if acc != big.Exact {
		isInt = false
	}
	// We square the result until we're at the largest z that is x^(2^n).
	// TODO: Find a better algorithm.
	z := newF()
	if exp >= 1 {
		z = newF().Set(x)
		soFar := int64(1)
		for 2*soFar <= exp {
			z.Mul(z, z)
			soFar *= 2
		}
		// Continue until what's left is the fractional exponent.
		// We'll do the fractional part with the Taylor series for e**x,
		// which converges fast when x < 1.
		for exp > soFar {
			z.Mul(z, x)
			exp--
		}
	} else {
		z.SetInt64(1)
	}
	if !isInt {
		f64exp, _ := fexp.Float64()
		frac := f64exp - float64(int64(f64exp))
		// x**frac is e**(frac*log x)
		logx := floatLog(x)
		y := newF().SetFloat64(frac)
		y.Mul(y, logx)
		z.Mul(z, exponential(y))
	}
	if !positive {
		one := newF().SetInt64(1)
		z.Quo(one, z)
	}
	return BigFloat{z}.shrink()
}

// exponential computes exp(x) using the Taylor series. It converges quickly
// since we call it with only small values of x.
func exponential(x *big.Float) *big.Float {
	// The Taylor series for e**x, exp(x), is 1 + x + x²/2! + x³/3! ...

	one := newF().SetInt64(1)
	xN := newF().Set(x)
	term := newF()
	n := newF().Set(one)
	nFactorial := newF().Set(one)
	z := newF().SetInt64(1)

	loop := newLoop("exponential", x, 1000)
	for {
		term.Set(xN)
		term.Quo(term, nFactorial)
		z.Add(z, term)

		if loop.terminate(z) {
			break
		}
		// Advance x**index (multiply by x).
		xN.Mul(xN, x)
		// Advance n, n!.
		n.Add(n, one)
		nFactorial.Mul(nFactorial, n)
	}

	return z

}
