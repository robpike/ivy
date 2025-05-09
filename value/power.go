// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"math/big"

	"robpike.io/ivy/config"
)

func power(c Context, u, v Value) Value {
	// Because of the promotions done in binary.go, if one
	// argument is complex, they both are.
	if _, ok := u.(Complex); ok {
		return complexPower(c, u.(Complex), v.(Complex)).shrink()
	}
	if sgn(c, u) < 0 {
		return complexPower(c, NewComplex(u, zero), NewComplex(v, zero)).shrink()
	}
	return floatPower(c, floatSelf(c, u), floatSelf(c, v)).shrink()
}

func exp(c Context, v Value) Value {
	if u, ok := v.(Complex); ok {
		if !isZero(u.imag) {
			return expComplex(c, u)
		}
		v = u.real
	}
	z := exponential(c.Config(), floatSelf(c, v).Float)
	return BigFloat{z}.shrink()
}

// expComplex returns e**v where v is Complex.
func expComplex(c Context, v Complex) Value {
	// Use the Euler formula: e**ix == cos x + i sin x.
	// Thus e**(x+iy) == e**x * (cos y + i sin y).
	// First turn v into (a + bi) where a and b are big.Floats.
	x := floatSelf(c, v.real).Float
	y := floatSelf(c, v.imag).Float
	eToX := exponential(c.Config(), x)
	cosY := floatCos(c, y)
	sinY := floatSin(c, y)
	return NewComplex(BigFloat{cosY.Mul(cosY, eToX)}, BigFloat{sinY.Mul(sinY, eToX)})
}

// floatPower computes bx to the power of bexp.
func floatPower(c Context, bx, bexp BigFloat) Value {
	x := bx.Float
	fexp := newFloat(c).Set(bexp.Float)
	positive := true
	conf := c.Config()
	switch fexp.Sign() {
	case 0:
		return BigFloat{newFloat(c).SetInt64(1)}
	case -1:
		if x.Sign() == 0 {
			Errorf("negative exponent of zero")
		}
		positive = false
		fexp = c.EvalUnary("-", bexp).toType("**", conf, bigFloatType).(BigFloat).Float
	}
	// Easy cases.
	switch {
	case x.Cmp(floatOne) == 0, x.Sign() == 0:
		return bx
	case fexp.Cmp(floatHalf) == 0:
		if sgn(c, bx) < 0 {
			return complexSqrt(c, NewComplex(bx, zero))
		}
		z := floatSqrt(c, x)
		if !positive {
			z = z.Quo(floatOne, z)
		}
		return BigFloat{z}
	}
	isInt := true
	exp, acc := fexp.Int64() // No point in doing *big.Ints now. TODO?
	if acc != big.Exact {
		isInt = false
	}
	// Integer part.
	z := integerPower(c, x, exp)
	// Fractional part.
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
	return BigFloat{z}
}

// exponential computes exp(x) using the Taylor series.
func exponential(conf *config.Config, x *big.Float) *big.Float {
	// The Taylor series for e**x, exp(x), is 1 + x + x²/2! + x³/3! ...

	// exp(x) is finite if 0.5 * 2^big.MinExp <= exp(x) < 1 * 2^big.MaxExp
	//   => log(2) * (big.MinExp-1) <= x < log(2) * big.MaxExp
	xTemp := new(big.Float)
	xTemp.Sub(xTemp.SetMantExp(floatLog2, 31), floatLog2) // log(2)*big.MantExp
	if x.Cmp(xTemp) >= 0 {
		return new(big.Float).SetInf(false)
	}
	xTemp.Sub(xTemp.SetMantExp(xTemp.Neg(floatLog2), 31), floatLog2) // log(2)*(big.MinExp-1)
	if x.Cmp(xTemp) < 0 {
		return floatZero
	}

	// We need 64 bits of added precision in the worst case.
	const prec = 64
	exp := x.MantExp(nil)
	xTemp.SetPrec(0).SetPrec(conf.FloatPrec()).Set(x)
	// scale |x| > 1 to [0.5, 1) for faster convergence.
	// Scaling |x| further down favorably trades iterations for multiplications
	// when scaling the result, with diminishing returns and no further benefit
	// for |x| < 2^-17, so we scale for |x| >= 2^-16.
	// With exp(1) for example, this results in 17 vs. 59 iterations at the cost
	// of 16 multiplications to scale z.
	const minExp = -16
	if minExp < exp {
		xTemp.SetMantExp(xTemp, -exp+minExp)
		exp += -minExp
	}

	xN := newFxP(conf, prec).Set(xTemp)
	term := newFxP(conf, prec)
	n := newF(conf)
	nFactorial := newFxP(conf, prec).SetUint64(1)
	z := newFxP(conf, prec).SetInt64(1)

	// TODO: cannot use loop here since it does not handle the extended precision.
	for i := uint64(1); ; i++ {
		term.Quo(xN, nFactorial)
		// if term < 1 ulp, we are done. Note that 0 >= z.exp >= 1, so z.exp-z.prec+1 never overflows.
		if term.MantExp(nil) < z.MantExp(nil)-int(z.Prec())+1 {
			break
		}
		z.Add(z, term)

		// Advance x**index (multiply by x).
		xN.Mul(xN, xTemp)
		// Advance n, n!.
		nFactorial.Mul(nFactorial, n.SetUint64(i+1))
	}

	// scale result
	for range exp {
		z.Mul(z, z)
	}

	// use xTemp as the rounded return value since it was allocated with the
	// proper precision.
	return xTemp.Set(z)
}

// integerPower returns x**exp where exp is an int64 of size <= intBits.
func integerPower(c Context, x *big.Float, exp int64) *big.Float {
	z := newFloat(c).SetInt64(1)
	y := newFloat(c).Set(x)
	// For each loop, we compute xⁿ where n is a power of two.
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

// complexIntegerPower returns x**exp where exp is an int64 of size <= intBits.
func complexIntegerPower(c Context, v Complex, exp int64) Complex {
	z := NewComplex(one, zero)
	y := NewComplex(v.real, v.imag)
	// For each loop, we compute xⁿ where n is a power of two.
	for exp > 0 {
		if exp&1 == 1 {
			// This bit contributes. Multiply it into the result.
			z = z.mul(c, y)
		}
		y = y.mul(c, y)
		exp >>= 1
	}
	return z
}

// complexPower computes v to the power of exp.
func complexPower(c Context, v, exp Complex) Value {
	if isZero(exp.imag) {
		// Easy special cases.
		if i, ok := exp.real.(Int); ok {
			switch {
			case i == 0:
				return one
			case i == 1:
				return v
			case i < 0:
				return complexIntegerPower(c, v, -int64(i)).inverse(c)
			default:
				return complexIntegerPower(c, v, int64(i))
			}
		} else if f, ok := exp.real.(BigFloat); ok && f.Cmp(floatHalf) == 0 {
			return complexSqrt(c, v)
		}
	}
	return expComplex(c, complexLog(c, v).mul(c, exp))
}
