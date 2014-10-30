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
	unaryAbs, unaryIota, unaryMin, unaryMax                 *unaryOp
	unaryOps                                                map[string]*unaryOp
)

func init() {
	unaryPlus = &unaryOp{
		fn: [numType]unaryFn{
			intType:    func(v Value) Value { return v },
			bigIntType: func(v Value) Value { return v },
			bigRatType: func(v Value) Value { return v },
			vectorType: func(v Value) Value { return v },
		},
	}

	unaryMinus = &unaryOp{
		fn: [numType]unaryFn{
			intType: func(v Value) Value {
				i := v.(Int)
				i.x = -i.x
				return i
			},
			bigIntType: func(v Value) Value {
				return unaryBigIntOp((*big.Int).Neg, v)
			},
			bigRatType: func(v Value) Value {
				return unaryBigRatOp((*big.Rat).Neg, v)
			},
			vectorType: func(v Value) Value {
				return unaryVectorOp("-", v)
			},
		},
	}

	unaryBitwiseNot = &unaryOp{
		fn: [numType]unaryFn{
			intType: func(v Value) Value {
				i := v.(Int)
				i.x = ^i.x
				return i
			},
			bigIntType: func(v Value) Value {
				// Lots of ways to do this, here's one.
				i := v.(BigInt)
				z := bigInt64(0)
				z.x.Xor(i.x, bigMinusOne.x)
				return z
			},
			vectorType: func(v Value) Value {
				return unaryVectorOp("^", v)
			},
		},
	}

	unaryLogicalNot = &unaryOp{
		fn: [numType]unaryFn{
			intType: func(v Value) Value {
				if v.(Int).x == 0 {
					return one
				}
				return zero
			},
			bigIntType: func(v Value) Value {
				i := v.(BigInt)
				if i.x.Sign() == 0 {
					return one
				}
				return zero
			},
			vectorType: func(v Value) Value {
				return unaryVectorOp("!", v)
			},
		},
	}

	unaryAbs = &unaryOp{
		fn: [numType]unaryFn{
			intType: func(v Value) Value {
				i := v.(Int)
				if i.x < 0 {
					i.x = -i.x
				}
				return i
			},
			bigIntType: func(v Value) Value {
				return unaryBigIntOp((*big.Int).Abs, v)
			},
			bigRatType: func(v Value) Value {
				return unaryBigRatOp((*big.Rat).Abs, v)
			},
			vectorType: func(v Value) Value {
				return unaryVectorOp("abs", v)
			},
		},
	}

	unaryMin = &unaryOp{
		// Floor.
		fn: [numType]unaryFn{
			intType:    func(v Value) Value { return v },
			bigIntType: func(v Value) Value { return v },
			bigRatType: func(v Value) Value {
				i := v.(BigRat)
				if i.x.IsInt() {
					// It can't be an integer, which means we must move up or down.
					panic("min: is int")
				}
				positive := i.x.Sign() >= 0
				if !positive {
					j := bigRatInt64(0)
					j.x.Abs(i.x)
					i = j
				}
				num := i.x.Num()
				denom := i.x.Denom()
				z := bigInt64(0)
				if positive {
					z.x.Quo(num, denom)
				} else {
					z.x.Quo(num, denom)
					z.x.Add(z.x, bigOne.x)
					z.x.Neg(z.x)
				}
				return z
			},
			vectorType: func(v Value) Value {
				return unaryVectorOp("min", v)
			},
		},
	}

	unaryMax = &unaryOp{
		// Ceiling.
		fn: [numType]unaryFn{
			intType:    func(v Value) Value { return v },
			bigIntType: func(v Value) Value { return v },
			bigRatType: func(v Value) Value {
				i := v.(BigRat)
				if i.x.IsInt() {
					// It can't be an integer, which means we must move up or down.
					panic("max: is int")
				}
				positive := i.x.Sign() >= 0
				if !positive {
					j := bigRatInt64(0)
					j.x.Abs(i.x)
					i = j
				}
				num := i.x.Num()
				denom := i.x.Denom()
				z := bigInt64(0)
				if positive {
					z.x.Quo(num, denom)
					z.x.Add(z.x, bigOne.x)
				} else {
					z.x.Quo(num, denom)
					z.x.Neg(z.x)
				}
				return z
			},
			vectorType: func(v Value) Value {
				return unaryVectorOp("max", v)
			},
		},
	}

	unaryIota = &unaryOp{
		fn: [numType]unaryFn{
			intType: func(v Value) Value {
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
		},
	}

	unaryOps = map[string]*unaryOp{
		"+":    unaryPlus,
		"-":    unaryMinus,
		"^":    unaryBitwiseNot,
		"!":    unaryLogicalNot,
		"abs":  unaryAbs,
		"max":  unaryMax,
		"min":  unaryMin,
		"iota": unaryIota,
	}
}
