// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"math/big"
)

func asinh(c Context, v Value) Value {
	if u, ok := v.(Complex); ok {
		if !isZero(u.imag) {
			return complexAsinh(c, u)
		}
		v = u.real
	}
	return evalFloatFunc(c, v, floatAsinh)
}

func acosh(c Context, v Value) Value {
	if u, ok := v.(Complex); ok {
		if !isZero(u.imag) {
			return complexAcosh(c, u)
		}
		v = u.real
	}
	if compare(v, 1) < 0 {
		return complexAcosh(c, newComplex(v, zero))
	}
	return evalFloatFunc(c, v, floatAcosh)
}

func atanh(c Context, v Value) Value {
	if u, ok := v.(Complex); ok {
		if !isZero(u.imag) {
			return complexAtanh(c, u)
		}
		v = u.real
	}
	if compare(v, -1) <= 0 || 0 <= compare(v, 1) {
		return complexAtanh(c, newComplex(v, zero))
	}
	return evalFloatFunc(c, v, floatAtanh)
}

// floatAsinh computes asinh(x) using the formula asinh(x) = log(x + sqrt(x²+1)).
// The domain is the real line.
func floatAsinh(c Context, x *big.Float) *big.Float {
	z := newFloat(c).Set(x)
	z.Mul(z, x)
	z.Add(z, floatOne)
	z = floatSqrt(c, z)
	z.Add(z, x)
	return floatLog(c, z)
}

// floatAcosh computes acosh(x) using the formula asinh(x) = log(x + sqrt(x²-1)).
// The domain is the real line >= 1.
func floatAcosh(c Context, x *big.Float) *big.Float {
	if x.Cmp(floatOne) < 0 {
		Errorf("real acosh out of range [1, +∞ )")
	}
	z := newFloat(c).Set(x)
	z.Mul(z, x)
	z.Sub(z, floatOne)
	z = floatSqrt(c, z)
	z.Add(z, x)
	return floatLog(c, z)
}

// floatAtanh computes atanh(x) using the formula asinh(x) = ½log((1+x)/(1-x))
// The domain is  the open interval (-1, 1).
func floatAtanh(c Context, x *big.Float) *big.Float {
	if x.Cmp(floatMinusOne) <= 0 || 0 <= x.Cmp(floatOne) {
		Errorf("real atanh out of range (-1, 1)")
	}
	num := newFloat(c).Add(floatOne, x)
	den := newFloat(c).Sub(floatOne, x)
	z := floatLog(c, newFloat(c).Quo(num, den))
	return z.Quo(z, floatTwo)
}

// complexAsinh computes asinh(x) using the formula asinh(x) = log(x + sqrt(x²+1)).
func complexAsinh(c Context, x Complex) Complex {
	z := x.mul(c, x)
	z = z.add(c, newComplex(one, zero))
	z = complexSqrt(c, z)
	z = z.add(c, x)
	return complexLog(c, z)
}

// complexAcosh computes asinh(x) using the formula asinh(x) = log(x + sqrt(x²-1)).
func complexAcosh(c Context, x Complex) Complex {
	z := x.mul(c, x)
	z = z.sub(c, newComplex(one, zero))
	z = complexSqrt(c, z)
	z = z.add(c, x)
	return complexLog(c, z)
}

// complexAtanh computes asinh(x) using the formula asinh(x) = ½log((1+x)/(1-x))
func complexAtanh(c Context, x Complex) Complex {
	num := complexOne.add(c, x)
	den := complexOne.sub(c, x)
	if isZero(num) || isZero(den) {
		Errorf("atanh is infinite")
	}
	z := num.div(c, den)
	return complexLog(c, z).mul(c, complexHalf)
}
