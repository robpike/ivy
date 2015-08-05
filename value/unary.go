// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"math/big"
	"unicode/utf8"
)

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

// unaryBigFloatOp applies the op to a BigFloat.
func unaryBigFloatOp(op func(*big.Float, *big.Float) *big.Float, v Value) Value {
	i := v.(BigFloat)
	z := bigFloatInt64(0)
	op(z.Float, i.Float)
	return z.shrink()
}

var (
	unaryRoll                         *unaryOp
	unaryPlus, unaryMinus, unaryRecip *unaryOp
	unaryAbs, unarySignum             *unaryOp
	unaryBitwiseNot, unaryLogicalNot  *unaryOp
	unaryIota, unaryRho, unaryRavel   *unaryOp
	gradeUp, gradeDown                *unaryOp
	reverse, flip                     *unaryOp
	floor, ceil                       *unaryOp
	unaryExp                          *unaryOp
	unaryCos, unarySin, unaryTan      *unaryOp
	unaryAcos, unaryAsin, unaryAtan   *unaryOp
	unaryLog, unarySqrt               *unaryOp
	unaryChar, unaryCode, unaryText   *unaryOp
	unaryFloat                        *unaryOp
	unaryIvy                          *unaryOp
	unaryOps                          map[string]*unaryOp
)

// bigIntRand sets a to a random number in [origin, origin+b].
func bigIntRand(a, b *big.Int) *big.Int {
	a.Rand(conf.Random(), b)
	return a.Add(a, conf.BigOrigin())
}

func self(c Context, v Value) Value {
	return v
}

// vectorSelf promotes v to type Vector.
// v must be a scalar.
func vectorSelf(c Context, v Value) Value {
	switch v.(type) {
	case Vector:
		Errorf("internal error: vectorSelf of vector")
	case Matrix:
		Errorf("internal error: vectorSelf of matrix")
	}
	return NewVector([]Value{v})
}

// floatSelf promotes v to type BigFloat.
func floatSelf(_ Context, v Value) Value {
	switch v.(type) {
	case Int:
		return v.(Int).toType(bigFloatType)
	case BigInt:
		return v.(BigInt).toType(bigFloatType)
	case BigRat:
		return v.(BigRat).toType(bigFloatType)
	case BigFloat:
		return v
	}
	Errorf("internal error: floatSelf of non-number")
	return nil
}

// text returns a vector of Chars holding the string representation
// of the value.
func text(v Value) Value {
	str := v.String()
	elem := make([]Value, utf8.RuneCountInString(str))
	for i, r := range str {
		elem[i] = Char(r)
	}
	return NewVector(elem)
}

var context Context

// SetContext sets the context for evaluation; needed by IvyEval.
func SetContext(c Context) {
	context = c
}

// Implemented in main, handled as a func to avoid a dependency loop.
var IvyEval func(context Context, s string) Value

func init() {
	unaryRoll = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType: func(c Context, v Value) Value {
				i := int64(v.(Int))
				if i <= 0 {
					Errorf("illegal roll value %v", v)
				}
				return Int(conf.Origin()) + Int(conf.Random().Int63n(i))
			},
			bigIntType: func(c Context, v Value) Value {
				if v.(BigInt).Sign() <= 0 {
					Errorf("illegal roll value %v", v)
				}
				return unaryBigIntOp(bigIntRand, v)
			},
		},
	}

	unaryPlus = &unaryOp{
		fn: [numType]unaryFn{
			intType:      self,
			bigIntType:   self,
			bigRatType:   self,
			bigFloatType: self,
			vectorType:   self,
			matrixType:   self,
		},
	}

	unaryMinus = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType: func(c Context, v Value) Value {
				return -v.(Int)
			},
			bigIntType: func(c Context, v Value) Value {
				return unaryBigIntOp((*big.Int).Neg, v)
			},
			bigRatType: func(c Context, v Value) Value {
				return unaryBigRatOp((*big.Rat).Neg, v)
			},
			bigFloatType: func(c Context, v Value) Value {
				return unaryBigFloatOp((*big.Float).Neg, v)
			},
		},
	}

	unaryRecip = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType: func(c Context, v Value) Value {
				i := int64(v.(Int))
				if i == 0 {
					Errorf("division by zero")
				}
				return BigRat{
					Rat: big.NewRat(0, 1).SetFrac64(1, i),
				}.shrink()
			},
			bigIntType: func(c Context, v Value) Value {
				// Zero division cannot happen for unary.
				return BigRat{
					Rat: big.NewRat(0, 1).SetFrac(bigOne.Int, v.(BigInt).Int),
				}.shrink()
			},
			bigRatType: func(c Context, v Value) Value {
				// Zero division cannot happen for unary.
				r := v.(BigRat)
				return BigRat{
					Rat: big.NewRat(0, 1).SetFrac(r.Denom(), r.Num()),
				}.shrink()
			},
			bigFloatType: func(c Context, v Value) Value {
				// Zero division cannot happen for unary.
				f := v.(BigFloat)
				one := new(big.Float).SetPrec(conf.FloatPrec()).SetInt64(1)
				return BigFloat{
					Float: one.Quo(one, f.Float),
				}.shrink()
			},
		},
	}

	unarySignum = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType: func(c Context, v Value) Value {
				i := int64(v.(Int))
				if i > 0 {
					return one
				}
				if i < 0 {
					return minusOne
				}
				return zero
			},
			bigIntType: func(c Context, v Value) Value {
				return Int(v.(BigInt).Sign())
			},
			bigRatType: func(c Context, v Value) Value {
				return Int(v.(BigRat).Sign())
			},
			bigFloatType: func(c Context, v Value) Value {
				return Int(v.(BigFloat).Sign())
			},
		},
	}

	unaryBitwiseNot = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType: func(c Context, v Value) Value {
				return ^v.(Int)
			},
			bigIntType: func(c Context, v Value) Value {
				// Lots of ways to do this, here's one.
				return BigInt{Int: bigInt64(0).Xor(v.(BigInt).Int, bigMinusOne.Int)}
			},
		},
	}

	unaryLogicalNot = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType: func(c Context, v Value) Value {
				if v.(Int) == 0 {
					return one
				}
				return zero
			},
			bigIntType: func(c Context, v Value) Value {
				if v.(BigInt).Sign() == 0 {
					return one
				}
				return zero
			},
			bigRatType: func(c Context, v Value) Value {
				if v.(BigRat).Sign() == 0 {
					return one
				}
				return zero
			},
			bigFloatType: func(c Context, v Value) Value {
				if v.(BigFloat).Sign() == 0 {
					return one
				}
				return zero
			},
		},
	}

	unaryAbs = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType: func(c Context, v Value) Value {
				i := v.(Int)
				if i < 0 {
					i = -i
				}
				return i
			},
			bigIntType: func(c Context, v Value) Value {
				return unaryBigIntOp((*big.Int).Abs, v)
			},
			bigRatType: func(c Context, v Value) Value {
				return unaryBigRatOp((*big.Rat).Abs, v)
			},
			bigFloatType: func(c Context, v Value) Value {
				return unaryBigFloatOp((*big.Float).Abs, v)
			},
		},
	}

	floor = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType:    func(c Context, v Value) Value { return v },
			bigIntType: func(c Context, v Value) Value { return v },
			bigRatType: func(c Context, v Value) Value {
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
			bigFloatType: func(c Context, v Value) Value {
				f := v.(BigFloat)
				i, acc := f.Int(nil)
				switch acc {
				case big.Exact, big.Below:
					// Done.
				case big.Above:
					i.Sub(i, bigOne.Int)
				}
				return BigInt{i}.shrink()
			},
		},
	}

	ceil = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType:    func(c Context, v Value) Value { return v },
			bigIntType: func(c Context, v Value) Value { return v },
			bigRatType: func(c Context, v Value) Value {
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
			bigFloatType: func(c Context, v Value) Value {
				f := v.(BigFloat)
				i, acc := f.Int(nil)
				switch acc {
				case big.Exact, big.Above:
					// Done
				case big.Below:
					i.Add(i, bigOne.Int)
				}
				return BigInt{i}.shrink()
			},
		},
	}

	unaryIota = &unaryOp{
		fn: [numType]unaryFn{
			intType: func(c Context, v Value) Value {
				i := v.(Int)
				if i < 0 || maxInt < i {
					Errorf("bad iota %d", i)
				}
				if i == 0 {
					return Vector{}
				}
				n := make([]Value, i)
				for k := range n {
					n[k] = Int(k + conf.Origin())
				}
				return NewVector(n)
			},
		},
	}

	unaryRho = &unaryOp{
		fn: [numType]unaryFn{
			intType: func(c Context, v Value) Value {
				return Vector{}
			},
			charType: func(c Context, v Value) Value {
				return Vector{}
			},
			bigIntType: func(c Context, v Value) Value {
				return Vector{}
			},
			bigRatType: func(c Context, v Value) Value {
				return Vector{}
			},
			bigFloatType: func(c Context, v Value) Value {
				return Vector{}
			},
			vectorType: func(c Context, v Value) Value {
				return Int(len(v.(Vector)))
			},
			matrixType: func(c Context, v Value) Value {
				return v.(Matrix).shape
			},
		},
	}

	unaryRavel = &unaryOp{
		fn: [numType]unaryFn{
			intType:      vectorSelf,
			charType:     vectorSelf,
			bigIntType:   vectorSelf,
			bigRatType:   vectorSelf,
			bigFloatType: vectorSelf,
			vectorType:   self,
			matrixType: func(c Context, v Value) Value {
				return v.(Matrix).data
			},
		},
	}

	gradeUp = &unaryOp{
		fn: [numType]unaryFn{
			intType:      self,
			charType:     self,
			bigIntType:   self,
			bigRatType:   self,
			bigFloatType: self,
			vectorType: func(c Context, v Value) Value {
				return v.(Vector).grade()
			},
		},
	}

	gradeDown = &unaryOp{
		fn: [numType]unaryFn{
			intType:      self,
			charType:     self,
			bigIntType:   self,
			bigRatType:   self,
			bigFloatType: self,
			vectorType: func(c Context, v Value) Value {
				x := v.(Vector).grade()
				for i, j := 0, len(x)-1; i < j; i, j = i+1, j-1 {
					x[i], x[j] = x[j], x[i]
				}
				return x
			},
		},
	}

	reverse = &unaryOp{
		fn: [numType]unaryFn{
			intType:      self,
			charType:     self,
			bigIntType:   self,
			bigRatType:   self,
			bigFloatType: self,
			vectorType: func(c Context, v Value) Value {
				x := v.(Vector)
				for i, j := 0, len(x)-1; i < j; i, j = i+1, j-1 {
					x[i], x[j] = x[j], x[i]
				}
				return x
			},
			matrixType: func(c Context, v Value) Value {
				m := v.(Matrix)
				if len(m.shape) == 0 {
					return m
				}
				if len(m.shape) == 1 {
					Errorf("rev: matrix is vector")
				}
				size := m.size()
				ncols := int(m.shape[len(m.shape)-1].(Int))
				x := m.data
				for index := 0; index <= size-ncols; index += ncols {
					for i, j := 0, ncols-1; i < j; i, j = i+1, j-1 {
						x[index+i], x[index+j] = x[index+j], x[index+i]
					}
				}
				return m
			},
		},
	}

	flip = &unaryOp{
		fn: [numType]unaryFn{
			intType:      self,
			charType:     self,
			bigIntType:   self,
			bigRatType:   self,
			bigFloatType: self,
			vectorType: func(c Context, v Value) Value {
				return Unary(c, "rev", v)
			},
			matrixType: func(c Context, v Value) Value {
				m := v.(Matrix)
				if len(m.shape) == 0 {
					return m
				}
				if len(m.shape) == 1 {
					Errorf("flip: matrix is vector")
				}
				elemSize := m.elemSize()
				size := m.size()
				x := m.data
				lo := 0
				hi := size - elemSize
				for lo < hi {
					for i := 0; i < elemSize; i++ {
						x[lo+i], x[hi+i] = x[hi+i], x[lo+i]
					}
					lo += elemSize
					hi -= elemSize
				}
				return m
			},
		},
	}

	unaryCos = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType:      func(c Context, v Value) Value { return cos(v) },
			bigIntType:   func(c Context, v Value) Value { return cos(v) },
			bigRatType:   func(c Context, v Value) Value { return cos(v) },
			bigFloatType: func(c Context, v Value) Value { return cos(v) },
		},
	}

	unaryLog = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType:      func(c Context, v Value) Value { return logn(v) },
			bigIntType:   func(c Context, v Value) Value { return logn(v) },
			bigRatType:   func(c Context, v Value) Value { return logn(v) },
			bigFloatType: func(c Context, v Value) Value { return logn(v) },
		},
	}

	unarySin = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType:      func(c Context, v Value) Value { return sin(v) },
			bigIntType:   func(c Context, v Value) Value { return sin(v) },
			bigRatType:   func(c Context, v Value) Value { return sin(v) },
			bigFloatType: func(c Context, v Value) Value { return sin(v) },
		},
	}

	unaryTan = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType:      func(c Context, v Value) Value { return tan(v) },
			bigIntType:   func(c Context, v Value) Value { return tan(v) },
			bigRatType:   func(c Context, v Value) Value { return tan(v) },
			bigFloatType: func(c Context, v Value) Value { return tan(v) },
		},
	}

	unaryAsin = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType:      func(c Context, v Value) Value { return asin(v) },
			bigIntType:   func(c Context, v Value) Value { return asin(v) },
			bigRatType:   func(c Context, v Value) Value { return asin(v) },
			bigFloatType: func(c Context, v Value) Value { return asin(v) },
		},
	}

	unaryAcos = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType:      func(c Context, v Value) Value { return acos(v) },
			bigIntType:   func(c Context, v Value) Value { return acos(v) },
			bigRatType:   func(c Context, v Value) Value { return acos(v) },
			bigFloatType: func(c Context, v Value) Value { return acos(v) },
		},
	}

	unaryAtan = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType:      func(c Context, v Value) Value { return atan(v) },
			bigIntType:   func(c Context, v Value) Value { return atan(v) },
			bigRatType:   func(c Context, v Value) Value { return atan(v) },
			bigFloatType: func(c Context, v Value) Value { return atan(v) },
		},
	}

	unaryExp = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType:      func(c Context, v Value) Value { return exp(v) },
			bigIntType:   func(c Context, v Value) Value { return exp(v) },
			bigRatType:   func(c Context, v Value) Value { return exp(v) },
			bigFloatType: func(c Context, v Value) Value { return exp(v) },
		},
	}

	unarySqrt = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType:      func(c Context, v Value) Value { return sqrt(v) },
			bigIntType:   func(c Context, v Value) Value { return sqrt(v) },
			bigRatType:   func(c Context, v Value) Value { return sqrt(v) },
			bigFloatType: func(c Context, v Value) Value { return sqrt(v) },
		},
	}

	unaryChar = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType: func(c Context, v Value) Value { return Char(v.(Int)).validate() },
		},
	}

	unaryCode = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			charType: func(c Context, v Value) Value { return Int(v.(Char)) },
		},
	}

	unaryText = &unaryOp{
		fn: [numType]unaryFn{
			intType:      func(c Context, v Value) Value { return text(v) },
			bigIntType:   func(c Context, v Value) Value { return text(v) },
			bigRatType:   func(c Context, v Value) Value { return text(v) },
			bigFloatType: func(c Context, v Value) Value { return text(v) },
			vectorType:   func(c Context, v Value) Value { return text(v) },
			matrixType:   func(c Context, v Value) Value { return text(v) },
		},
	}

	unaryIvy = &unaryOp{
		fn: [numType]unaryFn{
			vectorType: func(c Context, v Value) Value {
				text := v.(Vector)
				if !text.AllChars() {
					Errorf("ivy: value is not a vector of char")
				}
				return IvyEval(context, text.makeString(false))
			},
		},
	}

	unaryFloat = &unaryOp{
		elementwise: true,
		fn: [numType]unaryFn{
			intType:      floatSelf,
			bigIntType:   floatSelf,
			bigRatType:   floatSelf,
			bigFloatType: floatSelf,
		},
	}

	unaryOps = map[string]*unaryOp{
		"**":    unaryExp,
		"+":     unaryPlus,
		",":     unaryRavel,
		"-":     unaryMinus,
		"/":     unaryRecip,
		"?":     unaryRoll,
		"^":     unaryBitwiseNot,
		"abs":   unaryAbs,
		"acos":  unaryAcos,
		"asin":  unaryAsin,
		"atan":  unaryAtan,
		"ceil":  ceil,
		"char":  unaryChar,
		"code":  unaryCode,
		"cos":   unaryCos,
		"down":  gradeDown,
		"flip":  flip,
		"float": unaryFloat,
		"floor": floor,
		"iota":  unaryIota,
		"ivy":   unaryIvy,
		"log":   unaryLog,
		"not":   unaryLogicalNot,
		"rev":   reverse,
		"rho":   unaryRho,
		"sgn":   unarySignum,
		"sin":   unarySin,
		"sqrt":  unarySqrt,
		"tan":   unaryTan,
		"text":  unaryText,
		"up":    gradeUp,
	}
}
