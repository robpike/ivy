// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

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
	return floatSinhCosh(c, x, '-')
}

// floatCosh computes sinh(x) = (e**x + e**-x)/2.
func floatCosh(c Context, x *big.Float) *big.Float {
	return floatSinhCosh(c, x, '+')
}

// floatSinhCosh computes {sinh|cosh}(x) = (e**x op e**-x)/2 where op is + for cosh and - for sinh.
func floatSinhCosh(c Context, x *big.Float, op rune) *big.Float {
	if x.IsInf() {
		Errorf("hyperbolic sine or cosine of infinity")
	}
	left := exponential(c.Config(), x)
	right := newFloat(c).Set(floatOne)
	right.Quo(right, left)
	if op == '-' {
		left.Sub(left, right)
	} else {
		left.Add(left, right)
	}
	return left.Quo(left, floatTwo)
}

// floatTanh computes tanh(x) = (e**2x-1)/(e**2x+1).
func floatTanh(c Context, x *big.Float) *big.Float {
	if x.IsInf() {
		Errorf("hyperbolic tangent of infinity")
	}
	z := newFloat(c).Set(x)
	z.Mul(z, floatTwo)
	left := exponential(c.Config(), z)
	right := newFloat(c).Set(left)
	left.Sub(left, floatOne)
	right.Add(right, floatOne)
	return left.Quo(left, right)
}
