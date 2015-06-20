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

func binaryBigFloatOp(u Value, op func(*big.Float, *big.Float, *big.Float) *big.Float, v Value) Value {
	i, j := u.(BigFloat), v.(BigFloat)
	z := bigFloatInt64(0)
	op(z.Float, i.Float, j.Float)
	return z.shrink()
}

// bigIntExp is the "op" for exp on *big.Int. Different signature for Exp means we can't use *big.Exp directly.
func bigIntExp(i, j, k *big.Int) *big.Int {
	if k.Cmp(bigBillion.Int) > 0 && j.Cmp(bigOne.Int) != 0 {
		Errorf("%s**%s: exponent too large", j, k) // Can be crazy expensive.
	}
	i.Exp(j, k, nil)
	return i
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
	index                             *binaryOp
	logicalAnd, logicalOr, logicalXor *binaryOp
	logicalNand, logicalNor           *binaryOp
	binaryIota, binaryRho             *binaryOp
	binaryCatenate                    *binaryOp
	take, drop                        *binaryOp
	min, max                          *binaryOp
	binaryOps                         map[string]*binaryOp
)

var (
	zero        = Int(0)
	one         = Int(1)
	minusOne    = Int(-1)
	bigZero     = bigInt64(0)
	bigOne      = bigInt64(1)
	bigMinusOne = bigInt64(-1)
	bigBillion  = bigInt64(1e9)
)

func init() {
	add = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return (u.(Int) + v.(Int)).maybeBig()
			},
			bigIntType: func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Add, v)
			},
			bigRatType: func(u, v Value) Value {
				return binaryBigRatOp(u, (*big.Rat).Add, v)
			},
			bigFloatType: func(u, v Value) Value {
				return binaryBigFloatOp(u, (*big.Float).Add, v)
			},
		},
	}

	sub = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return (u.(Int) - v.(Int)).maybeBig()
			},
			bigIntType: func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Sub, v)
			},
			bigRatType: func(u, v Value) Value {
				return binaryBigRatOp(u, (*big.Rat).Sub, v)
			},
			bigFloatType: func(u, v Value) Value {
				return binaryBigFloatOp(u, (*big.Float).Sub, v)
			},
		},
	}

	mul = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return (u.(Int) * v.(Int)).maybeBig()
			},
			bigIntType: func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Mul, v)
			},
			bigRatType: func(u, v Value) Value {
				return binaryBigRatOp(u, (*big.Rat).Mul, v)
			},
			bigFloatType: func(u, v Value) Value {
				return binaryBigFloatOp(u, (*big.Float).Mul, v)
			},
		},
	}

	quo = &binaryOp{ // Rational division.
		elementwise: true,
		whichType:   rationalType, // Use BigRats to avoid the analysis here.
		fn: [numType]binaryFn{
			bigRatType: func(u, v Value) Value {
				if v.(BigRat).Sign() == 0 {
					Errorf("division by zero")
				}
				return binaryBigRatOp(u, (*big.Rat).Quo, v) // True division.
			},
			bigFloatType: func(u, v Value) Value {
				return binaryBigFloatOp(u, (*big.Float).Quo, v)
			},
		},
	}

	idiv = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				if v.(Int) == 0 {
					Errorf("division by zero")
				}
				return u.(Int) / v.(Int)
			},
			bigIntType: func(u, v Value) Value {
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
			intType: func(u, v Value) Value {
				if v.(Int) == 0 {
					Errorf("modulo by zero")
				}
				return u.(Int) % v.(Int)
			},
			bigIntType: func(u, v Value) Value {
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
			bigIntType: func(u, v Value) Value {
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
			bigIntType: func(u, v Value) Value {
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
			bigIntType: func(u, v Value) Value {
				switch v.(BigInt).Sign() {
				case 0:
					return one
				case -1:
					if u.(BigInt).Sign() == 0 {
						Errorf("negative exponent of zero")
					}
					v = Unary("abs", v).toType(bigIntType)
					return Unary("/", binaryBigIntOp(u, bigIntExp, v))
				}
				x := u.(BigInt).Int
				if x.Cmp(bigOne.Int) == 0 || x.Sign() == 0 {
					return u
				}
				return binaryBigIntOp(u, bigIntExp, v)
			},
			bigRatType: func(u, v Value) Value {
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
					rexp = Unary("-", v).toType(bigRatType).(BigRat)
				}
				if !rexp.IsInt() {
					// Lift to float.
					return Binary(floatSelf(u), "**", floatSelf(v))
				}
				exp := rexp.Num()
				rat := u.(BigRat)
				num := new(big.Int).Set(rat.Num())
				den := new(big.Int).Set(rat.Denom())
				num.Exp(num, exp, nil)
				den.Exp(den, exp, nil)
				z := bigRatInt64(0)
				if positive {
					z.SetFrac(num, den)
				} else {
					z.SetFrac(den, num)
				}
				return z.shrink()
			},
			bigFloatType: func(u, v Value) Value { return power(u, v) },
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
			intType: func(u, v Value) Value {
				return u.(Int) & v.(Int)
			},
			bigIntType: func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).And, v)
			},
		},
	}

	bitOr = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return u.(Int) | v.(Int)
			},
			bigIntType: func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Or, v)
			},
		},
	}

	bitXor = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return u.(Int) ^ v.(Int)
			},
			bigIntType: func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Xor, v)
			},
		},
	}

	lsh = &binaryOp{
		elementwise: true,
		whichType:   divType, // Shifts are like exp: let BigInt do the work.
		fn: [numType]binaryFn{
			bigIntType: func(u, v Value) Value {
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
			bigIntType: func(u, v Value) Value {
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
			intType: func(u, v Value) Value {
				return toInt(u.(Int) == v.(Int))
			},
			charType: func(u, v Value) Value {
				return toInt(u.(Char) == v.(Char))
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.Cmp(j.Int) == 0)
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.Cmp(j.Rat) == 0)
			},
			bigFloatType: func(u, v Value) Value {
				i, j := u.(BigFloat), v.(BigFloat)
				return toInt(i.Cmp(j.Float) == 0)
			},
		},
	}

	ne = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(u.(Int) != v.(Int))
			},
			charType: func(u, v Value) Value {
				return toInt(u.(Char) != v.(Char))
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.Cmp(j.Int) != 0)
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.Cmp(j.Rat) != 0)
			},
			bigFloatType: func(u, v Value) Value {
				i, j := u.(BigFloat), v.(BigFloat)
				return toInt(i.Cmp(j.Float) != 0)
			},
		},
	}

	lt = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(u.(Int) < v.(Int))
			},
			charType: func(u, v Value) Value {
				return toInt(u.(Char) < v.(Char))
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.Cmp(j.Int) < 0)
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.Cmp(j.Rat) < 0)
			},
			bigFloatType: func(u, v Value) Value {
				i, j := u.(BigFloat), v.(BigFloat)
				return toInt(i.Cmp(j.Float) < 0)
			},
		},
	}

	le = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(u.(Int) <= v.(Int))
			},
			charType: func(u, v Value) Value {
				return toInt(u.(Char) <= v.(Char))
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.Cmp(j.Int) <= 0)
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.Cmp(j.Rat) <= 0)
			},
			bigFloatType: func(u, v Value) Value {
				i, j := u.(BigFloat), v.(BigFloat)
				return toInt(i.Cmp(j.Float) <= 0)
			},
		},
	}

	gt = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(u.(Int) > v.(Int))
			},
			charType: func(u, v Value) Value {
				return toInt(u.(Char) > v.(Char))
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.Cmp(j.Int) > 0)
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.Cmp(j.Rat) > 0)
			},
			bigFloatType: func(u, v Value) Value {
				i, j := u.(BigFloat), v.(BigFloat)
				return toInt(i.Cmp(j.Float) > 0)
			},
		},
	}

	ge = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(u.(Int) >= v.(Int))
			},
			charType: func(u, v Value) Value {
				return toInt(u.(Char) >= v.(Char))
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.Cmp(j.Int) >= 0)
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.Cmp(j.Rat) >= 0)
			},
			bigFloatType: func(u, v Value) Value {
				i, j := u.(BigFloat), v.(BigFloat)
				return toInt(i.Cmp(j.Float) >= 0)
			},
		},
	}

	logicalAnd = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(toBool(u) && toBool(v))
			},
			charType: func(u, v Value) Value {
				return toInt(toBool(u) && toBool(v))
			},
			bigIntType: func(u, v Value) Value {
				return toInt(toBool(u) && toBool(v))
			},
			bigRatType: func(u, v Value) Value {
				return toInt(toBool(u) && toBool(v))
			},
			bigFloatType: func(u, v Value) Value {
				return toInt(toBool(u) && toBool(v))
			},
		},
	}

	logicalOr = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(toBool(u) || toBool(v))
			},
			charType: func(u, v Value) Value {
				return toInt(toBool(u) || toBool(v))
			},
			bigIntType: func(u, v Value) Value {
				return toInt(toBool(u) || toBool(v))
			},
			bigRatType: func(u, v Value) Value {
				return toInt(toBool(u) || toBool(v))
			},
			bigFloatType: func(u, v Value) Value {
				return toInt(toBool(u) || toBool(v))
			},
		},
	}

	logicalXor = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(toBool(u) != toBool(v))
			},
			charType: func(u, v Value) Value {
				return toInt(toBool(u) != toBool(v))
			},
			bigIntType: func(u, v Value) Value {
				return toInt(toBool(u) != toBool(v))
			},
			bigRatType: func(u, v Value) Value {
				return toInt(toBool(u) != toBool(v))
			},
			bigFloatType: func(u, v Value) Value {
				return toInt(toBool(u) != toBool(v))
			},
		},
	}

	logicalNand = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(!(toBool(u) && toBool(v)))
			},
			charType: func(u, v Value) Value {
				return toInt(!(toBool(u) && toBool(v)))
			},
			bigIntType: func(u, v Value) Value {
				return toInt(!(toBool(u) && toBool(v)))
			},
			bigRatType: func(u, v Value) Value {
				return toInt(!(toBool(u) && toBool(v)))
			},
			bigFloatType: func(u, v Value) Value {
				return toInt(!(toBool(u) && toBool(v)))
			},
		},
	}

	logicalNor = &binaryOp{
		elementwise: true,
		whichType:   binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(!(toBool(u) || toBool(v)))
			},
			charType: func(u, v Value) Value {
				return toInt(!(toBool(u) || toBool(v)))
			},
			bigIntType: func(u, v Value) Value {
				return toInt(!(toBool(u) || toBool(v)))
			},
			bigRatType: func(u, v Value) Value {
				return toInt(!(toBool(u) || toBool(v)))
			},
			bigFloatType: func(u, v Value) Value {
				return toInt(!(toBool(u) || toBool(v)))
			},
		},
	}

	index = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			vectorType: func(u, v Value) Value {
				// A[B]: The successive elements of A with indexes elements of B.
				A, B := u.(Vector), v.(Vector)
				values := make([]Value, len(B))
				origin := Int(conf.Origin())
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
				return NewVector(values)
			},
			matrixType: func(u, v Value) Value {
				// A[B]: The successive elements of A with indexes given by elements of B.
				A, mB := u.(Matrix), v.(Matrix)
				if len(mB.shape) != 1 {
					Errorf("bad index rank %d", len(mB.shape))
				}
				B := mB.data
				elemSize := Int(A.elemSize())
				values := make(Vector, 0, elemSize*Int(len(B)))
				origin := Int(conf.Origin())
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
					return newMatrix(newShape, values)
				}
				newShape := make(Vector, len(A.shape))
				copy(newShape, A.shape)
				newShape[0] = Int(len(B))
				return newMatrix(newShape, values)
			},
		},
	}

	binaryIota = &binaryOp{
		whichType: atLeastVectorType,
		fn: [numType]binaryFn{
			vectorType: func(u, v Value) Value {
				// A⍳B: The location (index) of B in A; 0 if not found. (APL does 1+⌈/⍳⍴A)
				A, B := u.(Vector), v.(Vector)
				indices := make([]Value, len(B))
				// TODO: This is n^2.
			Outer:
				for i, b := range B {
					for j, a := range A {
						if toBool(Binary(a, "==", b)) {
							indices[i] = Int(j + conf.Origin())
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
			intType: func(u, v Value) Value {
				if u.(Int) < v.(Int) {
					return u
				}
				return v
			},
			charType: func(u, v Value) Value {
				if u.(Char) < v.(Char) {
					return u
				}
				return v
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				if i.Cmp(j.Int) < 0 {
					return i.shrink()
				}
				return j.shrink()
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				if i.Cmp(j.Rat) < 0 {
					return i.shrink()
				}
				return j.shrink()
			},
			bigFloatType: func(u, v Value) Value {
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
			intType: func(u, v Value) Value {
				if u.(Int) > v.(Int) {
					return u
				}
				return v
			},
			charType: func(u, v Value) Value {
				if u.(Char) > v.(Char) {
					return u
				}
				return v
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				if i.Cmp(j.Int) > 0 {
					return u
				}
				return v
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				if i.Cmp(j.Rat) > 0 {
					return i.shrink()
				}
				return j.shrink()
			},
			bigFloatType: func(u, v Value) Value {
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
			vectorType: func(u, v Value) Value {
				return reshape(u.(Vector), v.(Vector))
			},
			matrixType: func(u, v Value) Value {
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
			vectorType: func(u, v Value) Value {
				return append(u.(Vector), v.(Vector)...)
			},
			matrixType: func(u, v Value) Value {
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
				return newMatrix(newShape, newData)
			},
		},
	}

	take = &binaryOp{
		whichType: atLeastVectorType,
		fn: [numType]binaryFn{
			vectorType: func(u, v Value) Value {
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
			vectorType: func(u, v Value) Value {
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
		"log":  binaryLog,
		"&":    bitAnd,
		"|":    bitOr,
		"^":    bitXor,
		"<<":   lsh,
		">>":   rsh,
		"==":   eq,
		"!=":   ne,
		"<":    lt,
		"<=":   le,
		">":    gt,
		">=":   ge,
		"[]":   index,
		"and":  logicalAnd,
		"or":   logicalOr,
		"xor":  logicalXor,
		"nand": logicalNand,
		"nor":  logicalNor,
		"iota": binaryIota,
		"min":  min,
		"max":  max,
		"rho":  binaryRho,
		",":    binaryCatenate,
		"take": take,
		"drop": drop,
	}
}
