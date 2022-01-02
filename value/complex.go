// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"

	"robpike.io/ivy/config"
)

func ParseImaginary(conf *config.Config, s string) (Value, error) {
	// Ignore 'i' suffix when parsing.
	val, err := Parse(conf, s[:len(s)-1])
	if err != nil {
		return nil, err
	}
	if !toBool(val) {
		// Parses '0i' as '0'.
		return Int(0), nil
	}
	return Complex{real: Int(0), imag: val}, nil
}

func newComplex(real Value) Complex {
	return Complex{real: real, imag: Int(0)}
}

type Complex struct {
	real, imag Value
}

func (c Complex) String() string {
	return c.Sprint(debugConf)
}

func (c Complex) Rank() int {
	return 0
}

func (c Complex) Sprint(conf *config.Config) string {
	return fmt.Sprintf("(%s%si)", c.real.Sprint(conf), imagPrefix(c.imag.Sprint(conf)))
}

func (c Complex) ProgString() string {
	return fmt.Sprintf("(%s%si)", c.real.ProgString(), imagPrefix(c.imag.ProgString()))
}

func imagPrefix(s string) string {
	if s[0] == '-' {
		return s
	}
	return "+" + s
}

func (c Complex) Eval(Context) Value {
	return c
}

func (c Complex) Inner() Value {
	return c
}

func (c Complex) toType(op string, conf *config.Config, which valueType) Value {
	switch which {
	case complexType:
		return c
	case vectorType:
		return NewVector([]Value{c})
	case matrixType:
		return NewMatrix([]int{1}, []Value{c})
	}
	if toBool(c.imag) {
		Errorf("%s: cannot convert complex with non-zero imaginary part to %s", op, which)
		return nil
	}
	return c.real.toType(op, conf, which)
}

func (c Complex) shrink() Value {
	if toBool(c.imag) {
		return c
	}
	return c.real
}

func (c Complex) Floor(ctx Context) Value {
	return Complex{ctx.EvalUnary("floor", c.real), ctx.EvalUnary("floor", c.imag)}.shrink()
}

func (c Complex) Ceil(ctx Context) Value {
	return Complex{ctx.EvalUnary("ceil", c.real), ctx.EvalUnary("ceil", c.imag)}.shrink()
}

func (c Complex) Real(ctx Context) Value {
	return c.real
}

func (c Complex) Imag(ctx Context) Value {
	return c.imag
}

// phase a + bi =
//  a = 0, b = 0:  0
//  a = 0, b > 0:  pi/2
//  a = 0, b < 0:  -pi/2
//  a > 0:         atan(b/y)
//  a < 0, b >= 0: atan(b/y) + pi
//  a < 0, b < 0:  atan(b/y) - pi
func (c Complex) Phase(ctx Context) Value {
	if toBool(ctx.EvalBinary(c.real, "==", zero)) {
		if toBool(ctx.EvalBinary(c.imag, "==", zero)) {
			return zero
		} else if toBool(ctx.EvalBinary(c.imag, ">", zero)) {
			return BigFloat{newF(ctx.Config()).Set(floatHalfPi)}
		} else {
			return BigFloat{newF(ctx.Config()).Set(floatMinusHalfPi)}
		}
	}
	slope := ctx.EvalBinary(c.imag, "/", c.real)
	atan := ctx.EvalUnary("atan", slope)
	if toBool(ctx.EvalBinary(c.real, ">", zero)) {
		return atan
	}
	if toBool(ctx.EvalBinary(c.imag, ">=", zero)) {
		return ctx.EvalBinary(atan, "+", BigFloat{newF(ctx.Config()).Set(floatPi)})
	}
	return ctx.EvalBinary(atan, "-", BigFloat{newF(ctx.Config()).Set(floatPi)})
}

func (c Complex) Neg(ctx Context) Value {
	return Complex{ctx.EvalUnary("-", c.real), ctx.EvalUnary("-", c.imag)}.shrink()
}

// sgn z = z / |z|
func (c Complex) Sign(ctx Context) Value {
	return ctx.EvalBinary(c, "/", c.Abs(ctx))
}

// |a+bi| = sqrt (a^2 + b^2)
func (c Complex) Abs(ctx Context) Value {
	aSq := ctx.EvalBinary(c.real, "*", c.real)
	bSq := ctx.EvalBinary(c.imag, "*", c.imag)
	sumSq := ctx.EvalBinary(aSq, "+", bSq)
	return ctx.EvalUnary("sqrt", sumSq)
}

func (c Complex) Cmp(ctx Context, right Complex) bool {
	return toBool(ctx.EvalBinary(c.real, "==", right.real)) && toBool(ctx.EvalBinary(c.imag, "==", right.imag))
}

// (a+bi) + (c+di) = (a+c) + (b+d)i
func (c Complex) Add(ctx Context, right Complex) Complex {
	return Complex{
		real: ctx.EvalBinary(c.real, "+", right.real),
		imag: ctx.EvalBinary(c.imag, "+", right.imag),
	}
}

// (a+bi) - (c+di) = (a-c) + (b-d)i
func (c Complex) Sub(ctx Context, right Complex) Complex {
	return Complex{
		real: ctx.EvalBinary(c.real, "-", right.real),
		imag: ctx.EvalBinary(c.imag, "-", right.imag),
	}
}

// (a+bi) * (c+di) = (ab - bd) + (ad - bc)i
func (c Complex) Mul(ctx Context, right Complex) Complex {
	ac := ctx.EvalBinary(c.real, "*", right.real)
	bd := ctx.EvalBinary(c.imag, "*", right.imag)
	ad := ctx.EvalBinary(c.real, "*", right.imag)
	bc := ctx.EvalBinary(c.imag, "*", right.real)
	return Complex{
		real: ctx.EvalBinary(ac, "-", bd),
		imag: ctx.EvalBinary(ad, "+", bc),
	}
}

// (a+bi) / (c+di) = (ac + bd)/(c^2 + d^2) + ((bc - ad)/(c^2 + d^2))i
func (c Complex) Quo(ctx Context, right Complex) Complex {
	ac := ctx.EvalBinary(c.real, "*", right.real)
	bd := ctx.EvalBinary(c.imag, "*", right.imag)
	ad := ctx.EvalBinary(c.real, "*", right.imag)
	bc := ctx.EvalBinary(c.imag, "*", right.real)
	realNum := ctx.EvalBinary(ac, "+", bd)
	imagNum := ctx.EvalBinary(bc, "-", ad)
	cSq := ctx.EvalBinary(right.real, "*", right.real)
	dSq := ctx.EvalBinary(right.imag, "*", right.imag)
	denom := ctx.EvalBinary(cSq, "+", dSq)
	return Complex{
		real: ctx.EvalBinary(realNum, "/", denom),
		imag: ctx.EvalBinary(imagNum, "/", denom),
	}
}
