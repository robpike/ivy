// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"math"
	"math/big"
	"sort"
)

// Binary operators.

// To avoid initialization cycles when we refer to the ops from inside
// themselves, we use an init function to initialize the ops.

// noPromoteType leaves the types as they are.
func noPromoteType(t1, t2 valueType) (valueType, valueType) {
	return t1, t2
}

// binaryArithType returns the maximum of the two types,
// so the smaller value is appropriately up-converted.
func binaryArithType(t1, t2 valueType) (valueType, valueType) {
	if t1 > t2 {
		return t1, t1
	}
	return t2, t2
}

// divType is like binaryArithType but never returns smaller than BigInt,
// because the only implementation of exponentiation we have is in big.Int.
func divType(t1, t2 valueType) (valueType, valueType) {
	if t1 == intType {
		t1 = bigIntType
	}
	return binaryArithType(t1, t2)
}

// rationalType promotes scalars to rationals so we can do rational division.
func rationalType(t1, t2 valueType) (valueType, valueType) {
	if t1 < bigRatType {
		t1 = bigRatType
	}
	return binaryArithType(t1, t2)
}

// atLeastVectorType promotes both arguments to at least vectors.
func atLeastVectorType(t1, t2 valueType) (valueType, valueType) {
	if t1 < matrixType && t2 < matrixType {
		return vectorType, vectorType
	}
	return matrixType, matrixType
}

// vectorAndMatrixType promotes the left arg to vector and the right arg to matrix.
func vectorAndMatrixType(t1, t2 valueType) (valueType, valueType) {
	return vectorType, matrixType
}

// vectorAndAtLeastVectorType promotes the left arg to vector
// and the right arg to at least vector.
func vectorAndAtLeastVectorType(t1, t2 valueType) (valueType, valueType) {
	if t2 < vectorType {
		t2 = vectorType
	}
	return vectorType, t2
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
	if j.Cmp(bigIntOne.Int) == 0 || j.Sign() == 0 {
		return i.Set(j)
	}
	// -1ⁿ is just parity.
	if j.Cmp(bigIntMinusOne.Int) == 0 {
		var x big.Int
		if x.And(k, bigIntOne.Int).Int64() == 0 {
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
	// "2" is just shift. math/big should do this, really.
	if j.Cmp(bigIntTwo.Int) == 0 && exp >= 0 {
		return i.Lsh(big.NewInt(1), uint(exp))
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
	case Complex:
		return !isZero(t.real) || !isZero(t.imag)
	}
	Errorf("cannot convert %T to bool", t)
	panic("not reached")
}

// andBool is like toBool but handles vectors by and'ing the values together.
// The results are known to be Ints, as they come from comparison operations.
func andBool(t Value) bool {
	if v, ok := t.(Vector); ok {
		for _, x := range v {
			if x == Int(0) {
				return false
			}
		}
		return true
	}
	return t.(Int) == one
}

var BinaryOps = make(map[string]BinaryOp)

func init() {
	var ops = []*binaryOp{
		{
			name:        "j",
			elementwise: true,
			whichType:   binaryArithType,
			fn: [numType]binaryFn{
				intType: func(c Context, u, v Value) Value {
					return newComplex(u, v)
				},
				bigIntType: func(c Context, u, v Value) Value {
					return newComplex(u, v)
				},
				bigRatType: func(c Context, u, v Value) Value {
					return newComplex(u, v)
				},
				bigFloatType: func(c Context, u, v Value) Value {
					return newComplex(u, v)
				},
			},
		},

		{
			name:        "+",
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
				complexType: func(c Context, u, v Value) Value {
					return u.(Complex).add(c, v.(Complex))
				},
			},
		},

		{
			name:        "-",
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
				complexType: func(c Context, u, v Value) Value {
					return u.(Complex).sub(c, v.(Complex))
				},
			},
		},

		{
			name:        "*",
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
				complexType: func(c Context, u, v Value) Value {
					return u.(Complex).mul(c, v.(Complex))
				},
			},
		},

		{ // Rational division.
			name:        "/",
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
				complexType: func(c Context, u, v Value) Value {
					return u.(Complex).div(c, v.(Complex))
				},
			},
		},

		{
			name:        "idiv", // Go integer division.
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
				bigRatType:   nil, // Not defined for rationals.
				bigFloatType: nil,
				complexType:  nil,
			},
		},

		{
			name:        "imod",
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
				complexType:  nil,
			},
		},

		{ // Euclidean integer division.
			name:        "div",
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
				complexType:  nil,
			},
		},

		{ // Euclidean integer modulus.
			name:        "mod",
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
				complexType:  nil,
			},
		},

		{
			name:        "**",
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
						if isNegative(v) {
							// Need the absolute value.
							v = BigInt{big.NewInt(0).Neg(v.(BigInt).Int)}
						}
						return c.EvalUnary("/", binaryBigIntOp(u, bigIntExpOp(c), v))
					}
					x := u.(BigInt).Int
					if x.Cmp(bigIntOne.Int) == 0 || x.Sign() == 0 {
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
						rexp = c.EvalUnary("-", v).toType("**", c.Config(), bigRatType).(BigRat)
					}
					if !rexp.IsInt() {
						// Lift to float.
						return c.EvalBinary(floatSelf(c, u), "**", floatSelf(c, v))
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
				complexType:  func(c Context, u, v Value) Value { return power(c, u, v) },
			},
		},

		{
			name:        "log",
			elementwise: true,
			whichType:   binaryArithType,
			fn: [numType]binaryFn{
				intType:      logBaseU,
				bigIntType:   logBaseU,
				bigRatType:   logBaseU,
				bigFloatType: logBaseU,
				complexType:  logBaseU,
			},
		},

		{
			name:        "!",
			elementwise: true,
			whichType:   binaryArithType,
			fn: [numType]binaryFn{
				intType: func(c Context, u, v Value) Value {
					a := int64(u.(Int))
					b := int64(v.(Int))
					if a == 0 || b == 0 || a == b {
						return one
					}
					if a < 0 || b < 0 || a > b {
						return zero
					}
					aFac := factorial(a)
					bFac := factorial(b)
					bMinusAFac := factorial(b - a)
					bFac.Div(bFac, aFac)
					bFac.Div(bFac, bMinusAFac)
					return BigInt{bFac}.shrink()
				},
			},
		},

		{
			name:        "&",
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
		},

		{
			name:        "|",
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
		},

		{
			name:        "^",
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
		},

		{
			name:        "<<",
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
		},

		{
			name:        ">>",
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
		},

		{
			name:        "==",
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
				complexType: func(c Context, u, v Value) Value {
					i, j := u.(Complex), v.(Complex)
					if c.EvalBinary(i.real, "==", j.real) == Int(0) {
						return Int(0)
					}
					return c.EvalBinary(i.imag, "==", j.imag)
				},
			},
		},

		{
			name:        "!=",
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
				complexType: func(c Context, u, v Value) Value {
					i, j := u.(Complex), v.(Complex)
					if c.EvalBinary(i.real, "!=", j.real) == Int(1) {
						return Int(1)
					}
					return c.EvalBinary(i.imag, "!=", j.imag)
				},
			},
		},

		{
			name:        "<",
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
		},

		{
			name:        "<=",
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
		},

		{
			name:        ">",
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
		},

		{
			name:        ">=",
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
		},

		{
			name:        "and",
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
				complexType: func(c Context, u, v Value) Value {
					return toInt(toBool(u) && toBool(v))
				},
			},
		},

		{
			name:        "or",
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
				complexType: func(c Context, u, v Value) Value {
					return toInt(toBool(u) || toBool(v))
				},
			},
		},

		{
			name:        "xor",
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
				complexType: func(c Context, u, v Value) Value {
					return toInt(toBool(u) != toBool(v))
				},
			},
		},

		{
			name:        "nand",
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
				complexType: func(c Context, u, v Value) Value {
					return toInt(!(toBool(u) && toBool(v)))
				},
			},
		},

		{
			name:        "nor",
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
				complexType: func(c Context, u, v Value) Value {
					return toInt(!(toBool(u) || toBool(v)))
				},
			},
		},

		{
			name:        "?",
			elementwise: false,
			whichType:   binaryArithType,
			fn: [numType]binaryFn{
				intType: func(c Context, u, v Value) Value {
					A := u.(Int)
					B := v.(Int)
					if uint64(A) > maxInt || uint64(B) > maxInt {
						Errorf("negative or too-large operand in %d?%d", A, B)
					}
					if A > B {
						Errorf("left operand larger than right in %d?%d", A, B)
					}
					ints := c.Config().Random().Perm(int(B))
					origin := c.Config().Origin()
					res := make([]Value, A)
					for i := range res {
						res[i] = Int(ints[i] + origin)
					}
					return NewVector(res)
				},
			},
		},

		{
			name:      "decode",
			whichType: vectorAndAtLeastVectorType,
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
							result = c.EvalBinary(result, "+", c.EvalBinary(prod, "*", get(B, i)))
							prod = c.EvalBinary(prod, "*", get(A, i))
						}
						return result
					}
					if len(A) != len(B) {
						Errorf("decode of unequal lengths")
					}
					return nil
				},
				matrixType: func(c Context, u, v Value) Value {
					A, B := u.(Vector), v.(*Matrix)
					if len(A) != 1 && B.shape[0] != 1 && len(A) != B.shape[0] {
						Errorf("decode of length %d and shape %s", len(A), NewIntVector(B.shape))
					}
					shape := B.shape[1:]
					elems := make([]Value, len(B.data)/B.shape[0])
					get := func(v Vector, i int) Value {
						if len(v) == 1 {
							return v[0]
						}
						return v[i]
					}
					n := len(A)
					if B.shape[0] > n {
						n = B.shape[0]
					}
					pfor(true, n, len(elems), func(lo, hi int) {
						for j := lo; j < hi; j++ {
							result := Value(Int(0))
							prod := Value(Int(1))
							for i := n - 1; i >= 0; i-- {
								Bslice := B.data
								if B.shape[0] > 1 {
									Bslice = B.data[i*len(elems) : (i+1)*len(elems)]
								}
								result = c.EvalBinary(result, "+", c.EvalBinary(prod, "*", Bslice[j]))
								prod = c.EvalBinary(prod, "*", get(A, i))
							}
							elems[j] = result
						}
					})
					if len(shape) == 1 {
						return NewVector(elems)
					}
					return NewMatrix(shape, elems)
				},
			},
		},

		{
			name:      "encode",
			whichType: vectorAndAtLeastVectorType,
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
					const op = "encode"
					A, B := u.(Vector), v.(Vector)
					// Scalar.
					if len(A) == 1 && len(B) == 1 {
						return emod(op, c, B[0], A[0])
					}
					// Vector.
					if len(B) == 1 {
						// 2 2 2 2 encode 11 is 1 0 1 1.
						elems := make([]Value, len(A))
						b := B[0]
						for i := len(A) - 1; i >= 0; i-- {
							a := A[i]
							elems[i] = emod(op, c, b, a)
							b = ediv(op, c, b, a)
						}
						return NewVector(elems)
					}
					if len(A) == 1 {
						// 3 encode 1 2 3 4 is 1 2 0 1
						elems := make([]Value, len(B))
						a := A[0]
						for i := range B {
							elems[i] = emod(op, c, B[i], a)
						}
						return NewVector(elems)
					}
					// Matrix.
					// 2 2 encode 1 2 3 has 3 columns encoding 1 2 3 downwards:
					// 0 1 1
					// 1 0 1
					elems := make([]Value, len(A)*len(B))
					shape := []int{len(A), len(B)}
					pfor(true, len(A), len(B), func(lo, hi int) {
						for j := lo; j < hi; j++ {
							b := B[j]
							for i := len(A) - 1; i >= 0; i-- {
								a := A[i]
								elems[j+i*len(B)] = emod(op, c, b, a)
								b = ediv(op, c, b, a)
							}
						}
					})
					return NewMatrix(shape, elems)
				},
				matrixType: func(c Context, u, v Value) Value {
					A, B := u.(Vector), v.(*Matrix)
					elems := make([]Value, len(A)*len(B.data))
					shape := append([]int{len(A)}, B.Shape()...)
					const op = "encode"
					pfor(true, len(A), len(B.data), func(lo, hi int) {
						for j := lo; j < hi; j++ {
							b := B.data[j]
							for i := len(A) - 1; i >= 0; i-- {
								a := A[i]
								elems[j+i*len(B.data)] = emod(op, c, b, a)
								b = ediv(op, c, b, a)
							}
						}
					})
					return NewMatrix(shape, elems)
				},
			},
		},

		{
			name: "in",
			// A in B: Membership: 0 or 1 according to which elements of A present in B.
			whichType: atLeastVectorType,
			fn: [numType]binaryFn{
				vectorType: func(c Context, u, v Value) Value {
					return NewVector(membership(c, u.(Vector), v.(Vector))).shrink()
				},
				matrixType: func(c Context, u, v Value) Value {
					m := u.(*Matrix)
					data := membership(c, m.data, v.(*Matrix).data)
					if m.Rank() <= 1 {
						return NewVector(data).shrink()
					}
					return NewMatrix(m.shape, data)
				},
			},
		},

		{
			name:      "iota",
			whichType: atLeastVectorType,
			fn: [numType]binaryFn{
				vectorType: func(c Context, u, v Value) Value {
					// A⍳B: The location (index) of B in A; 0 if not found. (APL does 1+⌈/⍳⍴A)
					A, B := u.(Vector), v.(Vector)
					type indexed struct {
						v     Value
						index int
					}
					origin := c.Config().Origin()
					sortedA := make([]indexed, len(A))
					for i, a := range A {
						sortedA[i] = indexed{a, i + origin}
					}
					sort.Slice(sortedA, func(i, j int) bool {
						return c.EvalBinary(sortedA[i].v, "<", sortedA[j].v) == Int(1)
					})
					indices := make([]Value, len(B))
					work := 2 * (1 + int(math.Log2(float64(len(A)))))
					pfor(true, work, len(B), func(lo, hi int) {
						for i := lo; i < hi; i++ {
							b := B[i]
							indices[i] = Int(origin - 1)
							pos := sort.Search(len(sortedA), func(j int) bool {
								return c.EvalBinary(sortedA[j].v, ">=", b) == Int(1)
							})
							if pos < len(sortedA) && c.EvalBinary(sortedA[pos].v, "==", b) == Int(1) {
								indices[i] = Int(sortedA[pos].index)
							}
						}
					})
					return NewVector(indices)
				},
				matrixType: func(c Context, u, v Value) Value {
					A, B := u.(*Matrix), v.(*Matrix)
					origin := c.Config().Origin()
					if A.Rank()-1 > B.Rank() || !sameShape(A.shape[1:], B.shape[B.Rank()-(A.Rank()-1):]) {
						Errorf("iota: mismatched shapes %s and %s", NewIntVector(A.shape), NewIntVector(B.shape))
					}
					// TODO: This is n^2. Use an algorithm similar to the Vector case, or perhaps use hashing.
					// However, one of the n's is the dimension of a matrix, so it is likely to be small.
					shape := B.shape[:B.Rank()-(A.Rank()-1)]
					if len(shape) == 0 {
						shape = []int{1}
					}
					n := len(A.data) / A.shape[0] // elements in each comparison
					indices := make([]Value, len(B.data)/n)
					pfor(true, n, len(B.data)/n, func(lo, hi int) {
						for i := lo; i < hi; i++ {
							indices[i] = Int(origin - 1)
							for j := 0; j < len(A.data); j += n {
								if andBool(c.EvalBinary(A.data[j:j+n], "==", B.data[i*n:(i+1)*n])) {
									indices[i] = Int(j/n + origin)
									break
								}
							}
						}
					})
					if len(shape) == 1 {
						return NewVector(indices)
					}
					return NewMatrix(shape, indices)
				},
			},
		},

		{
			name:        "min",
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
		},

		{
			name:        "max",
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
		},

		{
			name:      "rho",
			whichType: atLeastVectorType,
			fn: [numType]binaryFn{
				vectorType: func(c Context, u, v Value) Value {
					return reshape(u.(Vector), v.(Vector))
				},
				matrixType: func(c Context, u, v Value) Value {
					// LHS must be a vector underneath.
					A, B := u.(*Matrix), v.(*Matrix)
					if A.Rank() != 1 {
						Errorf("lhs of rho cannot be matrix")
					}
					return reshape(A.data, B.data)
				},
			},
		},

		{
			name:      ",",
			whichType: atLeastVectorType,
			fn: [numType]binaryFn{
				vectorType: func(c Context, u, v Value) Value {
					uu := u.(Vector)
					uu = uu[:len(uu):len(uu)]
					return append(uu, v.(Vector)...)
				},
				matrixType: func(c Context, u, v Value) Value {
					return u.(*Matrix).catenate(v.(*Matrix))
				},
			},
		},

		{
			name:      "take",
			whichType: vectorAndAtLeastVectorType,
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
						i = i[len+n : len : len]
					case n == 0:
						return NewVector(nil)
					case n > 0:
						if n > len {
							panic(bad)
						}
						i = i[0:n:n]
					}
					return i
				},
				matrixType: func(c Context, u, v Value) Value {
					return v.(*Matrix).take(c, u.(Vector))
				},
			},
		},

		{
			name:      "drop",
			whichType: vectorAndAtLeastVectorType,
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
				matrixType: func(c Context, u, v Value) Value {
					return v.(*Matrix).drop(c, u.(Vector))
				},
			},
		},

		{
			name:      "rot",
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
					countMat := u.(*Matrix)
					if countMat.Rank() != 1 || len(countMat.data) != 1 {
						Errorf("rot: count must be small integer")
					}
					count, ok := countMat.data[0].(Int)
					if !ok {
						Errorf("rot: count must be small integer")
					}
					return v.(*Matrix).rotate(int(count))
				},
			},
		},

		{
			name:      "flip",
			whichType: atLeastVectorType,
			fn: [numType]binaryFn{
				vectorType: func(c Context, u, v Value) Value {
					countVec := u.(Vector)
					if len(countVec) != 1 {
						Errorf("flip: count must be small integer")
					}
					count, ok := countVec[0].(Int)
					if !ok {
						Errorf("flip: count must be small integer")
					}
					return v.(Vector).rotate(int(count))
				},
				matrixType: func(c Context, u, v Value) Value {
					countMat := u.(*Matrix)
					if countMat.Rank() != 1 || len(countMat.data) != 1 {
						Errorf("flip: count must be small integer")
					}
					count, ok := countMat.data[0].(Int)
					if !ok {
						Errorf("flip: count must be small integer")
					}
					return v.(*Matrix).vrotate(int(count))
				},
			},
		},

		{
			name:      "fill",
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
							Errorf("fill: left operand must be small integers")
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
		},

		{
			name:      "sel",
			whichType: vectorAndAtLeastVectorType,
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
							Errorf("sel: left operand must be small integers")
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
				matrixType: func(c Context, u, v Value) Value {
					return v.(*Matrix).sel(c, u.(Vector))
				},
			},
		},

		{
			name:      "transp",
			whichType: vectorAndMatrixType,
			fn: [numType]binaryFn{
				matrixType: func(c Context, u, v Value) Value {
					m := v.(*Matrix).binaryTranspose(c, u.(Vector))
					if m.Rank() <= 1 {
						return m.Data()
					}
					return m
				},
			},
		},

		{
			// Special case, handled in EvalBinary: don't modify types.
			name:        "text",
			elementwise: true,
			whichType:   nil,
			fn: [numType]binaryFn{
				0: fmtText,
			},
		},
	}

	for _, op := range ops {
		BinaryOps[op.name] = op
	}
}
