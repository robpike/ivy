// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

type valueType int

const (
	intType valueType = iota
	bigIntType
	vectorType
	numType
)

type unaryFn func(Value) Value

type unaryOp struct {
	fn [numType]unaryFn
}

func Unary(opName string, v Value) Value {
	op := unaryOps[opName]
	return op.fn[whichType(v)](v)
}

type binaryFn func(Value, Value) Value

type binaryOp struct {
	whichType func(a, b valueType) valueType
	fn        [numType]binaryFn
}

func whichType(v Value) valueType {
	switch v.(type) {
	case Int:
		return intType
	case BigInt:
		return bigIntType
	case Vector:
		return vectorType
	}
	panic("which type")
}

func Binary(v1 Value, opName string, v2 Value) Value {
	op := binaryOps[opName]
	which := op.whichType(whichType(v1), whichType(v2))
	return op.fn[which](v1.ToType(which), v2.ToType(which))
}

// Unary operators.

// To aovid initialization cycles when we refer to the ops from inside
// themselves, we use an init function to initialize the ops.

var (
	unaryPlus, unaryMinus, unaryIota *unaryOp
	unaryOps                         map[string]*unaryOp
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
				vv := v.(Vector)
				for i, x := range vv.x {
					vv.x[i] = Unary("-", x)
				}
				return vv
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
		"iota": unaryIota,
	}
}

// Binary operators.

func binaryArithType(t1, t2 valueType) valueType {
	if t1 > t2 {
		return t1
	}
	return t2
}

func powType(t1, t2 valueType) valueType {
	if t1 == intType {
		t1 = bigIntType
	}
	return binaryArithType(t1, t2)
}

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
		reduced := count.reduce()
		if _, ok := reduced.(Int); ok {
			return shiftCount(reduced)
		}
	}
	panic(Error("illegal shift count type"))
}

func binaryVectorOp(u Vector, op string, v Vector) Value {
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

var (
	add, sub, mul, div, pow, and, or, xor, lsh, rsh *binaryOp
	binaryOps                                       map[string]*binaryOp
)

func init() {
	add = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				i, j := u.(Int), v.(Int)
				return valueInt64(i.x + j.x)
			},
			func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				var z BigInt
				z.x.Add(&i.x, &j.x)
				return z.reduce()
			},
			func(u, v Value) Value {
				return binaryVectorOp(u.(Vector), "+", v.(Vector))
			},
		},
	}

	sub = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				i, j := u.(Int), v.(Int)
				return valueInt64(i.x - j.x)
			},
			func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				var z BigInt
				z.x.Sub(&i.x, &j.x)
				return z.reduce()
			},
			func(u, v Value) Value {
				return binaryVectorOp(u.(Vector), "-", v.(Vector))
			},
		},
	}

	mul = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				i, j := u.(Int), v.(Int)
				return valueInt64(i.x * j.x)
			},
			func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				var z BigInt
				z.x.Mul(&i.x, &j.x)
				return z.reduce()
			},
			func(u, v Value) Value {
				return binaryVectorOp(u.(Vector), "*", v.(Vector))
			},
		},
	}

	div = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				i, j := u.(Int), v.(Int)
				return valueInt64(i.x / j.x)
			},
			func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				var z BigInt
				z.x.Div(&i.x, &j.x)
				return z.reduce()
			},
			func(u, v Value) Value {
				return binaryVectorOp(u.(Vector), "/", v.(Vector))
			},
		},
	}

	pow = &binaryOp{
		whichType: powType,
		fn: [numType]binaryFn{
			nil, // Use BigInt for this.
			func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				var z BigInt
				z.x.Exp(&i.x, &j.x, nil)
				return z.reduce()
			},
			func(u, v Value) Value {
				return binaryVectorOp(u.(Vector), "**", v.(Vector))
			},
		},
	}

	and = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				i, j := u.(Int), v.(Int)
				return valueInt64(i.x & j.x)
			},
			func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				var z BigInt
				z.x.And(&i.x, &j.x)
				return z.reduce()
			},
			func(u, v Value) Value {
				return binaryVectorOp(u.(Vector), "&", v.(Vector))
			},
		},
	}

	or = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				i, j := u.(Int), v.(Int)
				return valueInt64(i.x | j.x)
			},
			func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				var z BigInt
				z.x.Or(&i.x, &j.x)
				return z.reduce()
			},
			func(u, v Value) Value {
				return binaryVectorOp(u.(Vector), "|", v.(Vector))
			},
		},
	}

	xor = &binaryOp{
		whichType: binaryArithType,
		fn: [numType]binaryFn{
			func(u, v Value) Value {
				i, j := u.(Int), v.(Int)
				return valueInt64(i.x ^ j.x)
			},
			func(u, v Value) Value {
				i, j := u.(BigInt), v.(BigInt)
				var z BigInt
				z.x.Xor(&i.x, &j.x)
				return z.reduce()
			},
			func(u, v Value) Value {
				return binaryVectorOp(u.(Vector), "^", v.(Vector))
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
				return z.reduce()
			},
			func(u, v Value) Value {
				return binaryVectorOp(u.(Vector), "<<", v.(Vector))
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
				return z.reduce()
			},
			func(u, v Value) Value {
				return binaryVectorOp(u.(Vector), ">>", v.(Vector))
			},
		},
	}

	binaryOps = map[string]*binaryOp{
		"+":  add,
		"-":  sub,
		"*":  mul,
		"/":  div,
		"**": pow,
		"&":  and,
		"|":  or,
		"^":  xor,
		"<<": lsh,
		">>": rsh,
	}
}
