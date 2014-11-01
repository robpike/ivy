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
	op(z.Int, i.Int)
	return z.shrink()
}

// unaryBigRatOp applies the op to a BigRat.
func unaryBigRatOp(op func(*big.Rat, *big.Rat) *big.Rat, v Value) Value {
	i := v.(BigRat)
	z := bigRatInt64(0)
	op(z.Rat, i.Rat)
	return z.shrink()
}

// unaryVectorOp applies op elementwise to i.
func unaryVectorOp(op string, i Value) Value {
	u := i.(Vector)
	n := make([]Value, u.Len())
	for k := range u {
		n[k] = Unary(op, u[k])
	}
	return ValueSlice(n)
}

// unaryMatrixOp applies op elementwise to i.
func unaryMatrixOp(op string, i Value) Value {
	u := i.(Matrix)
	n := make([]Value, u.data.Len())
	for k := range u.data {
		n[k] = Unary(op, u.data[k])
	}
	return Matrix{
		shape: u.shape,
		data:  ValueSlice(n),
	}
}

var (
	unaryPlus, unaryMinus, unaryRecip *unaryOp
	unaryAbs, unarySignum             *unaryOp
	unaryBitwiseNot, unaryLogicalNot  *unaryOp
	unaryIota, unaryRho, unaryRavel   *unaryOp
	floor, ceil                       *unaryOp
	unaryOps                          map[string]*unaryOp
)

func init() {
	unaryPlus = &unaryOp{
		fn: [numType]unaryFn{
			intType:    func(v Value) Value { return v },
			bigIntType: func(v Value) Value { return v },
			bigRatType: func(v Value) Value { return v },
			vectorType: func(v Value) Value { return v },
			matrixType: func(v Value) Value { return v },
		},
	}

	unaryMinus = &unaryOp{
		fn: [numType]unaryFn{
			intType: func(v Value) Value {
				return -v.(Int)
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
			matrixType: func(v Value) Value {
				return unaryMatrixOp("-", v)
			},
		},
	}

	unaryRecip = &unaryOp{
		fn: [numType]unaryFn{
			intType: func(v Value) Value {
				i := int64(v.(Int))
				if i == 0 {
					panic(Error("division by zero"))
				}
				return BigRat{
					Rat: big.NewRat(0, 1).SetFrac64(1, i),
				}.shrink()
			},
			bigIntType: func(v Value) Value {
				// Zero division cannot happen for unary.
				return BigRat{
					Rat: big.NewRat(0, 1).SetFrac(bigOne.Int, v.(BigInt).Int),
				}.shrink()
			},
			bigRatType: func(v Value) Value {
				// Zero division cannot happen for unary.
				r := v.(BigRat)
				return BigRat{
					Rat: big.NewRat(0, 1).SetFrac(r.Denom(), r.Num()),
				}.shrink()
			},
			vectorType: func(v Value) Value {
				return unaryVectorOp("/", v)
			},
			matrixType: func(v Value) Value {
				return unaryMatrixOp("/", v)
			},
		},
	}

	unarySignum = &unaryOp{
		fn: [numType]unaryFn{
			intType: func(v Value) Value {
				i := int64(v.(Int))
				if i > 0 {
					return one
				}
				if i < 0 {
					return minusOne
				}
				return zero
			},
			bigIntType: func(v Value) Value {
				return Int(v.(BigInt).Sign())
			},
			bigRatType: func(v Value) Value {
				return Int(v.(BigRat).Sign())
			},
			vectorType: func(v Value) Value {
				return unaryVectorOp("sgn", v)
			},
			matrixType: func(v Value) Value {
				return unaryMatrixOp("sgn", v)
			},
		},
	}

	unaryBitwiseNot = &unaryOp{
		fn: [numType]unaryFn{
			intType: func(v Value) Value {
				return ^v.(Int)
			},
			bigIntType: func(v Value) Value {
				// Lots of ways to do this, here's one.
				return BigInt{Int: bigInt64(0).Xor(v.(BigInt).Int, bigMinusOne.Int)}
			},
			vectorType: func(v Value) Value {
				return unaryVectorOp("^", v)
			},
			matrixType: func(v Value) Value {
				return unaryMatrixOp("^", v)
			},
		},
	}

	unaryLogicalNot = &unaryOp{
		fn: [numType]unaryFn{
			intType: func(v Value) Value {
				if v.(Int) == 0 {
					return one
				}
				return zero
			},
			bigIntType: func(v Value) Value {
				if v.(BigInt).Sign() == 0 {
					return one
				}
				return zero
			},
			vectorType: func(v Value) Value {
				return unaryVectorOp("~", v)
			},
			matrixType: func(v Value) Value {
				return unaryMatrixOp("~", v)
			},
		},
	}

	unaryAbs = &unaryOp{
		fn: [numType]unaryFn{
			intType: func(v Value) Value {
				i := v.(Int)
				if i < 0 {
					i = -i
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
			matrixType: func(v Value) Value {
				return unaryMatrixOp("abs", v)
			},
		},
	}

	floor = &unaryOp{
		fn: [numType]unaryFn{
			intType:    func(v Value) Value { return v },
			bigIntType: func(v Value) Value { return v },
			bigRatType: func(v Value) Value {
				i := v.(BigRat)
				if i.IsInt() {
					// It can't be an integer, which means we must move up or down.
					panic("min: is int")
				}
				positive := i.Sign() >= 0
				if !positive {
					j := bigRatInt64(0)
					j.Abs(i.Rat)
					i = j
				}
				z := bigInt64(0)
				z.Quo(i.Num(), i.Denom())
				if !positive {
					z.Add(z.Int, bigOne.Int)
					z.Neg(z.Int)
				}
				return z
			},
			vectorType: func(v Value) Value {
				return unaryVectorOp("floor", v)
			},
			matrixType: func(v Value) Value {
				return unaryMatrixOp("floor", v)
			},
		},
	}

	ceil = &unaryOp{
		fn: [numType]unaryFn{
			intType:    func(v Value) Value { return v },
			bigIntType: func(v Value) Value { return v },
			bigRatType: func(v Value) Value {
				i := v.(BigRat)
				if i.IsInt() {
					// It can't be an integer, which means we must move up or down.
					panic("max: is int")
				}
				positive := i.Sign() >= 0
				if !positive {
					j := bigRatInt64(0)
					j.Abs(i.Rat)
					i = j
				}
				z := bigInt64(0)
				z.Quo(i.Num(), i.Denom())
				if positive {
					z.Add(z.Int, bigOne.Int)
				} else {
					z.Neg(z.Int)
				}
				return z
			},
			vectorType: func(v Value) Value {
				return unaryVectorOp("ceil", v)
			},
			matrixType: func(v Value) Value {
				return unaryMatrixOp("ceil", v)
			},
		},
	}

	unaryIota = &unaryOp{
		fn: [numType]unaryFn{
			intType: func(v Value) Value {
				i := v.(Int)
				if i <= 0 || maxInt < i {
					panic(Errorf("bad iota %d", i))
				}
				n := make([]Value, i)
				for k := range n {
					n[k] = Int(k + conf.Origin())
				}
				return ValueSlice(n)
			},
		},
	}

	unaryRho = &unaryOp{
		fn: [numType]unaryFn{
			// TODO: scalars should return an empty vector
			vectorType: func(v Value) Value {
				return Int(len(v.(Vector)))
			},
			matrixType: func(v Value) Value {
				return v.(Matrix).shape
			},
		},
	}

	unaryRavel = &unaryOp{
		fn: [numType]unaryFn{
			intType: func(v Value) Value {
				return ValueSlice([]Value{v})
			},
			bigIntType: func(v Value) Value {
				return ValueSlice([]Value{v})
			},
			bigRatType: func(v Value) Value {
				return ValueSlice([]Value{v})
			},
			vectorType: func(v Value) Value {
				return v
			},
			matrixType: func(v Value) Value {
				return v.(Matrix).data
			},
		},
	}

	unaryOps = map[string]*unaryOp{
		"+":     unaryPlus,
		"-":     unaryMinus,
		"/":     unaryRecip,
		"sgn":   unarySignum,
		"^":     unaryBitwiseNot,
		"~":     unaryLogicalNot,
		"abs":   unaryAbs,
		"ceil":  ceil,
		"floor": floor,
		"iota":  unaryIota,
		"rho":   unaryRho,
		",":     unaryRavel,
	}
}
