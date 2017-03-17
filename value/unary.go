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
func unaryBigIntOp(c Context, op func(Context, *big.Int, *big.Int) *big.Int, v Value) Value {
	i := v.(BigInt)
	z := bigInt64(0)
	op(c, z.Int, i.Int)
	return z.shrink()
}

func bigIntWrap(op func(*big.Int, *big.Int) *big.Int) func(Context, *big.Int, *big.Int) *big.Int {
	return func(_ Context, u *big.Int, v *big.Int) *big.Int {
		return op(u, v)
	}
}

// unaryBigRatOp applies the op to a BigRat.
func unaryBigRatOp(op func(*big.Rat, *big.Rat) *big.Rat, v Value) Value {
	i := v.(BigRat)
	z := bigRatInt64(0)
	op(z.Rat, i.Rat)
	return z.shrink()
}

// unaryBigFloatOp applies the op to a BigFloat.
func unaryBigFloatOp(c Context, op func(Context, *big.Float, *big.Float) *big.Float, v Value) Value {
	i := v.(BigFloat)
	z := bigFloatInt64(c.Config(), 0)
	op(c, z.Float, i.Float)
	return z.shrink()
}

func bigFloatWrap(op func(*big.Float, *big.Float) *big.Float) func(Context, *big.Float, *big.Float) *big.Float {
	return func(_ Context, u *big.Float, v *big.Float) *big.Float {
		return op(u, v)
	}
}

// bigIntRand sets a to a random number in [origin, origin+b].
func bigIntRand(c Context, a, b *big.Int) *big.Int {
	a.Rand(c.Config().Random(), b)
	return a.Add(a, c.Config().BigOrigin())
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
func floatSelf(c Context, v Value) Value {
	conf := c.Config()
	switch v.(type) {
	case Int:
		return v.(Int).toType(conf, bigFloatType)
	case BigInt:
		return v.(BigInt).toType(conf, bigFloatType)
	case BigRat:
		return v.(BigRat).toType(conf, bigFloatType)
	case BigFloat:
		return v
	}
	Errorf("internal error: floatSelf of non-number")
	return nil
}

// text returns a vector of Chars holding the string representation
// of the value.
func text(c Context, v Value) Value {
	str := v.Sprint(c.Config())
	elem := make([]Value, utf8.RuneCountInString(str))
	for i, r := range str {
		elem[i] = Char(r)
	}
	return NewVector(elem)
}

// Implemented in main, handled as a func to avoid a dependency loop.
var IvyEval func(context Context, s string) Value

var UnaryOps = make(map[string]UnaryOp)

func init() {
	ops := []*unaryOp{
		{
			name:        "?",
			elementwise: true,
			fn: [numType]unaryFn{
				intType: func(c Context, v Value) Value {
					i := int64(v.(Int))
					if i <= 0 {
						Errorf("illegal roll value %v", v)
					}
					return Int(c.Config().Origin()) + Int(c.Config().Random().Int63n(i))
				},
				bigIntType: func(c Context, v Value) Value {
					if v.(BigInt).Sign() <= 0 {
						Errorf("illegal roll value %v", v)
					}
					return unaryBigIntOp(c, bigIntRand, v)
				},
			},
		},

		{
			name: "+",
			fn: [numType]unaryFn{
				intType:      self,
				bigIntType:   self,
				bigRatType:   self,
				bigFloatType: self,
				vectorType:   self,
				matrixType:   self,
			},
		},

		{
			name:        "-",
			elementwise: true,
			fn: [numType]unaryFn{
				intType: func(c Context, v Value) Value {
					return -v.(Int)
				},
				bigIntType: func(c Context, v Value) Value {
					return unaryBigIntOp(c, bigIntWrap((*big.Int).Neg), v)
				},
				bigRatType: func(c Context, v Value) Value {
					return unaryBigRatOp((*big.Rat).Neg, v)
				},
				bigFloatType: func(c Context, v Value) Value {
					return unaryBigFloatOp(c, bigFloatWrap((*big.Float).Neg), v)
				},
			},
		},

		{
			name:        "/",
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
					one := new(big.Float).SetPrec(c.Config().FloatPrec()).SetInt64(1)
					return BigFloat{
						Float: one.Quo(one, f.Float),
					}.shrink()
				},
			},
		},

		{
			name:        "sgn",
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
		},

		{
			name:        "^",
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
		},

		{
			name:        "not",
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
		},

		{
			name:        "abs",
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
					return unaryBigIntOp(c, bigIntWrap((*big.Int).Abs), v)
				},
				bigRatType: func(c Context, v Value) Value {
					return unaryBigRatOp((*big.Rat).Abs, v)
				},
				bigFloatType: func(c Context, v Value) Value {
					return unaryBigFloatOp(c, bigFloatWrap((*big.Float).Abs), v)
				},
			},
		},

		{
			name:        "floor",
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
		},

		{
			name:        "ceil",
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
		},

		{
			name: "iota",
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
						n[k] = Int(k + c.Config().Origin())
					}
					return NewVector(n)
				},
			},
		},

		{
			name: "rho",
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
		},

		{
			name: ",",
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
		},

		{
			name: "up",
			fn: [numType]unaryFn{
				intType:      self,
				charType:     self,
				bigIntType:   self,
				bigRatType:   self,
				bigFloatType: self,
				vectorType: func(c Context, v Value) Value {
					if c == nil {
						panic("NIL IN gradeUP")
					}
					return v.(Vector).grade(c)
				},
			},
		},

		{
			name: "down",
			fn: [numType]unaryFn{
				intType:      self,
				charType:     self,
				bigIntType:   self,
				bigRatType:   self,
				bigFloatType: self,
				vectorType: func(c Context, v Value) Value {
					x := v.(Vector).grade(c)
					for i, j := 0, len(x)-1; i < j; i, j = i+1, j-1 {
						x[i], x[j] = x[j], x[i]
					}
					return x
				},
			},
		},

		{
			name: "rot",
			fn: [numType]unaryFn{
				intType:      self,
				charType:     self,
				bigIntType:   self,
				bigRatType:   self,
				bigFloatType: self,
				vectorType: func(c Context, v Value) Value {
					x := v.(Vector).Copy()
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
						Errorf("rot: matrix is vector")
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
		},

		{
			name: "flip",
			fn: [numType]unaryFn{
				intType:      self,
				charType:     self,
				bigIntType:   self,
				bigRatType:   self,
				bigFloatType: self,
				vectorType: func(c Context, v Value) Value {
					return c.EvalUnary("rot", v)
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
		},

		{
			name:        "cos",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      func(c Context, v Value) Value { return cos(c, v) },
				bigIntType:   func(c Context, v Value) Value { return cos(c, v) },
				bigRatType:   func(c Context, v Value) Value { return cos(c, v) },
				bigFloatType: func(c Context, v Value) Value { return cos(c, v) },
			},
		},

		{
			name:        "log",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      func(c Context, v Value) Value { return logn(c, v) },
				bigIntType:   func(c Context, v Value) Value { return logn(c, v) },
				bigRatType:   func(c Context, v Value) Value { return logn(c, v) },
				bigFloatType: func(c Context, v Value) Value { return logn(c, v) },
			},
		},

		{
			name:        "sin",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      func(c Context, v Value) Value { return sin(c, v) },
				bigIntType:   func(c Context, v Value) Value { return sin(c, v) },
				bigRatType:   func(c Context, v Value) Value { return sin(c, v) },
				bigFloatType: func(c Context, v Value) Value { return sin(c, v) },
			},
		},

		{
			name:        "tan",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      func(c Context, v Value) Value { return tan(c, v) },
				bigIntType:   func(c Context, v Value) Value { return tan(c, v) },
				bigRatType:   func(c Context, v Value) Value { return tan(c, v) },
				bigFloatType: func(c Context, v Value) Value { return tan(c, v) },
			},
		},

		{
			name:        "asin",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      func(c Context, v Value) Value { return asin(c, v) },
				bigIntType:   func(c Context, v Value) Value { return asin(c, v) },
				bigRatType:   func(c Context, v Value) Value { return asin(c, v) },
				bigFloatType: func(c Context, v Value) Value { return asin(c, v) },
			},
		},

		{
			name:        "acos",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      func(c Context, v Value) Value { return acos(c, v) },
				bigIntType:   func(c Context, v Value) Value { return acos(c, v) },
				bigRatType:   func(c Context, v Value) Value { return acos(c, v) },
				bigFloatType: func(c Context, v Value) Value { return acos(c, v) },
			},
		},

		{
			name:        "atan",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      func(c Context, v Value) Value { return atan(c, v) },
				bigIntType:   func(c Context, v Value) Value { return atan(c, v) },
				bigRatType:   func(c Context, v Value) Value { return atan(c, v) },
				bigFloatType: func(c Context, v Value) Value { return atan(c, v) },
			},
		},

		{
			name:        "**",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      func(c Context, v Value) Value { return exp(c, v) },
				bigIntType:   func(c Context, v Value) Value { return exp(c, v) },
				bigRatType:   func(c Context, v Value) Value { return exp(c, v) },
				bigFloatType: func(c Context, v Value) Value { return exp(c, v) },
			},
		},

		{
			name:        "sqrt",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      func(c Context, v Value) Value { return sqrt(c, v) },
				bigIntType:   func(c Context, v Value) Value { return sqrt(c, v) },
				bigRatType:   func(c Context, v Value) Value { return sqrt(c, v) },
				bigFloatType: func(c Context, v Value) Value { return sqrt(c, v) },
			},
		},

		{
			name:        "char",
			elementwise: true,
			fn: [numType]unaryFn{
				intType: func(c Context, v Value) Value { return Char(v.(Int)).validate() },
			},
		},

		{
			name:        "code",
			elementwise: true,
			fn: [numType]unaryFn{
				charType: func(c Context, v Value) Value { return Int(v.(Char)) },
			},
		},

		{
			name: "text",
			fn: [numType]unaryFn{
				intType:      func(c Context, v Value) Value { return text(c, v) },
				bigIntType:   func(c Context, v Value) Value { return text(c, v) },
				bigRatType:   func(c Context, v Value) Value { return text(c, v) },
				bigFloatType: func(c Context, v Value) Value { return text(c, v) },
				vectorType:   func(c Context, v Value) Value { return text(c, v) },
				matrixType:   func(c Context, v Value) Value { return text(c, v) },
			},
		},

		{
			name: "ivy",
			fn: [numType]unaryFn{
				vectorType: func(c Context, v Value) Value {
					text := v.(Vector)
					if !text.AllChars() {
						Errorf("ivy: value is not a vector of char")
					}
					return IvyEval(c, text.makeString(c.Config(), false))
				},
			},
		},

		{
			name:        "float",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      floatSelf,
				bigIntType:   floatSelf,
				bigRatType:   floatSelf,
				bigFloatType: floatSelf,
			},
		},
	}

	for _, op := range ops {
		UnaryOps[op.name] = op
	}
}
