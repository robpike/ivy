// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"math/big"
)

func sinh(c Context, v Value) Value {
	return evalFloatFunc(c, v, floatSinh)
}

func cosh(c Context, v Value) Value {
	return evalFloatFunc(c, v, floatCosh)
}

func tanh(c Context, v Value) Value {
	return evalFloatFunc(c, v, floatTanh)
}

// floatSinh computes sinh(x) = (e**x - e**-x)/2.
func floatSinh(c Context, x *big.Float) *big.Float {
	// The Taylor series for sinh(x) is the odd terms of exp(x): x + x³/3! + x⁵/5!...

	conf := c.Config()
	xN := newF(conf).Set(x)
	term := newF(conf)
	n := newF(conf)
	nFactorial := newF(conf).SetUint64(1)
	z := newF(conf).SetInt64(0)

	for loop := newLoop(conf, "sinh", x, 10); ; { // Big exponentials converge slowly.
		term.Set(xN)
		term.Quo(term, nFactorial)
		z.Add(z, term)

		if loop.done(z) {
			break
		}
		// Advance x**index (multiply by x).
		xN.Mul(xN, x)
		xN.Mul(xN, x)
		// Advance n, n!.
		nFactorial.Mul(nFactorial, n.SetUint64(2*loop.i))
		nFactorial.Mul(nFactorial, n.SetUint64(2*loop.i+1))
	}

	return z
}

// floatCosh computes sinh(x) = (e**x + e**-x)/2.
func floatCosh(c Context, x *big.Float) *big.Float {
	// The Taylor series for cosh(x) is the even terms of exp(x): 1 + x²/2! + x⁴/4!...

	conf := c.Config()
	xN := newF(conf).Set(x)
	xN.Mul(xN, x) // x²
	term := newF(conf)
	n := newF(conf)
	nFactorial := newF(conf).SetUint64(2)
	z := newF(conf).SetInt64(1)

	for loop := newLoop(conf, "cosh", x, 10); ; { // Big exponentials converge slowly.
		term.Set(xN)
		term.Quo(term, nFactorial)
		z.Add(z, term)

		if loop.done(z) {
			break
		}
		// Advance x**index (multiply by x).
		xN.Mul(xN, x)
		xN.Mul(xN, x)
		// Advance n, n!.
		nFactorial.Mul(nFactorial, n.SetUint64(2*loop.i+1))
		nFactorial.Mul(nFactorial, n.SetUint64(2*loop.i+2))
	}

	return z
}

// floatTanh computes tanh(x) = sinh(x)/cosh(x)
func floatTanh(c Context, x *big.Float) *big.Float {
	if x.IsInf() {
		Errorf("tanh of infinity")
	}
	denom := floatCosh(c, x)
	if denom.Cmp(floatZero) == 0 {
		Errorf("tanh is infinite")
	}
	num := floatSinh(c, x)
	return num.Quo(num, denom)
}
