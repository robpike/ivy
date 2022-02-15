// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"math/big"
)

func sinh(c Context, v Value) Value {
	if u, ok := v.(Complex); ok {
		if !isZero(u.imag) {
			return complexSinh(c, u)
		}
		v = u.real
	}
	return evalFloatFunc(c, v, floatSinh)
}

func cosh(c Context, v Value) Value {
	if u, ok := v.(Complex); ok {
		if !isZero(u.imag) {
			return complexCosh(c, u)
		}
		v = u.real
	}
	return evalFloatFunc(c, v, floatCosh)
}

func tanh(c Context, v Value) Value {
	if u, ok := v.(Complex); ok {
		if !isZero(u.imag) {
			return complexTanh(c, u)
		}
		v = u.real
	}
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

func complexSinh(c Context, v Complex) Value {
	// Use the formula: sinh(x+yi) = sinh(x)cos(y) + i cosh(x)sin(y)
	// First turn v into (a + bi) where a and b are big.Floats.
	x := floatSelf(c, v.real).Float
	y := floatSelf(c, v.imag).Float
	sinhX := floatSinh(c, x)
	cosY := floatCos(c, y)
	coshX := floatCosh(c, x)
	sinY := floatSin(c, y)
	lhs := sinhX.Mul(sinhX, cosY)
	rhs := coshX.Mul(coshX, sinY)
	return newComplex(BigFloat{lhs}, BigFloat{rhs}).shrink()
}

func complexCosh(c Context, v Complex) Value {
	// Use the formula: cosh(x+yi) = cosh(x)cos(y) + i sinh(x)sin(y)
	// First turn v into (a + bi) where a and b are big.Floats.
	x := floatSelf(c, v.real).Float
	y := floatSelf(c, v.imag).Float
	coshX := floatCosh(c, x)
	cosY := floatCos(c, y)
	sinhX := floatSinh(c, x)
	sinY := floatSin(c, y)
	lhs := coshX.Mul(coshX, cosY)
	rhs := sinhX.Mul(sinhX, sinY)
	return newComplex(BigFloat{lhs}, BigFloat{rhs}).shrink()
}

func complexTanh(c Context, v Complex) Value {
	// Use the formula: tanh(x+yi) = (sinh(2x) + i sin(2y)/(cosh(2x) + cos(2y))
	// First turn v into (a + bi) where a and b are big.Floats.
	x := floatSelf(c, v.real).Float
	y := floatSelf(c, v.imag).Float
	// Double them - all the arguments are 2X.
	x.Mul(x, floatTwo)
	y.Mul(y, floatTwo)
	sinh2X := floatSinh(c, x)
	sin2Y := floatSin(c, y)
	cosh2X := floatCosh(c, x)
	cos2Y := floatCos(c, y)
	den := cosh2X.Add(cosh2X, cos2Y)
	if den.Sign() == 0 {
		Errorf("tangent is infinite")
	}
	return newComplex(BigFloat{sinh2X.Quo(sinh2X, den)}, BigFloat{sin2Y.Quo(sin2Y, den)}).shrink()
}
