// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

// Unary operators.

// To avoid initialization cycles when we refer to the ops from inside
// themselves, we use an init function to initialize the ops.

// unaryBigIntOp applies the op to a BigInt.
func unaryBigIntOp(op func(*big.Int, *big.Int) *big.Int, v Value) Value {
	i := v.(BigInt)
	z := bigInt64(0)
	op(z.x, i.x)
	return z.shrink()
}

// unaryBigRatOp applies the op to a BigRat.
func unaryBigRatOp(op func(*big.Rat, *big.Rat) *big.Rat, v Value) Value {
	i := v.(BigRat)
	z := bigRatInt64(0)
	op(z.x, i.x)
	return z.shrink()
}

// unaryVectorOp applies op elementwise to i.
func unaryVectorOp(op string, i Value) Value {
	u := i.(Vector)
	n := make([]Value, u.Len())
	for k := range u.x {
		n[k] = Unary(op, u.x[k])
	}
	return ValueSlice(n)
}

var (
	unaryPlus, unaryMinus, unaryBitwiseNot, unaryLogicalNot *unaryOp
	unaryAbs, unaryInt, unaryIota                           *unaryOp
	unaryOps                                                map[string]*unaryOp
)

func init() {
	unaryPlus = &unaryOp{
		fn: [numType]unaryFn{
			func(v Value) Value { return v },
			func(v Value) Value { return v },
			func(v Value) Value { return v },
			func(v Value) Value { return v },
		},
	}

	unaryMinus = &unaryOp{
		fn: [numType]unaryFn{
			func(v Value) Value {
				i := v.(Int)
				i.x = -i.x
				return i
			},
			func(v Value) Value {
				return unaryBigIntOp((*big.Int).Neg, v)
			},
			func(v Value) Value {
				return unaryBigRatOp((*big.Rat).Neg, v)
			},
			func(v Value) Value {
				return unaryVectorOp("-", v)
			},
		},
	}

	unaryBitwiseNot = &unaryOp{
		fn: [numType]unaryFn{
			func(v Value) Value {
				i := v.(Int)
				i.x = ^i.x
				return i
			},
			func(v Value) Value {
				// Lots of ways to do this, here's one.
				i := v.(BigInt)
				z := bigInt64(0)
				z.x.Xor(i.x, bigMinusOne.x)
				return z
			},
			nil,
			func(v Value) Value {
				return unaryVectorOp("^", v)
			},
		},
	}

	unaryLogicalNot = &unaryOp{
		fn: [numType]unaryFn{
			func(v Value) Value {
				if v.(Int).x == 0 {
					return one
				}
				return zero
			},
			func(v Value) Value {
				i := v.(BigInt)
				if i.x.Sign() == 0 {
					return one
				}
				return zero
			},
			nil,
			func(v Value) Value {
				return unaryVectorOp("!", v)
			},
		},
	}

	unaryAbs = &unaryOp{
		fn: [numType]unaryFn{
			func(v Value) Value {
				i := v.(Int)
				if i.x < 0 {
					i.x = -i.x
				}
				return i
			},
			func(v Value) Value {
				return unaryBigIntOp((*big.Int).Abs, v)
			},
			func(v Value) Value {
				return unaryBigRatOp((*big.Rat).Abs, v)
			},
			func(v Value) Value {
				return unaryVectorOp("abs", v)
			},
		},
	}

	unaryInt = &unaryOp{
		fn: [numType]unaryFn{
			func(v Value) Value { return v },
			func(v Value) Value { return v },
			func(v Value) Value {
				i := v.(BigRat)
				z := bigInt64(0)
				z.x.Quo(i.x.Num(), i.x.Denom()) // Truncates towards zero.
				return z
			},
			nil,
		},
	}

	unaryIota = &unaryOp{
		fn: [numType]unaryFn{
			func(v Value) Value {
				i := v.(Int)
				if i.x <= 0 || maxInt < i.x {
					panic(Errorf("bad iota %d", i.x))
				}
				n := make([]Value, i.x)
				for k := range n {
					n[k] = Int{x: int64(k) + 1}
				}
				return ValueSlice(n)
			},
			nil,
			nil,
			nil,
		},
	}

	unaryOps = map[string]*unaryOp{
		"+":    unaryPlus,
		"-":    unaryMinus,
		"^":    unaryBitwiseNot,
		"!":    unaryLogicalNot,
		"abs":  unaryAbs,
		"int":  unaryInt, // TODO: should be min and max
		"iota": unaryIota,
	}
}
