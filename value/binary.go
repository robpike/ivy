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

// divType is like binaryArithType but never returns smaller than BigInt,
// because the only implementation of exponentiation we have is in big.Int.
func divType(t1, t2 valueType) valueType {
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

// atLeastVectorType promotes both arguments to at least vectors.
func atLeastVectorType(t1, t2 valueType) valueType {
	if t1 < matrixType && t2 < matrixType {
		return vectorType
	}
	return matrixType
}

// shiftCount converts x to an unsigned integer.
func shiftCount(x Value) uint {
	switch count := x.(type) {
	case Int:
		if count < 0 || count >= maxInt {
			Errorf("illegal shift count %d", count)
		}
		return uint(count)
	case BigInt:
		// Must be small enough for an int; that will happen if
		// the LHS is a BigInt because the RHS will have been lifted.
		reduced := count.shrink()
		if _, ok := reduced.(Int); ok {
			return shiftCount(reduced)
		}
	}
	Errorf("illegal shift count type")
	panic("not reached")
}

func binaryBigIntOp(u Value, op func(*big.Int, *big.Int, *big.Int) *big.Int, v Value) Value {
	i, j := u.(BigInt), v.(BigInt)
	z := bigInt64(0)
	op(z.Int, i.Int, j.Int)
	return z.shrink()
}

func binaryBigRatOp(u Value, op func(*big.Rat, *big.Rat, *big.Rat) *big.Rat, v Value) Value {
	i, j := u.(BigRat), v.(BigRat)
	z := bigRatInt64(0)
	op(z.Rat, i.Rat, j.Rat)
	return z.shrink()
}

func binaryBigFloatOp(c Context, u Value, op func(*big.Float, *big.Float, *big.Float) *big.Float, v Value) Value {
	i, j := u.(BigFloat), v.(BigFloat)
	z := bigFloatInt64(c.Config(), 0)
	op(z.Float, i.Float, j.Float)
	return z.shrink()
}

// bigIntExp is the "op" for exp on *big.Int. Different signature for Exp means we can't use *big.Exp directly.
// Also we need a context (really a config); see the bigIntExpOp function below.
// We know this is not 0**negative.
func bigIntExp(c Context, i, j, k *big.Int) *big.Int {
	if j.Cmp(bigOne.Int) == 0 || j.Sign() == 0 {
		return i.Set(j)
	}
	// -1**n is just parity.
	if j.Cmp(bigMinusOne.Int) == 0 {
		if k.And(k, bigOne.Int).Int64() == 0 {
			return i.Neg(j)
		}
		return i.Set(j)
	}
	// Large exponents can be very expensive.
	// First, it must fit in an int64.
	if k.BitLen() > 63 {
		Errorf("%s**%s: exponent too large", j, k)
	}
	exp := k.Int64()
	if exp < 0 {
		exp = -exp
	}
	mustFit(c.Config(), int64(j.BitLen())*exp)
	i.Exp(j, k, nil)
	return i
}

// bigIntExpOp wraps bigIntExp with a Context and returns the closure as an op.
func bigIntExpOp(c Context) func(i, j, k *big.Int) *big.Int {
	return func(i, j, k *big.Int) *big.Int {
		return bigIntExp(c, i, j, k)
	}
}

// toInt turns the boolean into an Int 0 or 1.
func toInt(t bool) Value {
	if t {
		return one
	}
	return zero
}

// toBool turns the Value into a Go bool.
func toBool(t Value) bool {
	switch t := t.(type) {
	case Int:
		return t != 0
	case Char:
		return t != 0
	case BigInt:
		return t.Sign() != 0
	case BigRat:
		return t.Sign() != 0
	case BigFloat:
		return t.Sign() != 0
	}
	Errorf("cannot convert %T to bool", t)
	panic("not reached")
}

var (
	add, sub, mul, pow                *binaryOp
	quo, idiv, imod, div, mod         *binaryOp
	binaryLog                         *binaryOp
	bitAnd, bitOr, bitXor             *binaryOp
	lsh, rsh                          *binaryOp
	eq, ne, lt, le, gt, ge            *binaryOp
	in, index                         *binaryOp
	logicalAnd, logicalOr, logicalXor *binaryOp
	logicalNand, logicalNor           *binaryOp
	decode, encode                    *binaryOp
	binaryIota, binaryRho             *binaryOp
	binaryCatenate                    *binaryOp
	take, drop                        *binaryOp
	min, max                          *binaryOp
	fill, sel, rot                    *binaryOp
	binaryOps                         map[string]*binaryOp
)

var (
	zero        = Int(0)
	one         = Int(1)
	minusOne    = Int(-1)
	bigZero     = bigInt64(0)
	bigOne      = bigInt64(1)
	bigMinusOne = bigInt64(-1)
)

func init() {
	add = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				return (u.(Int) + v.(Int)).maybeBig()
			},
			bigIntType: func(c Context, u, v Value) Value {
				mustFit(c.Config(), u.(BigInt).BitLen()+1)
				mustFit(c.Config(), v.(BigInt).BitLen()+1)
				return binaryBigIntOp(u, (*big.Int).Add, v)
			},
			bigRatType: func(c Context, u, v Value) Value {
				return binaryBigRatOp(u, (*big.Rat).Add, v)
			},
			bigFloatType: func(c Context, u, v Value) Value {
				return binaryBigFloatOp(c, u, (*big.Float).Add, v)
			},
		},
	}

	sub = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				return (u.(Int) - v.(Int)).maybeBig()
			},
			bigIntType: func(c Context, u, v Value) Value {
				mustFit(c.Config(), u.(BigInt).BitLen()+1)
				mustFit(c.Config(), v.(BigInt).BitLen()+1)
				return binaryBigIntOp(u, (*big.Int).Sub, v)
			},
			bigRatType: func(c Context, u, v Value) Value {
				return binaryBigRatOp(u, (*big.Rat).Sub, v)
			},
			bigFloatType: func(c Context, u, v Value) Value {
				return binaryBigFloatOp(c, u, (*big.Float).Sub, v)
			},
		},
	}

	mul = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				return (u.(Int) * v.(Int)).maybeBig()
			},
			bigIntType: func(c Context, u, v Value) Value {
				mustFit(c.Config(), u.(BigInt).BitLen()+v.(BigInt).BitLen())
				return binaryBigIntOp(u, (*big.Int).Mul, v)
			},
			bigRatType: func(c Context, u, v Value) Value {
				return binaryBigRatOp(u, (*big.Rat).Mul, v)
			},
			bigFloatType: func(c Context, u, v Value) Value {
				return binaryBigFloatOp(c, u, (*big.Float).Mul, v)
			},
		},
	}

	quo = &binaryOp{ // Rational division.
		elementwise: true,
		whichType:   rationalType, // Use BigRats to avoid the analysis here.
		fn: [numType]binaryFn{
			bigRatType: func(c Context, u, v Value) Value {
				if v.(BigRat).Sign() == 0 {
					Errorf("division by zero")
				}
				return binaryBigRatOp(u, (*big.Rat).Quo, v) // True division.
			},
			bigFloatType: func(c Context, u, v Value) Value {
				return binaryBigFloatOp(c, u, (*big.Float).Quo, v)
			},
		},
	}

	idiv = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				if v.(Int) == 0 {
					Errorf("division by zero")
				}
				return u.(Int) / v.(Int)
			},
			bigIntType: func(c Context, u, v Value) Value {
				if v.(BigInt).Sign() == 0 {
					Errorf("division by zero")
				}
				return binaryBigIntOp(u, (*big.Int).Quo, v) // Go-like division.
			},
			bigRatType:   nil, // Not defined for rationals. Use div.
			bigFloatType: nil,
		},
	}

	imod = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				if v.(Int) == 0 {
					Errorf("modulo by zero")
				}
				return u.(Int) % v.(Int)
			},
			bigIntType: func(c Context, u, v Value) Value {
				if v.(BigInt).Sign() == 0 {
					Errorf("modulo by zero")
				}
				return binaryBigIntOp(u, (*big.Int).Rem, v) // Go-like modulo.
			},
			bigRatType:   nil, // Not defined for rationals. Use mod.
			bigFloatType: nil,
		},
	}

	div = &binaryOp{ // Euclidean integer division.
		elementwise: true,
		whichType:   divType, // Use BigInts to avoid the analysis here.
		fn: [numType]binaryFn{
			bigIntType: func(c Context, u, v Value) Value {
				if v.(BigInt).Sign() == 0 {
					Errorf("division by zero")
				}
				return binaryBigIntOp(u, (*big.Int).Div, v) // Euclidean division.
			},
			bigRatType:   nil, // Not defined for rationals. Use div.
			bigFloatType: nil,
		},
	}

	mod = &binaryOp{ // Euclidean integer modulus.
		elementwise: true,
		whichType:   divType, // Use BigInts to avoid the analysis here.
		fn: [numType]binaryFn{
			bigIntType: func(c Context, u, v Value) Value {
				if v.(BigInt).Sign() == 0 {
					Errorf("modulo by zero")
				}
				return binaryBigIntOp(u, (*big.Int).Mod, v) // Euclidan modulo.
			},
			bigRatType:   nil, // Not defined for rationals. Use mod.
			bigFloatType: nil,
		},
	}

	pow = &binaryOp{
		elementwise: true,
		whichType:   divType,
		fn: [numType]binaryFn{
			bigIntType: func(c Context, u, v Value) Value {
				switch v.(BigInt).Sign() {
				case 0:
					return one
				case -1:
					if u.(BigInt).Sign() == 0 {
						Errorf("negative exponent of zero")
					}
					v = Unary(c, "abs", v).toType(c.Config(), bigIntType)
					return Unary(c, "/", binaryBigIntOp(u, bigIntExpOp(c), v))
				}
				x := u.(BigInt).Int
				if x.Cmp(bigOne.Int) == 0 || x.Sign() == 0 {
					return u
				}
				return binaryBigIntOp(u, bigIntExpOp(c), v)
			},
			bigRatType: func(c Context, u, v Value) Value {
				// (n/d)**2 is n**2/d**2.
				rexp := v.(BigRat)
				positive := true
				switch rexp.Sign() {
				case 0:
					return one
				case -1:
					if u.(BigRat).Sign() == 0 {
						Errorf("negative exponent of zero")
					}
					positive = false
					rexp = Unary(c, "-", v).toType(c.Config(), bigRatType).(BigRat)
				}
				if !rexp.IsInt() {
					// Lift to float.
					return Binary(c, floatSelf(c, u), "**", floatSelf(c, v))
				}
				exp := rexp.Num()
				rat := u.(BigRat)
				num := new(big.Int).Set(rat.Num())
				den := new(big.Int).Set(rat.Denom())
				bigIntExp(c, num, num, exp)
				bigIntExp(c, den, den, exp)
				z := bigRatInt64(0)
				if positive {
					z.SetFrac(num, den)
				} else {
					z.SetFrac(den, num)
				}
				return z.shrink()
			},
			bigFloatType: func(c Context, u, v Value) Value { return power(c, u, v) },
		},
	}

	binaryLog = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType:      logBaseU,
			bigIntType:   logBaseU,
			bigRatType:   logBaseU,
			bigFloatType: logBaseU,
		},
	}

	bitAnd = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				return u.(Int) & v.(Int)
			},
			bigIntType: func(c Context, u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).And, v)
			},
		},
	}

	bitOr = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				return u.(Int) | v.(Int)
			},
			bigIntType: func(c Context, u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Or, v)
			},
		},
	}

	bitXor = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				return u.(Int) ^ v.(Int)
			},
			bigIntType: func(c Context, u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Xor, v)
			},
		},
	}

	lsh = &binaryOp{
		elementwise: true,
		whichType:   divType, // Shifts are like exp: let BigInt do the work.
		fn: [numType]binaryFn{
			bigIntType: func(c Context, u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				z := bigInt64(0)
				z.Lsh(i.Int, shiftCount(j))
				return z.shrink()
			},
			// TODO: lsh for bigfloat
		},
	}

	rsh = &binaryOp{
		elementwise: true,
		whichType:   divType, // Shifts are like exp: let BigInt do the work.
		fn: [numType]binaryFn{
			bigIntType: func(c Context, u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				z := bigInt64(0)
				z.Rsh(i.Int, shiftCount(j))
				return z.shrink()
			},
			// TODO: rsh for bigfloat
		},
	}

	eq = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				return toInt(u.(Int) == v.(Int))
			},
			charType: func(c Context, u, v Value) Value {
				return toInt(u.(Char) == v.(Char))
			},
			bigIntType: func(c Context, u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.Cmp(j.Int) == 0)
			},
			bigRatType: func(c Context, u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.Cmp(j.Rat) == 0)
			},
			bigFloatType: func(c Context, u, v Value) Value {
				i, j := u.(BigFloat), v.(BigFloat)
				return toInt(i.Cmp(j.Float) == 0)
			},
		},
	}

	ne = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				return toInt(u.(Int) != v.(Int))
			},
			charType: func(c Context, u, v Value) Value {
				return toInt(u.(Char) != v.(Char))
			},
			bigIntType: func(c Context, u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.Cmp(j.Int) != 0)
			},
			bigRatType: func(c Context, u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.Cmp(j.Rat) != 0)
			},
			bigFloatType: func(c Context, u, v Value) Value {
				i, j := u.(BigFloat), v.(BigFloat)
				return toInt(i.Cmp(j.Float) != 0)
			},
		},
	}

	lt = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				return toInt(u.(Int) < v.(Int))
			},
			charType: func(c Context, u, v Value) Value {
				return toInt(u.(Char) < v.(Char))
			},
			bigIntType: func(c Context, u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.Cmp(j.Int) < 0)
			},
			bigRatType: func(c Context, u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.Cmp(j.Rat) < 0)
			},
			bigFloatType: func(c Context, u, v Value) Value {
				i, j := u.(BigFloat), v.(BigFloat)
				return toInt(i.Cmp(j.Float) < 0)
			},
		},
	}

	le = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				return toInt(u.(Int) <= v.(Int))
			},
			charType: func(c Context, u, v Value) Value {
				return toInt(u.(Char) <= v.(Char))
			},
			bigIntType: func(c Context, u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.Cmp(j.Int) <= 0)
			},
			bigRatType: func(c Context, u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.Cmp(j.Rat) <= 0)
			},
			bigFloatType: func(c Context, u, v Value) Value {
				i, j := u.(BigFloat), v.(BigFloat)
				return toInt(i.Cmp(j.Float) <= 0)
			},
		},
	}

	gt = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				return toInt(u.(Int) > v.(Int))
			},
			charType: func(c Context, u, v Value) Value {
				return toInt(u.(Char) > v.(Char))
			},
			bigIntType: func(c Context, u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.Cmp(j.Int) > 0)
			},
			bigRatType: func(c Context, u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.Cmp(j.Rat) > 0)
			},
			bigFloatType: func(c Context, u, v Value) Value {
				i, j := u.(BigFloat), v.(BigFloat)
				return toInt(i.Cmp(j.Float) > 0)
			},
		},
	}

	ge = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				return toInt(u.(Int) >= v.(Int))
			},
			charType: func(c Context, u, v Value) Value {
				return toInt(u.(Char) >= v.(Char))
			},
			bigIntType: func(c Context, u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.Cmp(j.Int) >= 0)
			},
			bigRatType: func(c Context, u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.Cmp(j.Rat) >= 0)
			},
			bigFloatType: func(c Context, u, v Value) Value {
				i, j := u.(BigFloat), v.(BigFloat)
				return toInt(i.Cmp(j.Float) >= 0)
			},
		},
	}

	logicalAnd = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				return toInt(toBool(u) && toBool(v))
			},
			charType: func(c Context, u, v Value) Value {
				return toInt(toBool(u) && toBool(v))
			},
			bigIntType: func(c Context, u, v Value) Value {
				return toInt(toBool(u) && toBool(v))
			},
			bigRatType: func(c Context, u, v Value) Value {
				return toInt(toBool(u) && toBool(v))
			},
			bigFloatType: func(c Context, u, v Value) Value {
				return toInt(toBool(u) && toBool(v))
			},
		},
	}

	logicalOr = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				return toInt(toBool(u) || toBool(v))
			},
			charType: func(c Context, u, v Value) Value {
				return toInt(toBool(u) || toBool(v))
			},
			bigIntType: func(c Context, u, v Value) Value {
				return toInt(toBool(u) || toBool(v))
			},
			bigRatType: func(c Context, u, v Value) Value {
				return toInt(toBool(u) || toBool(v))
			},
			bigFloatType: func(c Context, u, v Value) Value {
				return toInt(toBool(u) || toBool(v))
			},
		},
	}

	logicalXor = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				return toInt(toBool(u) != toBool(v))
			},
			charType: func(c Context, u, v Value) Value {
				return toInt(toBool(u) != toBool(v))
			},
			bigIntType: func(c Context, u, v Value) Value {
				return toInt(toBool(u) != toBool(v))
			},
			bigRatType: func(c Context, u, v Value) Value {
				return toInt(toBool(u) != toBool(v))
			},
			bigFloatType: func(c Context, u, v Value) Value {
				return toInt(toBool(u) != toBool(v))
			},
		},
	}

	logicalNand = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				return toInt(!(toBool(u) && toBool(v)))
			},
			charType: func(c Context, u, v Value) Value {
				return toInt(!(toBool(u) && toBool(v)))
			},
			bigIntType: func(c Context, u, v Value) Value {
				return toInt(!(toBool(u) && toBool(v)))
			},
			bigRatType: func(c Context, u, v Value) Value {
				return toInt(!(toBool(u) && toBool(v)))
			},
			bigFloatType: func(c Context, u, v Value) Value {
				return toInt(!(toBool(u) && toBool(v)))
			},
		},
	}

	logicalNor = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				return toInt(!(toBool(u) || toBool(v)))
			},
			charType: func(c Context, u, v Value) Value {
				return toInt(!(toBool(u) || toBool(v)))
			},
			bigIntType: func(c Context, u, v Value) Value {
				return toInt(!(toBool(u) || toBool(v)))
			},
			bigRatType: func(c Context, u, v Value) Value {
				return toInt(!(toBool(u) || toBool(v)))
			},
			bigFloatType: func(c Context, u, v Value) Value {
				return toInt(!(toBool(u) || toBool(v)))
			},
		},
	}

	decode = &binaryOp{
		whichType: atLeastVectorType,
		fn: [numType]binaryFn{
			vectorType: func(c Context, u, v Value) Value {
				// A decode B is the result of polyomial B at x=A.
				// If A is a vector, the elements of A align with B.
				A, B := u.(Vector), v.(Vector)
				if len(A) == 0 || len(B) == 0 {
					return Int(0)
				}
				if len(A) == 1 || len(B) == 1 || len(A) == len(B) {
					result := Value(Int(0))
					prod := Value(Int(1))
					get := func(v Vector, i int) Value {
						if len(v) == 1 {
							return v[0]
						}
						return v[i]
					}
					n := len(A)
					if len(B) > n {
						n = len(B)
					}
					for i := n - 1; i >= 0; i-- {
						result = Binary(c, result, "+", Binary(c, prod, "*", get(B, i)))
						prod = Binary(c, prod, "*", get(A, i))
					}
					return result
				}
				if len(A) != len(B) {
					Errorf("decode of unequal lengths")
				}
				return nil
			},
		},
	}

	encode = &binaryOp{
		whichType: atLeastVectorType,
		fn: [numType]binaryFn{
			vectorType: func(c Context, u, v Value) Value {
				// A encode B is a matrix of len(A) rows and len(B) columns.
				// Each entry is the residue base A[i] of B[j].
				// Thus 2 encode 3 is just the low bit of 3, 2 2 encode 3 is the low 2 bits,
				// and 2 2 encode 1 2 3 has 3 columns encoding 1 2 3 downwards:
				// 0 1 1
				// 1 0 1
				// If they are negative the answers disagree with APL because
				// of how modulo arithmetic works.
				mod := func(b, a Value) Value {
					if z, ok := a.(Int); ok && z == 0 {
						return b
					}
					return Binary(c, b, "mod", a)
				}
				div := func(b, a Value) Value {
					if z, ok := a.(Int); ok && z == 0 {
						return b
					}
					return Binary(c, b, "div", a)
				}
				A, B := u.(Vector), v.(Vector)
				// Scalar.
				if len(A) == 1 && len(B) == 1 {
					return mod(B[0], A[0])
				}
				// Vector.
				if len(B) == 1 {
					// 2 2 2 2 encode 11 is 1 0 1 1.
					elems := make([]Value, len(A))
					b := B[0]
					for i := len(A) - 1; i >= 0; i-- {
						a := A[i]
						elems[i] = mod(b, a)
						b = div(b, a)
					}
					return NewVector(elems)
				}
				if len(A) == 1 {
					// 3 encode 1 2 3 4 is 1 2 0 1
					elems := make([]Value, len(B))
					a := A[0]
					for i := range B {
						b := B[i]
						elems[i] = mod(b, a)
						b = div(b, a)
					}
					return NewVector(elems)
				}
				// Matrix.
				// 2 2 encode 1 2 3 has 3 columns encoding 1 2 3 downwards:
				// 0 1 1
				// 1 0 1
				elems := make([]Value, len(A)*len(B))
				shape := []Value{Int(len(A)), Int(len(B))}
				for j := range B {
					b := B[j]
					for i := len(A) - 1; i >= 0; i-- {
						a := A[i]
						elems[j+i*len(B)] = mod(b, a)
						b = div(b, a)
					}
				}
				return NewMatrix(shape, elems)
			},
		},
	}

	in = &binaryOp{
		// A in B: Membership: 0 or 1 according to which elements of A present in B.
		whichType: atLeastVectorType,
		fn: [numType]binaryFn{
			vectorType: func(c Context, u, v Value) Value {
				return membership(c, u.(Vector), v.(Vector))
			},
			matrixType: func(c Context, u, v Value) Value {
				return membership(c, u.(Matrix).data, v.(Matrix).data)
			},
		},
	}

	index = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			vectorType: func(c Context, u, v Value) Value {
				// A[B]: The successive elements of A with indexes elements of B.
				A, B := u.(Vector), v.(Vector)
				values := make([]Value, len(B))
				origin := Int(c.Config().Origin())
				for i, b := range B {
					x, ok := b.(Int)
					if !ok {
						Errorf("index must be integer")
					}
					x -= origin
					if x < 0 || Int(len(A)) <= x {
						Errorf("index %d out of range", x+origin)
					}
					values[i] = A[x]
				}
				if len(values) == 1 {
					return values[0]
				}
				return NewVector(values).shrink()
			},
			matrixType: func(c Context, u, v Value) Value {
				// A[B]: The successive elements of A with indexes given by elements of B.
				A, mB := u.(Matrix), v.(Matrix)
				if len(mB.shape) != 1 {
					Errorf("bad index rank %d", len(mB.shape))
				}
				B := mB.data
				elemSize := Int(A.elemSize())
				values := make(Vector, 0, elemSize*Int(len(B)))
				origin := Int(c.Config().Origin())
				for _, b := range B {
					x, ok := b.(Int)
					if !ok {
						Errorf("index must be integer")
					}
					x -= origin
					if x < 0 || Int(A.shape[0].(Int)) <= x {
						Errorf("index %d out of range (shape %s)", x+origin, A.shape)
					}
					start := elemSize * x
					values = append(values, A.data[start:start+elemSize]...)
				}
				if len(B) == 1 {
					// Special considerations. The result might need type reduction.
					// TODO: Should this be Matrix.shrink?
					// TODO: In some cases, can get a scalar.
					// Is the result a vector?
					if len(A.shape) == 2 {
						return values
					}
					// Matrix of one less degree.
					newShape := make(Vector, len(A.shape)-1)
					copy(newShape, A.shape[1:])
					return NewMatrix(newShape, values)
				}
				newShape := make(Vector, len(A.shape))
				copy(newShape, A.shape)
				newShape[0] = Int(len(B))
				return NewMatrix(newShape, values)
			},
		},
	}

	binaryIota = &binaryOp{
		whichType: atLeastVectorType,
		fn: [numType]binaryFn{
			vectorType: func(c Context, u, v Value) Value {
				// A⍳B: The location (index) of B in A; 0 if not found. (APL does 1+⌈/⍳⍴A)
				A, B := u.(Vector), v.(Vector)
				indices := make([]Value, len(B))
				// TODO: This is n^2.
				origin := c.Config().Origin()
			Outer:
				for i, b := range B {
					for j, a := range A {
						if toBool(Binary(c, a, "==", b)) {
							indices[i] = Int(j + origin)
							continue Outer
						}
					}
					indices[i] = zero
				}
				return NewVector(indices)
			},
		},
	}

	min = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				if u.(Int) < v.(Int) {
					return u
				}
				return v
			},
			charType: func(c Context, u, v Value) Value {
				if u.(Char) < v.(Char) {
					return u
				}
				return v
			},
			bigIntType: func(c Context, u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				if i.Cmp(j.Int) < 0 {
					return i.shrink()
				}
				return j.shrink()
			},
			bigRatType: func(c Context, u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				if i.Cmp(j.Rat) < 0 {
					return i.shrink()
				}
				return j.shrink()
			},
			bigFloatType: func(c Context, u, v Value) Value {
				i, j := u.(BigFloat), v.(BigFloat)
				if i.Cmp(j.Float) < 0 {
					return i.shrink()
				}
				return j.shrink()
			},
		},
	}

	max = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(c Context, u, v Value) Value {
				if u.(Int) > v.(Int) {
					return u
				}
				return v
			},
			charType: func(c Context, u, v Value) Value {
				if u.(Char) > v.(Char) {
					return u
				}
				return v
			},
			bigIntType: func(c Context, u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				if i.Cmp(j.Int) > 0 {
					return u
				}
				return v
			},
			bigRatType: func(c Context, u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				if i.Cmp(j.Rat) > 0 {
					return i.shrink()
				}
				return j.shrink()
			},
			bigFloatType: func(c Context, u, v Value) Value {
				i, j := u.(BigFloat), v.(BigFloat)
				if i.Cmp(j.Float) > 0 {
					return i.shrink()
				}
				return j.shrink()
			},
		},
	}

	binaryRho = &binaryOp{
		whichType: atLeastVectorType,
		fn: [numType]binaryFn{
			vectorType: func(c Context, u, v Value) Value {
				return reshape(u.(Vector), v.(Vector))
			},
			matrixType: func(c Context, u, v Value) Value {
				// LHS must be a vector underneath.
				A, B := u.(Matrix), v.(Matrix)
				if len(A.shape) != 1 {
					Errorf("lhs of rho cannot be matrix")
				}
				return reshape(A.data, B.data)
			},
		},
	}

	binaryCatenate = &binaryOp{
		whichType: atLeastVectorType,
		fn: [numType]binaryFn{
			vectorType: func(c Context, u, v Value) Value {
				return append(u.(Vector), v.(Vector)...)
			},
			matrixType: func(c Context, u, v Value) Value {
				A := u.(Matrix)
				B := v.(Matrix)
				if len(A.shape) == 0 || len(B.shape) == 0 {
					Errorf("empty matrix for ,")
				}
				if len(A.shape) != len(B.shape)+1 || A.elemSize() != B.size() {
					Errorf("catenate rank mismatch: %s != %s", A.shape[1:], B.shape)
				}
				elemSize := A.elemSize()
				newShape := make(Vector, len(A.shape))
				copy(newShape, A.shape)
				newData := make(Vector, len(A.data), len(A.data)+elemSize)
				copy(newData, A.data)
				newData = append(newData, B.data...)
				newShape[0] = newShape[0].(Int) + 1
				return NewMatrix(newShape, newData)
			},
		},
	}

	take = &binaryOp{
		whichType: atLeastVectorType,
		fn: [numType]binaryFn{
			vectorType: func(c Context, u, v Value) Value {
				const bad = Error("bad count for take")
				i := v.(Vector)
				nv, ok := u.(Vector)
				if !ok || len(nv) != 1 {
					panic(bad)
				}
				n, ok := nv[0].(Int)
				if !ok {
					panic(bad)
				}
				len := Int(len(i))
				switch {
				case n < 0:
					if -n > len {
						panic(bad)
					}
					i = i[len+n : len]
				case n == 0:
					return NewVector(nil)
				case n > 0:
					if n > len {
						panic(bad)
					}
					i = i[0:n]
				}
				return i
			},
		},
	}

	drop = &binaryOp{
		whichType: atLeastVectorType,
		fn: [numType]binaryFn{
			vectorType: func(c Context, u, v Value) Value {
				const bad = Error("bad count for drop")
				i := v.(Vector)
				nv, ok := u.(Vector)
				if !ok || len(nv) != 1 {
					panic(bad)
				}
				n, ok := nv[0].(Int)
				if !ok {
					panic(bad)
				}
				len := Int(len(i))
				switch {
				case n < 0:
					if -n > len {
						panic(bad)
					}
					i = i[0 : len+n]
				case n == 0:
				case n > 0:
					if n > len {
						panic(bad)
					}
					i = i[n:]
				}
				return i
			},
		},
	}

	rot = &binaryOp{
		whichType: atLeastVectorType,
		fn: [numType]binaryFn{
			vectorType: func(c Context, u, v Value) Value {
				countVec := u.(Vector)
				count, ok := countVec[0].(Int)
				if !ok {
					Errorf("rot: count must be small integer")
				}
				return v.(Vector).rotate(int(count))
			},
			matrixType: func(c Context, u, v Value) Value {
				countMat := u.(Matrix)
				if len(countMat.shape) != 1 || len(countMat.data) != 1 {
					Errorf("rot: count must be small integer")
				}
				count, ok := countMat.data[0].(Int)
				if !ok {
					Errorf("rot: count must be small integer")
				}
				return v.(Matrix).rotate(int(count))
			},
		},
	}

	fill = &binaryOp{
		whichType: atLeastVectorType,
		fn: [numType]binaryFn{
			vectorType: func(c Context, u, v Value) Value {
				i := u.(Vector)
				j := v.(Vector)
				if len(i) == 0 {
					return NewVector(nil)
				}
				// All lhs values must be small integers.
				var count int64
				numLeft := 0
				for _, x := range i {
					y, ok := x.(Int)
					if !ok {
						Errorf("left operand of fill must be small integers")
					}
					switch {
					case y == 0:
						count++
					case y < 0:
						count -= int64(y)
					default:
						numLeft++
						count += int64(y)
					}
				}
				if numLeft != len(j) {
					Errorf("fill: count > 0 on left (%d) must equal length of right (%d)", numLeft, len(j))
				}
				if count > 1e8 {
					Errorf("fill: result too large: %d elements", count)
				}
				result := make([]Value, 0, count)
				jx := 0
				var zero Value
				if j.AllChars() {
					zero = Char(' ')
				} else {
					zero = Int(0)
				}
				for _, x := range i {
					y := x.(Int)
					switch {
					case y == 0:
						result = append(result, zero)
					case y < 0:
						for y = -y; y > 0; y-- {
							result = append(result, zero)
						}
					default:
						for ; y > 0; y-- {
							result = append(result, j[jx])
						}
						jx++
					}
				}
				return NewVector(result)
			},
		},
	}

	sel = &binaryOp{
		whichType: atLeastVectorType,
		fn: [numType]binaryFn{
			vectorType: func(c Context, u, v Value) Value {
				i := u.(Vector)
				j := v.(Vector)
				if len(i) == 0 {
					return NewVector(nil)
				}
				// All lhs values must be small integers.
				var count int64
				for _, x := range i {
					y, ok := x.(Int)
					if !ok {
						Errorf("left operand of sel must be small integers")
					}
					if y < 0 {
						count -= int64(y)
					} else {
						count += int64(y)
					}
				}
				if count > 1e8 {
					Errorf("sel: result too large: %d elements", count)
				}
				result := make([]Value, 0, count)
				add := func(howMany, what Value) {
					hm := int(howMany.(Int))
					if hm < 0 {
						hm = -hm
						what = Int(0)
					}
					for ; hm > 0; hm-- {
						result = append(result, what)
					}
				}
				if len(i) == 1 {
					for _, y := range j {
						add(i[0], y)
					}
				} else {
					if len(i) != len(j) {
						Errorf("sel: unequal lengths %d != %d", len(i), len(j))
					}
					for x, y := range j {
						add(i[x], y)
					}
				}
				return NewVector(result)
			},
		},
	}

	binaryOps = map[string]*binaryOp{
		"!=":     ne,
		"&":      bitAnd,
		"*":      mul,
		"**":     pow,
		"+":      add,
		",":      binaryCatenate,
		"-":      sub,
		"/":      quo, // Exact rational division.
		"<":      lt,
		"<<":     lsh,
		"<=":     le,
		"==":     eq,
		">":      gt,
		">=":     ge,
		">>":     rsh,
		"[]":     index,
		"^":      bitXor,
		"and":    logicalAnd,
		"decode": decode,
		"div":    div, // Euclidean integer division.
		"drop":   drop,
		"encode": encode,
		"fill":   fill,
		"idiv":   idiv, // Go-like truncating integer division.
		"imod":   imod, // Go-like integer moduls.
		"in":     in,
		"iota":   binaryIota,
		"log":    binaryLog,
		"max":    max,
		"min":    min,
		"mod":    mod, // Euclidean integer division.
		"nand":   logicalNand,
		"nor":    logicalNor,
		"or":     logicalOr,
		"rho":    binaryRho,
		"rot":    rot,
		"sel":    sel,
		"take":   take,
		"xor":    logicalXor,
		"|":      bitOr,
	}
}
