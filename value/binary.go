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
			panic(Errorf("illegal shift count %d", count))
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
	panic(Error("illegal shift count type"))
}

// binaryVectorOp applies op elementwise to i and j.
func binaryVectorOp(i Value, op string, j Value) Value {
	u, v := i.(Vector), j.(Vector)
	if len(u) == 1 {
		n := make([]Value, v.Len())
		for k := range v {
			n[k] = Binary(u[0], op, v[k])
		}
		return ValueSlice(n)
	}
	if len(v) == 1 {
		n := make([]Value, u.Len())
		for k := range u {
			n[k] = Binary(u[k], op, v[0])
		}
		return ValueSlice(n)
	}
	u.sameLength(v)
	n := make([]Value, u.Len())
	for k := range u {
		n[k] = Binary(u[k], op, v[k])
	}
	return ValueSlice(n)
}

// binaryMatrixOp applies op elementwise to i and j.
func binaryMatrixOp(i Value, op string, j Value) Value {
	u, v := i.(Matrix), j.(Matrix)
	u.sameShape(v)
	n := make([]Value, u.data.Len())
	for k := range u.data {
		n[k] = Binary(u.data[k], op, v.data[k])
	}
	return Matrix{
		u.shape,
		ValueSlice(n),
	}
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
	binaryIota, rho           *binaryOp
	min, max                  *binaryOp
	binaryOps                 map[string]*binaryOp
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
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return u.(Int) + v.(Int)
			},
			bigIntType: func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Add, v)
			},
			bigRatType: func(u, v Value) Value {
				return binaryBigRatOp(u, (*big.Rat).Add, v)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "+", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, "+", v)
			},
		},
	}

	sub = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return u.(Int) - v.(Int)
			},
			bigIntType: func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Sub, v)
			},
			bigRatType: func(u, v Value) Value {
				return binaryBigRatOp(u, (*big.Rat).Sub, v)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "-", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, "-", v)
			},
		},
	}

	mul = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return u.(Int) * v.(Int)
			},
			bigIntType: func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Mul, v)
			},
			bigRatType: func(u, v Value) Value {
				return binaryBigRatOp(u, (*big.Rat).Mul, v)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "*", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, "*", v)
			},
		},
	}

	quo = &binaryOp{ // Rational division.
		whichType: rationalType, // Use BigRats to avoid the analysis here.
		fn: [numType]binaryFn{
			bigRatType: func(u, v Value) Value {
				if v.(BigRat).Sign() == 0 {
					panic(Error("division by zero"))
				}
				return binaryBigRatOp(u, (*big.Rat).Quo, v) // True division.
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "/", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, "/", v)
			},
		},
	}

	idiv = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				if v.(Int) == 0 {
					panic(Error("division by zero"))
				}
				return u.(Int) / v.(Int)
			},
			bigIntType: func(u, v Value) Value {
				if v.(BigInt).Sign() == 0 {
					panic(Error("division by zero"))
				}
				return binaryBigIntOp(u, (*big.Int).Quo, v) // Go-like division.
			},
			bigRatType: nil, // Not defined for rationals. Use div.
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "idiv", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, "idiv", v)
			},
		},
	}

	imod = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				if v.(Int) == 0 {
					panic(Error("modulo by zero"))
				}
				return u.(Int) % v.(Int)
			},
			bigIntType: func(u, v Value) Value {
				if v.(BigInt).Sign() == 0 {
					panic(Error("modulo by zero"))
				}
				return binaryBigIntOp(u, (*big.Int).Rem, v) // Go-like modulo.
			},
			bigRatType: nil, // Not defined for rationals. Use mod.
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "imod", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, "imod", v)
			},
		},
	}

	div = &binaryOp{ // Euclidean integer division.
		whichType: divType, // Use BigInts to avoid the analysis here.
		fn: [numType]binaryFn{
			bigIntType: func(u, v Value) Value {
				if v.(BigInt).Sign() == 0 {
					panic(Error("division by zero"))
				}
				return binaryBigIntOp(u, (*big.Int).Div, v) // Euclidean division.
			},
			bigRatType: nil, // Not defined for rationals. Use div.
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "div", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, "div", v)
			},
		},
	}

	mod = &binaryOp{ // Euclidean integer modulus.
		whichType: divType, // Use BigInts to avoid the analysis here.
		fn: [numType]binaryFn{
			bigIntType: func(u, v Value) Value {
				if v.(BigInt).Sign() == 0 {
					panic(Error("modulo by zero"))
				}
				return binaryBigIntOp(u, (*big.Int).Mod, v) // Euclidan modulo.
			},
			bigRatType: nil, // Not defined for rationals. Use mod.
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "mod", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, "mod", v)
			},
		},
	}

	pow = &binaryOp{
		whichType: divType,
		fn: [numType]binaryFn{
			bigIntType: func(u, v Value) Value {
				switch v.(BigInt).Sign() {
				case 0:
					return one
				case -1:
					panic(Error("negative exponent not implemented"))
				}
				return binaryBigIntOp(u, bigIntPow, v)
			},
			bigRatType: func(u, v Value) Value {
				// We know v is integral. (n/d)**2 is n**2/d**2.
				rexp := v.(BigRat)
				switch rexp.Sign() {
				case 0:
					return one
				case -1:
					panic(Error("negative exponent not implemented"))
				}
				exp := rexp.Num()
				rat := u.(BigRat)
				num := rat.Num()
				den := rat.Denom()
				num.Exp(num, exp, nil)
				den.Exp(den, exp, nil)
				z := bigRatInt64(0)
				z.SetFrac(num, den)
				return z
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "**", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, "**", v)
			},
		},
	}

	and = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return u.(Int) & v.(Int)
			},
			bigIntType: func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).And, v)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "&", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, "&", v)
			},
		},
	}

	or = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return u.(Int) | v.(Int)
			},
			bigIntType: func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Or, v)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "|", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, "|", v)
			},
		},
	}

	xor = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return u.(Int) ^ v.(Int)
			},
			bigIntType: func(u, v Value) Value {
				return binaryBigIntOp(u, (*big.Int).Xor, v)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "^", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, "^", v)
			},
		},
	}

	lsh = &binaryOp{
		whichType: divType, // Shifts are like power: let BigInt do the work.
		fn: [numType]binaryFn{
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				z := bigInt64(0)
				z.Lsh(i.Int, shiftCount(j))
				return z.shrink()
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "<<", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, "<<", v)
			},
		},
	}

	rsh = &binaryOp{
		whichType: divType, // Shifts are like power: let BigInt do the work.
		fn: [numType]binaryFn{
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				z := bigInt64(0)
				z.Rsh(i.Int, shiftCount(j))
				return z.shrink()
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, ">>", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, ">>", v)
			},
		},
	}

	eq = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(u.(Int) == v.(Int))
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.Cmp(j.Int) == 0)
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.Cmp(j.Rat) == 0)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "==", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, "==", v)
			},
		},
	}

	ne = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(u.(Int) != v.(Int))
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.Cmp(j.Int) != 0)
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.Cmp(j.Rat) != 0)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "!=", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, "!=", v)
			},
		},
	}

	lt = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(u.(Int) < v.(Int))
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.Cmp(j.Int) < 0)
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.Cmp(j.Rat) < 0)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "<", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, "<", v)
			},
		},
	}

	le = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(u.(Int) <= v.(Int))
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.Cmp(j.Int) <= 0)
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.Cmp(j.Rat) <= 0)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "<=", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, "<=", v)
			},
		},
	}

	gt = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(u.(Int) > v.(Int))
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.Cmp(j.Int) > 0)
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.Cmp(j.Rat) > 0)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, ">", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, ">", v)
			},
		},
	}

	ge = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				return toInt(u.(Int) >= v.(Int))
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				return toInt(i.Cmp(j.Int) >= 0)
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				return toInt(i.Cmp(j.Rat) >= 0)
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, ">=", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, ">=", v)
			},
		},
	}

	binaryIota = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			vectorType: func(u, v Value) Value {
				// A⍳B: The location (index) of B in A; 0 if not found. (APL does 1+⌈/⍳⍴A)
				A, B := u.(Vector), v.(Vector)
				indices := make([]Value, len(B))
				// TODO: This is n^2.
			Outer:
				for i, b := range B {
					for j, a := range A {
						if Binary(a, "==", b).(Int) == 1 {
							indices[i] = Int(j + 1)
							continue Outer
						}
					}
					indices[i] = zero
				}
				return ValueSlice(indices)
			},
		},
	}

	min = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				if u.(Int) < v.(Int) {
					return u
				}
				return v
			},
			bigIntType: func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				if i.Cmp(j.Int) < 0 {
					return u
				}
				return v
			},
			bigRatType: func(u, v Value) Value {
				i, j := u.(BigRat), v.(BigRat)
				if i.Cmp(j.Rat) < 0 {
					return u
				}
				return v
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "min", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, "min", v)
			},
		},
	}

	max = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			intType: func(u, v Value) Value {
				if u.(Int) > v.(Int) {
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
					return u
				}
				return v
			},
			vectorType: func(u, v Value) Value {
				return binaryVectorOp(u, "min", v)
			},
			matrixType: func(u, v Value) Value {
				return binaryMatrixOp(u, "max", v)
			},
		},
	}

	rho = &binaryOp{
		whichType: atLeastVectorType, // TODO: correct?
		fn: [numType]binaryFn{
			vectorType: func(u, v Value) Value {
				return reshape(u.(Vector), v.(Vector))
			},
			matrixType: func(u, v Value) Value {
				// LHS must be a vector underneath.
				A, B := u.(Matrix), v.(Matrix)
				if len(A.shape) != 1 {
					panic(Error("lhs of rho cannot be matrix"))
				}
				return reshape(A.data, B.data)
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
		"iota": binaryIota,
		"min":  min,
		"max":  max,
		"rho":  rho,
	}
}
