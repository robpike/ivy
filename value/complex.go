// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"

	"robpike.io/ivy/config"
)

type Complex struct {
	real Value
	imag Value
}

func newComplex(u, v Value) Complex {
	if !simpleNumber(u) || !simpleNumber(v) {
		Errorf("bad complex construction: %v %v", u, v)
	}
	return Complex{u, v}
}

func (c Complex) Components() (Value, Value) {
	return c.real, c.imag
}

func simpleNumber(v Value) bool {
	switch v.(type) {
	case Int, BigInt, BigRat, BigFloat:
		return true
	}
	return false
}

func (c Complex) String() string {
	return "(" + c.Sprint(debugConf) + ")"
}

func (c Complex) Rank() int {
	return 0
}

func (c Complex) Sprint(conf *config.Config) string {
	return fmt.Sprintf("%sj%s", c.real.Sprint(conf), c.imag.Sprint(conf))
}

func (c Complex) ProgString() string {
	return fmt.Sprintf("%sj%s", c.real, c.imag)
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
		return NewMatrix([]int{1, 1}, []Value{c})
	}
	Errorf("%s: cannot convert complex to %s", op, which)
	return nil
}

func (c Complex) isReal() bool {
	return isZero(c.imag)
}

// shrink pulls, if possible, a Complex down to a scalar.
// It also shrinks its components.
func (c Complex) shrink() Value {
	sc := Complex{
		c.real.shrink(),
		c.imag.shrink(),
	}
	if sc.isReal() {
		return sc.real
	}
	return sc
}

// Arithmetic.

func (c Complex) neg(ctx Context) Complex {
	return newComplex(ctx.EvalUnary("-", c.real), ctx.EvalUnary("-", c.imag))
}

func (c Complex) recip(ctx Context) Complex {
	if isZero(c.real) && isZero(c.imag) {
		Errorf("complex reciprocal of zero")
	}
	denom := ctx.EvalBinary(ctx.EvalBinary(c.real, "*", c.real), "+", ctx.EvalBinary(c.imag, "*", c.imag))
	r := ctx.EvalBinary(c.real, "/", denom)
	i := ctx.EvalUnary("-", ctx.EvalBinary(c.imag, "/", denom))
	return newComplex(r, i)
}

func (c Complex) abs(ctx Context) Value {
	mag := ctx.EvalBinary(ctx.EvalBinary(c.real, "*", c.real), "+", ctx.EvalBinary(c.imag, "*", c.imag))
	return ctx.EvalUnary("sqrt", mag)
}

func (c Complex) add(ctx Context, d Complex) Complex {
	return newComplex(ctx.EvalBinary(c.real, "+", d.real), ctx.EvalBinary(c.imag, "+", d.imag))
}

func (c Complex) sub(ctx Context, d Complex) Complex {
	return newComplex(ctx.EvalBinary(c.real, "-", d.real), ctx.EvalBinary(c.imag, "-", d.imag))
}

func (c Complex) mul(ctx Context, d Complex) Complex {
	r := ctx.EvalBinary(ctx.EvalBinary(c.real, "*", d.real), "-", ctx.EvalBinary(c.imag, "*", d.imag))
	i := ctx.EvalBinary(ctx.EvalBinary(d.imag, "*", c.real), "+", ctx.EvalBinary(d.real, "*", c.imag))
	return newComplex(r, i)
}

func (c Complex) div(ctx Context, d Complex) Complex {
	if isZero(d.real) && isZero(d.imag) {
		Errorf("complex division by zero")
	}
	if d.isReal() { // A common case, like dividing by 2.
		denom := ctx.EvalBinary(d.real, "*", d.real)
		r := ctx.EvalBinary(c.real, "*", d.real)
		r = ctx.EvalBinary(r, "/", denom)
		i := ctx.EvalBinary(c.imag, "*", d.real)
		i = ctx.EvalBinary(i, "/", denom)
		return newComplex(r, i)
	}
	denom := ctx.EvalBinary(ctx.EvalBinary(d.real, "*", d.real), "+", ctx.EvalBinary(d.imag, "*", d.imag))
	r := ctx.EvalBinary(ctx.EvalBinary(c.real, "*", d.real), "+", ctx.EvalBinary(c.imag, "*", d.imag))
	r = ctx.EvalBinary(r, "/", denom)
	i := ctx.EvalBinary(ctx.EvalBinary(c.imag, "*", d.real), "-", ctx.EvalBinary(c.real, "*", d.imag))
	i = ctx.EvalBinary(i, "/", denom)
	return newComplex(r, i)
}
