// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

// Binary operators.

// To aovid initialization cycles when we refer to the ops from inside
// themselves, we use an init function to initialize the ops.

// binaryArithType returns the maximum of the two types,
// so the smaller value is appropriately up-converted.
func binaryArithType(t1, t2 valueType) valueType {
	if t1 > t2 {
		return t1
	}
	return t2
}

// powType is like binaryArithType but never returns smaller than BigInt,
// because the only implementation of exponentiation we have is in big.Int.
func powType(t1, t2 valueType) valueType {
	if t1 == intType {
		t1 = bigIntType
	}
	return binaryArithType(t1, t2)
}

// rationalType promotes scalars to rationals so we can do rational division.
func rationalType(t1, t2 valueType) valueType {
	if t1 < bigRatType {
		t1 = bigRatType
	}
	return binaryArithType(t1, t2)
}

// shiftCount converts x to an unsigned integer.
func shiftCount(x Value) uint {
	switch count := x.(type) {
	case Int:
		if count.x < 0 || count.x >= maxInt {
			panic(Errorf("illegal shift count %d", count.x))
		}
		return uint(count.x)
	case BigInt:
		// Must be small enough for an int; that will happen if
		// the LHS is a BigInt because the RHS will have been lifted.
		reduced := count.shrink()
		if _, ok := reduced.(Int); ok {
			return shiftCount(reduced)
		}
	}
	panic(Error("illegal shift count type"))
}

// binaryVectorOp applies op elementwise to i and j.
func binaryVectorOp(i Value, op string, j Value) Value {
	u, v := i.(Vector), j.(Vector)
	if len(u.x) == 1 {
		n := make([]Value, v.Len())
		for k := range v.x {
			n[k] = Binary(u.x[0], op, v.x[k])
		}
		return ValueSlice(n)
	}
	if len(v.x) == 1 {
		n := make([]Value, u.Len())
		for k := range u.x {
			n[k] = Binary(u.x[k], op, v.x[0])
		}
		return ValueSlice(n)
	}
	u.sameLength(v)
	n := make([]Value, u.Len())
	for k := range u.x {
		n[k] = Binary(u.x[k], op, v.x[k])
	}
	return ValueSlice(n)
}

func binaryBigIntOp(u Value, op func(*big.Int, *big.Int, *big.Int) *big.Int, v Value) Value {
	i, j := u.(BigInt), v.(BigInt)
	var z BigInt
	op(&z.x, &i.x, &j.x)
	return z.shrink()
}

func binaryBigRatOp(u Value, op func(*big.Rat, *big.Rat, *big.Rat) *big.Rat, v Value) Value {
	i, j := u.(BigRat), v.(BigRat)
	var z BigRat
	op(&z.x, &i.x, &j.x)
	return z.shrink()
}

// bigIntPow is the "op" for pow on *big.Int. Different signature for Exp means we can't use *big.Exp directly.
func bigIntPow(i, j, k *big.Int) *big.Int {
	i.Exp(j, k, nil)
	return i
}

// toInt turns the boolean into 0 or 1.
func toInt(t bool) Value {
	if t {
		return one
	}
	return zero
}

var (
	add, sub, mul, pow        *binaryOp
	quo, idiv, imod, div, mod *binaryOp
	and, or, xor, lsh, rsh    *binaryOp
	eq, ne, lt, le, gt, ge    *binaryOp
	binaryOps                 map[string]*binaryOp
)

var (
	zero        = valueInt64(0)
	one         = valueInt64(1)
	minusOne    = valueInt64(-1)
	bigZero     = bigInt64(0)
	bigOne      = bigInt64(1)
	bigMinusOne = bigInt64(-1)
)

func init() {
	add = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				return valueInt64(u.(Int).x + v.(Int).x)
			},
			func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Add, v)
			},
			func(u, v Value) Value {
				return binaryBigRatOp(u, (*big.Rat).Add, v)
			},
			func(u, v Value) Value {
				return binaryVectorOp(u, "+", v)
			},
		},
	}

	sub = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				return valueInt64(u.(Int).x - v.(Int).x)
			},
			func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Sub, v)
			},
			func(u, v Value) Value {
				return binaryBigRatOp(u, (*big.Rat).Sub, v)
			},
			func(u, v Value) Value {
				return binaryVectorOp(u, "-", v)
			},
		},
	}

	mul = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				return valueInt64(u.(Int).x * v.(Int).x)
			},
			func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Mul, v)
			},
			func(u, v Value) Value {
				return binaryBigRatOp(u, (*big.Rat).Mul, v)
			},
			func(u, v Value) Value {
				return binaryVectorOp(u, "*", v)
			},
		},
	}

	quo = &binaryOp{ // Rational division.
		whichType: rationalType, // Use BigRats to avoid the analysis here.
		fn: [numType]binaryFn{
			nil,
			nil,
			func(u, v Value) Value {
				x := v.(BigRat)
				if x.x.Sign() == 0 {
					panic(Error("division by zero"))
				}
				return binaryBigRatOp(u, (*big.Rat).Quo, v) // True division.
			},
			func(u, v Value) Value {
				return binaryVectorOp(u, "/", v)
			},
		},
	}

	idiv = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				if v.(Int).x == 0 {
					panic(Error("division by zero"))
				}
				return valueInt64(u.(Int).x / v.(Int).x)
			},
			func(u, v Value) Value {
				x := v.(BigInt)
				if x.x.Sign() == 0 {
					panic(Error("division by zero"))
				}
				return binaryBigIntOp(u, (*big.Int).Quo, v) // Go-like division.
			},
			nil, // Not defined for rationals. Use div.
			func(u, v Value) Value {
				return binaryVectorOp(u, "idiv", v)
			},
		},
	}

	imod = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				if v.(Int).x == 0 {
					panic(Error("modulo by zero"))
				}
				return valueInt64(u.(Int).x % v.(Int).x)
			},
			func(u, v Value) Value {
				x := v.(BigInt)
				if x.x.Sign() == 0 {
					panic(Error("modulo by zero"))
				}
				return binaryBigIntOp(u, (*big.Int).Rem, v) // Go-like modulo.
			},
			nil, // Not defined for rationals. Use mod.
			func(u, v Value) Value {
				return binaryVectorOp(u, "imod", v)
			},
		},
	}

	div = &binaryOp{ // Euclidean integer division.
		whichType: powType, // Use BigInts to avoid the analysis here.
		fn: [numType]binaryFn{
			nil,
			func(u, v Value) Value {
				x := v.(BigInt)
				if x.x.Sign() == 0 {
					panic(Error("division by zero"))
				}
				return binaryBigIntOp(u, (*big.Int).Div, v) // Euclidean division.
			},
			nil, // Not defined for rationals. Use div.
			func(u, v Value) Value {
				return binaryVectorOp(u, "div", v)
			},
		},
	}

	mod = &binaryOp{ // Euclidean integer modulus.
		whichType: powType, // Use BigInts to avoid the analysis here.
		fn: [numType]binaryFn{
			nil,
			func(u, v Value) Value {
				x := v.(BigInt)
				if x.x.Sign() == 0 {
					panic(Error("modulo by zero"))
				}
				return binaryBigIntOp(u, (*big.Int).Mod, v) // Euclidan modulo.
			},
			nil, // Not defined for rationals. Use mod.
			func(u, v Value) Value {
				return binaryVectorOp(u, "mod", v)
			},
		},
	}

	pow = &binaryOp{
		whichType: powType,
		fn: [numType]binaryFn{
			nil, // Use BigInt for this.
			func(u, v Value) Value {
				return binaryBigIntOp(u, bigIntPow, v)
			},
			nil, // TODO. Not too hard, but messy.
			func(u, v Value) Value {
				return binaryVectorOp(u, "**", v)
			},
		},
	}

	and = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				return valueInt64(u.(Int).x & v.(Int).x)
			},
			func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).And, v)
			},
			nil,
			func(u, v Value) Value {
				return binaryVectorOp(u, "&", v)
			},
		},
	}

	or = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				return valueInt64(u.(Int).x | v.(Int).x)
			},
			func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Or, v)
			},
			nil,
			func(u, v Value) Value {
				return binaryVectorOp(u, "|", v)
			},
		},
	}

	xor = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				return valueInt64(u.(Int).x ^ v.(Int).x)
			},
			func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Xor, v)
			},
			nil,
			func(u, v Value) Value {
				return binaryVectorOp(u, "^", v)
			},
		},
	}

	lsh = &binaryOp{
		whichType: powType, // Shifts are like power: let BigInt do the work.
		fn: [numType]binaryFn{
			nil, // Use BigInt for this.
			func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				var z BigInt
				z.x.Lsh(&i.x, shiftCount(j))
				return z.shrink()
			},
			nil,
			func(u, v Value) Value {
				return binaryVectorOp(u, "<<", v)
			},
		},
	}

	rsh = &binaryOp{
		whichType: powType, // Shifts are like power: let BigInt do the work.
		fn: [numType]binaryFn{
			nil, // Use BigInt for this.
			func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				var z BigInt
				z.x.Rsh(&i.x, shiftCount(j))
				return z.shrink()
			},
			nil,
			func(u, v Value) Value {
				return binaryVectorOp(u, ">>", v)
			},
		},
	}

	eq = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				return toInt(u.(Int).x == v.(Int).x)
			},
			func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.x.Cmp(&j.x) == 0)
			},
			func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.x.Cmp(&j.x) == 0)
			},
			func(u, v Value) Value {
				return binaryVectorOp(u, "==", v)
			},
		},
	}

	ne = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				return toInt(u.(Int).x != v.(Int).x)
			},
			func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.x.Cmp(&j.x) != 0)
			},
			func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.x.Cmp(&j.x) != 0)
			},
			func(u, v Value) Value {
				return binaryVectorOp(u, "!=", v)
			},
		},
	}

	lt = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				return toInt(u.(Int).x < v.(Int).x)
			},
			func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.x.Cmp(&j.x) < 0)
			},
			func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.x.Cmp(&j.x) < 0)
			},
			func(u, v Value) Value {
				return binaryVectorOp(u, "<", v)
			},
		},
	}

	le = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				return toInt(u.(Int).x <= v.(Int).x)
			},
			func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.x.Cmp(&j.x) <= 0)
			},
			func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.x.Cmp(&j.x) <= 0)
			},
			func(u, v Value) Value {
				return binaryVectorOp(u, "<=", v)
			},
		},
	}

	gt = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				return toInt(u.(Int).x > v.(Int).x)
			},
			func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.x.Cmp(&j.x) > 0)
			},
			func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.x.Cmp(&j.x) > 0)
			},
			func(u, v Value) Value {
				return binaryVectorOp(u, ">", v)
			},
		},
	}

	ge = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				return toInt(u.(Int).x >= v.(Int).x)
			},
			func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.x.Cmp(&j.x) >= 0)
			},
			func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.x.Cmp(&j.x) >= 0)
			},
			func(u, v Value) Value {
				return binaryVectorOp(u, ">=", v)
			},
		},
	}

	binaryOps = map[string]*binaryOp{
		"+":    add,
		"-":    sub,
		"*":    mul,
		"/":    quo,  // Exact rational division.
		"idiv": idiv, // Go-like truncating integer division.
		"imod": imod, // Go-like integer moduls.
		"div":  div,  // Euclidean integer division.
		"mod":  mod,  // Euclidean integer division.
		"**":   pow,
		"&":    and,
		"|":    or,
		"^":    xor,
		"<<":   lsh,
		">>":   rsh,
		"==":   eq,
		"!=":   ne,
		"<":    lt,
		"<=":   le,
		">":    gt,
		">=":   ge,
	}
}
