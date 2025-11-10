// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"crypto/rand"
	"fmt"
	"math/big"
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
	config := c.Config()
	config.LockRandom()
	for {
		i, err := rand.Int(config.Source(), b)
		if err == nil {
			a.Set(i)
			break
		}
	}
	config.UnlockRandom()
	return a
}

func self(c Context, v Value) Value {
	return v
}

func returnZero(c Context, v Value) Value {
	return zero
}

func realPhase(c Context, v Value) Value {
	if isNegative(v) {
		return BigFloat{newFloat(c).Set(floatPi)}
	}
	return zero
}

// vectorSelf promotes v to type Vector.
// v must be a scalar.
func vectorSelf(c Context, v Value) Value {
	switch v.(type) {
	case *Vector:
		Errorf("internal error: vectorSelf of vector")
	case *Matrix:
		Errorf("internal error: vectorSelf of matrix")
	}
	return oneElemVector(v)
}

// floatValueSelf promotes v to type BigFloat, and wraps it as a value.
func floatValueSelf(c Context, v Value) Value {
	return floatSelf(c, v)
}

// floatSelf promotes v to type BigFloat.
func floatSelf(c Context, v Value) BigFloat {
	conf := c.Config()
	switch v := v.(type) {
	case Int:
		return v.toType("float", conf, bigFloatType).(BigFloat)
	case BigInt:
		return v.toType("float", conf, bigFloatType).(BigFloat)
	case BigRat:
		return v.toType("float", conf, bigFloatType).(BigFloat)
	case BigFloat:
		return v
	}
	Errorf("internal error: floatSelf of non-number")
	panic("unreached")
}

// text returns a vector of Chars holding the string representation
// of the value.
func text(c Context, v Value) Value {
	return newCharVector(v.Sprint(c.Config()))
}

// newCharVector takes a string and returns its representation as a Vector of Chars.
func newCharVector(s string) Value {
	edit := newVectorEditor(0, nil)
	for _, r := range s {
		edit.Append(Char(r))
	}
	return edit.Publish()
}

// Implemented in package run, handled as a func to avoid a dependency loop.
var IvyEval func(context Context, s string) Value

var UnaryOps = make(map[string]UnaryOp)

func printValue(c Context, v Value) Value {
	fmt.Fprintln(c.Config().Output(), v.Sprint(c.Config()))
	return QuietValue{v}
}

// bigFloatRand returns a uniformly distributed BigFloat in the range [0, f).
// For [0, 1), the mean should be 0.5 and 𝛔 should be 1/√12, or 0.2887.
// A test of a million values yielded 0.500161824174 0.288777488704.
func bigFloatRand(c Context, f *big.Float) Value {
	if f.Sign() < 0 {
		Errorf("rand of negative number")
	}
	// We want two integers FloatPrec bits long.
	config := c.Config()
	max := big.NewInt(1)
	max.Lsh(max, config.FloatPrec())
	max.Add(max, bigIntOne.Int) // We want range [0,n).
	// Now grab a random integer from [0, max)
	randInt := big.NewInt(0)
	bigIntRand(c, randInt, max)
	// Make it a float and normalize it.
	x := big.NewFloat(0).SetInt(randInt)
	x.Quo(x, big.NewFloat(0).SetInt(max))
	// Finally, scale it up to [0, f).
	x.Mul(x, f)
	return BigFloat{x}
}

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
					c.Config().LockRandom()
					res := Int(c.Config().Random().Int64N(int64(i)))
					c.Config().UnlockRandom()
					return Int(c.Config().Origin()) + res
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
			name:        "rand",
			elementwise: true,
			fn: [numType]unaryFn{
				intType: func(c Context, v Value) Value {
					var x big.Float
					x.SetInt64(int64(v.(Int)))
					return bigFloatRand(c, &x)
				},
				bigIntType: func(c Context, v Value) Value {
					var x big.Float
					x.SetInt(v.(BigInt).Int)
					return bigFloatRand(c, &x)
				},
				bigRatType: func(c Context, v Value) Value {
					var x big.Float
					x.SetRat(v.(BigRat).Rat)
					return bigFloatRand(c, &x)
				},
				bigFloatType: func(c Context, v Value) Value {
					return bigFloatRand(c, v.(BigFloat).Float)
				},
			},
		},

		{
			name:        "j",
			elementwise: true,
			fn: [numType]unaryFn{
				intType: func(c Context, v Value) Value {
					return NewComplex(zero, v).shrink()
				},
				bigIntType: func(c Context, v Value) Value {
					return NewComplex(zero, v).shrink()
				},
				bigRatType: func(c Context, v Value) Value {
					return NewComplex(zero, v).shrink()
				},
				bigFloatType: func(c Context, v Value) Value {
					return NewComplex(zero, v).shrink()
				},
				complexType: func(c Context, v Value) Value {
					// Multiply by i.
					u := v.(Complex)
					return NewComplex(c.EvalUnary("-", u.imag), u.real).shrink()
				},
			},
		},

		{
			name: "split",
			fn: [numType]unaryFn{
				intType:      self,
				bigIntType:   self,
				bigRatType:   self,
				bigFloatType: self,
				complexType:  self,
				// TODO: Vector?
				matrixType: func(c Context, v Value) Value {
					return v.(*Matrix).split()
				},
			},
		},

		{
			name: "mix",
			fn: [numType]unaryFn{
				intType:      self,
				bigIntType:   self,
				bigRatType:   self,
				bigFloatType: self,
				complexType:  self,
				vectorType: func(c Context, v Value) Value {
					vv := v.(*Vector)
					return NewMatrix([]int{vv.Len()}, vv).mix(c).shrink()

				},
				matrixType: func(c Context, v Value) Value {
					return v.(*Matrix).mix(c)
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
				complexType:  self,
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
				complexType: func(c Context, v Value) Value {
					return v.(Complex).neg(c).shrink()
				},
			},
		},

		{
			name:        "/",
			elementwise: true,
			fn: [numType]unaryFn{
				intType: func(c Context, v Value) Value {
					return v.(Int).inverse()
				},
				bigIntType: func(c Context, v Value) Value {
					return v.(BigInt).inverse()
				},
				bigRatType: func(c Context, v Value) Value {
					return v.(BigRat).inverse()
				},
				bigFloatType: func(c Context, v Value) Value {
					return v.(BigFloat).inverse()
				},
				complexType: func(c Context, v Value) Value {
					return v.(Complex).inverse(c).shrink()
				},
			},
		},

		{
			name:        "inv",
			elementwise: false,
			fn: [numType]unaryFn{
				intType: func(c Context, v Value) Value {
					return v.(Int).inverse()
				},
				bigIntType: func(c Context, v Value) Value {
					return v.(BigInt).inverse()
				},
				bigRatType: func(c Context, v Value) Value {
					return v.(BigRat).inverse()
				},
				bigFloatType: func(c Context, v Value) Value {
					return v.(BigFloat).inverse()
				},
				complexType: func(c Context, v Value) Value {
					return v.(Complex).inverse(c).shrink()
				},
				vectorType: func(c Context, v Value) Value {
					return v.(*Vector).inverse(c)
				},
				matrixType: func(c Context, v Value) Value {
					return v.(*Matrix).inverse(c)
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
				complexType: func(c Context, v Value) Value {
					return v.(Complex).Signum(c).shrink()
				},
			},
		},
		{
			name:        "conj",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      self,
				bigIntType:   self,
				bigRatType:   self,
				bigFloatType: self,
				complexType: func(c Context, v Value) Value {
					u := v.(Complex)
					return NewComplex(u.real, c.EvalUnary("-", u.imag)).shrink().shrink()
				},
			},
		},

		{
			name:        "!",
			elementwise: true,
			fn: [numType]unaryFn{
				intType: func(c Context, v Value) Value {
					return BigInt{factorial(int64(v.(Int)))}.shrink()
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
					return BigInt{Int: bigInt64(0).Xor(v.(BigInt).Int, bigIntMinusOne.Int)}.shrink()
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
				complexType: func(c Context, v Value) Value {
					if isZero(v) {
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
				complexType: func(c Context, v Value) Value {
					return v.(Complex).abs(c).shrink()
				},
			},
		},

		{
			name:        "real",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      self,
				bigIntType:   self,
				bigRatType:   self,
				bigFloatType: self,
				complexType: func(c Context, v Value) Value {
					return v.(Complex).real
				},
			},
		},

		{
			name:        "imag",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      returnZero,
				bigIntType:   returnZero,
				bigRatType:   returnZero,
				bigFloatType: returnZero,
				complexType: func(c Context, v Value) Value {
					return v.(Complex).imag
				},
			},
		},

		{
			name:        "phase",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      realPhase,
				bigIntType:   realPhase,
				bigRatType:   realPhase,
				bigFloatType: realPhase,
				complexType: func(c Context, v Value) Value {
					return v.(Complex).phase(c).shrink()
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
						z.Add(z.Int, bigIntOne.Int)
						z.Neg(z.Int)
					}
					return z.shrink()
				},
				bigFloatType: func(c Context, v Value) Value {
					f := v.(BigFloat)
					if f.Float.IsInf() {
						Errorf("floor of %s", v.Sprint(c.Config()))
					}
					i, acc := f.Int(nil)
					switch acc {
					case big.Exact, big.Below:
						// Done.
					case big.Above:
						i.Sub(i, bigIntOne.Int)
					}
					return BigInt{i}.shrink()
				},
				complexType: func(c Context, v Value) Value {
					return v.(Complex).floor(c).shrink()
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
						z.Add(z.Int, bigIntOne.Int)
					} else {
						z.Neg(z.Int)
					}
					return z.shrink()
				},
				bigFloatType: func(c Context, v Value) Value {
					f := v.(BigFloat)
					if f.Float.IsInf() {
						Errorf("ceil of %s", v.Sprint(c.Config()))
					}
					i, acc := f.Int(nil)
					switch acc {
					case big.Exact, big.Above:
						// Done
					case big.Below:
						i.Add(i, bigIntOne.Int)
					}
					return BigInt{i}.shrink()
				},
				complexType: func(c Context, v Value) Value {
					return v.(Complex).ceil(c).shrink()
				},
			},
		},

		{
			name: "iota",
			fn: [numType]unaryFn{
				intType: func(c Context, v Value) Value {
					return newIota(c.Config().Origin(), int(v.(Int)))
				},
				vectorType: func(c Context, v Value) Value {
					// Produce a matrix of coordinates.
					vv := v.(*Vector)
					if vv.Len() == 0 {
						return empty
					}
					if vv.Len() == 1 {
						return newIota(c.Config().Origin(), vv.intAt(0, "iota argument"))
					}
					nElems := 1
					shape := make([]int, vv.Len())
					for i, coord := range vv.All() {
						c, ok := coord.(Int)
						if !ok || c < 0 || maxInt < c {
							Errorf("bad coordinate in iota %s", coord)
						}
						shape[i] = int(c)
						nElems *= int(c)
						if nElems > maxInt {
							Errorf("shape too large in iota %s", vv)
						}
					}
					origin := c.Config().Origin()
					elems := newVectorEditor(nElems, nil)
					counter := make([]int, vv.Len())
					for i := range counter {
						counter[i] = origin
					}
					for i := range nElems {
						elems.Set(i, NewIntVector(counter...))
						for axis := len(counter) - 1; axis >= 0; axis-- {
							counter[axis]++
							if counter[axis]-origin < shape[axis] {
								break
							}
							counter[axis] = origin
						}
					}
					return NewMatrix(shape, elems.Publish())
				},
			},
		},

		{
			name: "rho",
			fn: [numType]unaryFn{
				intType: func(c Context, v Value) Value {
					return empty
				},
				charType: func(c Context, v Value) Value {
					return empty
				},
				bigIntType: func(c Context, v Value) Value {
					return empty
				},
				bigRatType: func(c Context, v Value) Value {
					return empty
				},
				bigFloatType: func(c Context, v Value) Value {
					return empty
				},
				complexType: func(c Context, v Value) Value {
					return empty
				},
				vectorType: func(c Context, v Value) Value {
					return NewIntVector(v.(*Vector).Len())
				},
				matrixType: func(c Context, v Value) Value {
					return NewIntVector(v.(*Matrix).shape...)
				},
			},
		},

		{
			name: "count",
			fn: [numType]unaryFn{
				intType: func(c Context, v Value) Value {
					return one
				},
				charType: func(c Context, v Value) Value {
					return one
				},
				bigIntType: func(c Context, v Value) Value {
					return one
				},
				bigRatType: func(c Context, v Value) Value {
					return one
				},
				bigFloatType: func(c Context, v Value) Value {
					return one
				},
				complexType: func(c Context, v Value) Value {
					return one
				},
				vectorType: func(c Context, v Value) Value {
					return Int(v.(*Vector).Len())
				},
				matrixType: func(c Context, v Value) Value {
					m := v.(*Matrix)
					if len(m.shape) == 0 {
						return zero
					}
					return Int(m.shape[0])
				},
			},
		},

		{
			name: "where",
			fn: [numType]unaryFn{
				intType: func(c Context, v Value) Value {
					return empty
				},
				charType: func(c Context, v Value) Value {
					return empty
				},
				bigIntType: func(c Context, v Value) Value {
					return empty
				},
				bigRatType: func(c Context, v Value) Value {
					return empty
				},
				bigFloatType: func(c Context, v Value) Value {
					return empty
				},
				complexType: func(c Context, v Value) Value {
					return empty
				},
				vectorType: func(c Context, v Value) Value {
					vec := v.(*Vector)
					result := newVectorEditor(0, nil)
					origin := c.Config().Origin()
					for i := range vec.Len() {
						e := vec.uintAt(i, "where argument")
						for range e {
							result.Append(Int(origin + i))
						}
					}
					return result.Publish()
				},
				matrixType: func(c Context, v Value) Value {
					m := v.(*Matrix)
					shape, vec := m.shape, m.data
					result := newVectorEditor(0, nil)
					coords := make([]int, len(shape)) // Zero-indexed.
					origin := c.Config().Origin()
					// Loop over the data in the matrix while odometer-counting
					// the coordinates.
					for i := range vec.Len() {
						e := vec.uintAt(i, "where argument")
						for range e {
							c := newVectorEditor(len(shape), nil)
							for i, x := range coords {
								c.Set(i, Int(x+origin))
							}
							result.Append(c.Publish())
						}
						for j := len(coords) - 1; j >= 0; j-- {
							if coords[j]++; coords[j] < shape[j] {
								break
							}
							coords[j] = 0
						}
					}
					return result.Publish()
				},
			},
		},
		{
			name: "flatten",
			fn: [numType]unaryFn{
				intType: func(c Context, v Value) Value {
					return oneElemVector(v)
				},
				charType: func(c Context, v Value) Value {
					return oneElemVector(v)
				},
				bigIntType: func(c Context, v Value) Value {
					return oneElemVector(v)
				},
				bigRatType: func(c Context, v Value) Value {
					return oneElemVector(v)
				},
				bigFloatType: func(c Context, v Value) Value {
					return oneElemVector(v)
				},
				complexType: func(c Context, v Value) Value {
					return oneElemVector(v)
				},
				vectorType: func(c Context, v Value) Value {
					return NewVectorSeq(flatten(v.(*Vector)))
				},
				matrixType: func(c Context, v Value) Value {
					return c.EvalUnary("flatten", v.(*Matrix).data)
				},
			},
		},

		{
			name: "print",
			fn: [numType]unaryFn{
				intType:      printValue,
				charType:     printValue,
				bigIntType:   printValue,
				bigRatType:   printValue,
				bigFloatType: printValue,
				vectorType:   printValue,
				matrixType:   printValue,
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
				complexType:  vectorSelf,
				vectorType:   self,
				matrixType: func(c Context, v Value) Value {
					return v.(*Matrix).data
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
					return v.(*Vector).grade(c)
				},
				matrixType: func(c Context, v Value) Value {
					return v.(*Matrix).grade(c)
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
					return v.(*Vector).grade(c).reverse()
				},
				matrixType: func(c Context, v Value) Value {
					return v.(*Matrix).grade(c).reverse()
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
				complexType:  self,
				vectorType: func(c Context, v Value) Value {
					return v.(*Vector).reverse()
				},
				matrixType: func(c Context, v Value) Value {
					m := v.(*Matrix)
					if m.Rank() == 0 {
						return m
					}
					if m.Rank() == 1 {
						Errorf("rot: matrix is vector")
					}
					size := int(m.Size())
					ncols := m.shape[m.Rank()-1]
					x := m.data.edit()
					for index := 0; index <= size-ncols; index += ncols {
						for i, j := 0, ncols-1; i < j; i, j = i+1, j-1 {
							xi, xj := x.At(index+i), x.At(index+j)
							x.Set(index+i, xj)
							x.Set(index+j, xi)
						}
					}
					return &Matrix{shape: m.shape, data: x.Publish()}
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
				complexType:  self,
				vectorType: func(c Context, v Value) Value {
					return v.(*Vector).reverse()
				},
				matrixType: func(c Context, v Value) Value {
					m := v.(*Matrix)
					if m.Rank() == 0 {
						return m
					}
					if m.Rank() == 1 {
						Errorf("flip: matrix is vector")
					}
					elemSize := int(m.ElemSize())
					size := int(m.Size())
					x := m.data.edit()
					lo := 0
					hi := size - elemSize
					for lo < hi {
						for i := 0; i < elemSize; i++ {
							xl, xh := x.At(lo+i), x.At(hi+i)
							x.Set(lo+i, xh)
							x.Set(hi+i, xl)
						}
						lo += elemSize
						hi -= elemSize
					}
					return &Matrix{shape: m.shape, data: x.Publish()}
				},
			},
		},

		{
			name: "transp",
			fn: [numType]unaryFn{
				intType:      self,
				charType:     self,
				bigIntType:   self,
				bigRatType:   self,
				bigFloatType: self,
				complexType:  self,
				vectorType:   self,
				matrixType: func(c Context, v Value) Value {
					m := v.(*Matrix)
					if m.Rank() == 1 {
						Errorf("transp: matrix is vector")
					}
					return m.transpose(c)
				},
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
				complexType:  func(c Context, v Value) Value { return logn(c, v) },
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
				complexType:  func(c Context, v Value) Value { return cos(c, v) },
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
				complexType:  func(c Context, v Value) Value { return sin(c, v) },
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
				complexType:  func(c Context, v Value) Value { return tan(c, v) },
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
				complexType:  func(c Context, v Value) Value { return asin(c, v) },
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
				complexType:  func(c Context, v Value) Value { return acos(c, v) },
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
				complexType:  func(c Context, v Value) Value { return atan(c, v) },
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
				complexType:  func(c Context, v Value) Value { return exp(c, v) },
			},
		},

		{
			name:        "sinh",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      func(c Context, v Value) Value { return sinh(c, v) },
				bigIntType:   func(c Context, v Value) Value { return sinh(c, v) },
				bigRatType:   func(c Context, v Value) Value { return sinh(c, v) },
				bigFloatType: func(c Context, v Value) Value { return sinh(c, v) },
				complexType:  func(c Context, v Value) Value { return sinh(c, v) },
			},
		},

		{
			name:        "cosh",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      func(c Context, v Value) Value { return cosh(c, v) },
				bigIntType:   func(c Context, v Value) Value { return cosh(c, v) },
				bigRatType:   func(c Context, v Value) Value { return cosh(c, v) },
				bigFloatType: func(c Context, v Value) Value { return cosh(c, v) },
				complexType:  func(c Context, v Value) Value { return cosh(c, v) },
			},
		},

		{
			name:        "asinh",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      func(c Context, v Value) Value { return asinh(c, v) },
				bigIntType:   func(c Context, v Value) Value { return asinh(c, v) },
				bigRatType:   func(c Context, v Value) Value { return asinh(c, v) },
				bigFloatType: func(c Context, v Value) Value { return asinh(c, v) },
				complexType:  func(c Context, v Value) Value { return asinh(c, v) },
			},
		},

		{
			name:        "acosh",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      func(c Context, v Value) Value { return acosh(c, v) },
				bigIntType:   func(c Context, v Value) Value { return acosh(c, v) },
				bigRatType:   func(c Context, v Value) Value { return acosh(c, v) },
				bigFloatType: func(c Context, v Value) Value { return acosh(c, v) },
				complexType:  func(c Context, v Value) Value { return acosh(c, v) },
			},
		},

		{
			name:        "atanh",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      func(c Context, v Value) Value { return atanh(c, v) },
				bigIntType:   func(c Context, v Value) Value { return atanh(c, v) },
				bigRatType:   func(c Context, v Value) Value { return atanh(c, v) },
				bigFloatType: func(c Context, v Value) Value { return atanh(c, v) },
				complexType:  func(c Context, v Value) Value { return atanh(c, v) },
			},
		},

		{
			name:        "tanh",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      func(c Context, v Value) Value { return tanh(c, v) },
				bigIntType:   func(c Context, v Value) Value { return tanh(c, v) },
				bigRatType:   func(c Context, v Value) Value { return tanh(c, v) },
				bigFloatType: func(c Context, v Value) Value { return tanh(c, v) },
				complexType:  func(c Context, v Value) Value { return tanh(c, v) },
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
				complexType:  func(c Context, v Value) Value { return sqrt(c, v) },
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
				complexType:  func(c Context, v Value) Value { return text(c, v) },
				vectorType:   func(c Context, v Value) Value { return text(c, v) },
				matrixType:   func(c Context, v Value) Value { return text(c, v) },
			},
		},

		{
			name: "ivy",
			fn: [numType]unaryFn{
				charType: func(c Context, v Value) Value {
					char := v.(Char)
					return IvyEval(c, string(char))
				},
				vectorType: func(c Context, v Value) Value {
					text := v.(*Vector)
					if !text.AllChars() {
						Errorf("ivy: value is not a vector of char")
					}
					return IvyEval(c, text.Sprint(c.Config()))
				},
			},
		},

		{
			name:        "float",
			elementwise: true,
			fn: [numType]unaryFn{
				intType:      floatValueSelf,
				bigIntType:   floatValueSelf,
				bigRatType:   floatValueSelf,
				bigFloatType: floatValueSelf,
				complexType: func(c Context, v Value) Value {
					u := v.(Complex)
					return NewComplex(floatValueSelf(c, u.real), floatValueSelf(c, u.imag))
				},
			},
		},

		{
			name:        "unique",
			elementwise: false,
			fn: [numType]unaryFn{
				intType:      unique,
				charType:     unique,
				bigIntType:   unique,
				bigRatType:   unique,
				bigFloatType: unique,
				complexType:  unique,
				vectorType:   unique,
			},
		},

		{
			name:        "box",
			elementwise: false,
			fn: [numType]unaryFn{
				intType:      box,
				charType:     box,
				bigIntType:   box,
				bigRatType:   box,
				bigFloatType: box,
				complexType:  box,
				vectorType:   box,
				matrixType:   box,
			},
		},

		{
			name:        "first",
			elementwise: false,
			fn: [numType]unaryFn{
				intType:      self,
				charType:     self,
				bigIntType:   self,
				bigRatType:   self,
				bigFloatType: self,
				complexType:  self,
				vectorType: func(c Context, v Value) Value {
					u := v.(*Vector)
					if u.Len() == 0 {
						return zero // TODO: If we add prototypes, this is a place it matters.
					}
					return u.At(0)
				},
				matrixType: func(c Context, v Value) Value {
					u := v.(*Matrix).data
					if u.Len() == 0 {
						return zero // TODO: If we add prototypes, this is a place it matters.
					}
					return u.At(0)
				},
			},
		},

		{
			name:        "sys",
			elementwise: false,
			fn: [numType]unaryFn{
				vectorType: sys, // Expect a vector of chars.
			},
		},
	}

	for _, op := range ops {
		UnaryOps[op.name] = op
	}
}
