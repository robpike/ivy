// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"math"
	"math/big"
)

func power(c Context, u, v Value) Value {
	// Because of the promotions done in binary.go, if one
	// argument is complex, they both are.
	if _, ok := u.(Complex); ok {
		return complexPower(c, u.(Complex), v.(Complex)).shrink()
	}
	if sgn(c, u) < 0 {
		return complexPower(c, NewComplex(c, u, zero), NewComplex(c, v, zero)).shrink()
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
	z := exponential(c, newFloat(c), floatSelf(c, v).Float)
	return BigFloat{z}.shrink()
}

// expComplex returns e**v where v is Complex.
func expComplex(c Context, v Complex) Value {
	// Use the Euler formula: e**ix == cos x + i sin x.
	// Thus e**(x+iy) == e**x * (cos y + i sin y).
	// First turn v into (a + bi) where a and b are big.Floats.
	x := floatSelf(c, v.real).Float
	y := floatSelf(c, v.imag).Float
	eToX := exponential(c, newFloat(c), x)
	cosY := floatCos(c, y)
	sinY := floatSin(c, y)
	return NewComplex(c, BigFloat{cosY.Mul(cosY, eToX)}, BigFloat{sinY.Mul(sinY, eToX)})
}

// floatPower computes bx to the power of bexp.
func floatPower(c Context, bx, bexp BigFloat) Value {
	x := bx.Float
	fexp := newFloat(c).Set(bexp.Float)
	positive := true
	switch fexp.Sign() {
	case 0:
		return BigFloat{newFloat(c).SetInt64(1)}
	case -1:
		if x.Sign() == 0 {
			c.Errorf("negative exponent of zero")
		}
		positive = false
		fexp = c.EvalUnary("-", bexp).toType("**", c, bigFloatType).(BigFloat).Float
	}
	// Easy cases.
	switch {
	case x.Cmp(floatOne) == 0, x.Sign() == 0:
		return bx
	case fexp.Cmp(floatHalf) == 0:
		if sgn(c, bx) < 0 {
			return complexSqrt(c, NewComplex(c, bx, zero))
		}
		z := floatSqrt(c, x)
		if !positive {
			z = z.Quo(floatOne, z)
		}
		return BigFloat{z}
	}
	isInt := true
	exp, acc := fexp.Int64() // No point in doing *big.Ints.
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
		z.Mul(z, exponential(c, newFloat(c), frac))
	}
	if !positive {
		z.Quo(floatOne, z)
	}
	return BigFloat{z}
}

// exponential sets z to the rounded value of e^x, and returns it.

// If z's precision is 0, it is changed to x's precision before the operation.
// Rounding is performed according to z's precision and rounding mode.
//
// The operation uses the Taylor series e^x = ∑(x^n/n!) for n ≥ 0.
func exponential(c Context, z *big.Float, x *big.Float) *big.Float {
	// exp(x) is finite if 0.5 × 2^big.MinExp ≤ exp(x) < 1 × 2^big.MaxExp
	//   ⇒ log(2) × (big.MinExp-1) ≤ x < log(2) × big.MaxExp
	// While this function properly handles values of x outside of this range,
	// exit early on extreme values to prevent long running times and simplify the
	// bounds check to x.exp-1 < log2(big.MaxExp)
	exp := x.MantExp(nil)
	if x.IsInf() || exp > 31 {
		if x.Sign() < 0 {
			return floatZero
		}
		c.Errorf("exponential overflow")
	}

	// The following is based on R. P. Brent, P. Zimmermann, Modern Computer
	// Arithmetic, Cambridge Monographs on Computational and Applied Mathematics
	// (No. 18), Cambridge University Press
	// https://members.loria.fr/PZimmermann/mca/pub226.html
	//
	// Argument reduction: bring x in the range [0.5, 1)×2^-k for faster
	// convergence. This also brings extreme values of x for which exp(x) is
	// 0 or +Inf into a computable range (i.e. for z=x^-k, ∑(z^n/n!) is finite).

	var invert bool
	z.Set(x)
	prec := z.Prec()
	// For z < 0, compute exp(-z) = 1/exp(z).
	// This is to prevent alternating signs in the power series terms and avoid
	// cancellation in the summation, as well as keeping the summation in a
	// known range after argument reduction (1 <= ∑(z^n/n!) < 1+2^(-k+1)).
	if z.Signbit() {
		invert = true
		z.Neg(z)
	}
	// §4.3.1 & §4.4.2 (k ≥ 1)
	k := int(math.Ceil(math.Sqrt(float64(prec))))
	// Working precision (§4.4)
	prec = addPrec(prec, uint(math.Log(float64(prec)))+1)
	if -k < exp {
		// -k <= -1 < exp
		exp += k
		// 0 ≤ k-1 < exp (condition needed to undo argument reduction)
		z.SetMantExp(z, -exp)
		// 2 bits of added precision per multiplication when undoing argument reduction.
		prec += 2 * uint(exp)
	}

	n := new(big.Float)
	t0 := newFloatPrec(prec)
	t1 := newFloatPrec(prec)
	term := newFloatPrec(prec).SetUint64(1)
	sum := newFloatPrec(prec).SetUint64(1)

	// TODO: cannot use loop here since it does not handle the extended precision.
	// term(n) = term(n-1) × x/n is faster than term(n) = x^n / n! (saves one .Mul)
	for i := uint64(1); ; i++ {
		t0.Quo(z, n.SetUint64(i))
		// term.Mul(term, t) and sum.Add(sum, term) require a temp Float for the
		// result. Manage that ourselves by using our own temps t0, t1, then swap the
		// pointers.
		t1.Mul(term, t0)
		t1, term = term, t1
		t1.Add(sum, term)
		t1, sum = sum, t1

		// If term < 1 ulp, we are done. This check is done after the summation since
		// sum may still change if term ≥ 0.5 ulp, depending on rounding mode.
		// term < 1 ulp of sum         ⇒ term < 0.5 × 2^(sum.exp-sum.prec+1)
		// 0 ≤ term < 1 × 2^term.exp   ⇒ 2^term.exp ≤ 2^(sum.exp-sum.prec)
		// Because of argument reduction, 1 ≤ sum < 1+2^(-k+1) ⇒ sum.exp == 1
		if term.Sign() == 0 || term.MantExp(nil) <= sum.MantExp(nil)-int(sum.Prec()) {
			break
		}
	}

	// Undo argument reduction if exp > 0
	for range exp {
		// Prevent temp allocations using the same trick as above
		t0.Mul(sum, sum)
		t0, sum = sum, t0
	}

	if invert {
		// If sum.IsInf the result will be 0 as intended.
		return z.Quo(n.SetUint64(1), sum)
	}
	if sum.IsInf() {
		c.Errorf("exponential overflow")
	}
	return z.Set(sum)
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
	z := NewComplex(c, one, zero)
	y := NewComplex(c, v.real, v.imag)
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
