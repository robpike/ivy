// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

// Unary operators.

// To avoid initialization cycles when we refer to the ops from inside
// themselves, we use an init function to initialize the ops.

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
	unaryPlus, unaryMinus, unaryBitwiseNot *unaryOp
	unaryLogicalNot, unaryIota             *unaryOp
	unaryOps                               map[string]*unaryOp
)

func init() {
	unaryPlus = &unaryOp{
		fn: [numType]unaryFn{
			func(v Value) Value { return v },
			func(v Value) Value { return v },
			func(v Value) Value { return v },
		},
	}

	unaryMinus = &unaryOp{
		fn: [numType]unaryFn{
			func(v Value) Value {
				i := v.(Int)
				i.x = -i.x
				return i
			},
			func(v Value) Value {
				i := v.(BigInt)
				i.x.Neg(&i.x)
				return i
			},
			func(v Value) Value {
				return unaryVectorOp("-", v)
			},
		},
	}

	unaryBitwiseNot = &unaryOp{
		fn: [numType]unaryFn{
			func(v Value) Value {
				i := v.(Int)
				i.x = ^i.x
				return i
			},
			func(v Value) Value {
				// Lots of ways to do this, here's one.
				i := v.(BigInt)
				var z BigInt
				z.x.Xor(&i.x, &bigMinusOne.x)
				return z
			},
			func(v Value) Value {
				return unaryVectorOp("^", v)
			},
		},
	}

	unaryLogicalNot = &unaryOp{
		fn: [numType]unaryFn{
			func(v Value) Value {
				if v.(Int).x == 0 {
					return one
				}
				return zero
			},
			func(v Value) Value {
				i := v.(BigInt)
				if i.x.Sign() == 0 {
					return one
				}
				return zero
			},
			func(v Value) Value {
				return unaryVectorOp("!", v)
			},
		},
	}

	unaryIota = &unaryOp{
		fn: [numType]unaryFn{
			func(v Value) Value {
				i := v.(Int)
				if i.x <= 0 || maxInt < i.x {
					panic(Errorf("bad iota %d)", i.x))
				}
				n := make([]Value, i.x)
				for k := range n {
					n[k] = Int{x: int64(k) + 1}
				}
				return ValueSlice(n)
			},
			func(v Value) Value { panic(Error("no iota for big int")) },
			func(v Value) Value { panic(Error("no iota for vector")) },
		},
	}

	unaryOps = map[string]*unaryOp{
		"+":    unaryPlus,
		"-":    unaryMinus,
		"^":    unaryBitwiseNot,
		"!":    unaryLogicalNot,
		"iota": unaryIota,
	}
}
