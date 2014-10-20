// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"
	"log"
	"strconv"
)

// Int is not only the simplest representation, it provides the operands that mix
// types upward. That is, BigInt.Add(Int) will be done by rewriting as Int.Add(BigInt).

type Int struct {
	unimplemented
	x int64
}

const (
	intBits = 8
	minInt  = -256
	maxInt  = 255
)

func SetInt(s string) (Int, ParseState) {
	i, err := strconv.ParseInt(s, 0, intBits)
	if err == nil {
		return Int{x: i}, Valid
	}
	if err, ok := err.(*strconv.NumError); ok && err.Err == strconv.ErrRange {
		return Int{}, Retry
	}
	log.Print(err)
	return Int{}, Fail
}

func (i Int) String() string {
	return fmt.Sprint(i.x)
}

func (i Int) set(res int64) Value {
	if minInt <= res && res <= maxInt {
		i.x = res
		return i
	}
	var z BigInt
	z.x.SetInt64(res)
	return z
}

func (i Int) Add(x Value) Value {
	switch x := x.(type) {
	case Int:
		return i.set(i.x + x.x)
	case BigInt:
		var z BigInt
		z.x.SetInt64(i.x)
		z.x = *z.x.Add(&z.x, &x.x)
		return z
	}
	panic(Errorf("unimplemented Add(Int, %T)", x))
}

func (i Int) Sub(x Value) Value {
	switch x := x.(type) {
	case Int:
		return i.set(i.x - x.x)
	case BigInt:
		var z BigInt
		z.x.SetInt64(i.x)
		z.x = *z.x.Sub(&z.x, &x.x)
		return z
	}
	panic(Errorf("unimplemented Sub(Int, %T)", x))
}

func (i Int) Mul(x Value) Value {
	switch x := x.(type) {
	case Int:
		return i.set(i.x * x.x)
	case BigInt:
		var z BigInt
		z.x.SetInt64(i.x)
		z.x = *z.x.Mul(&z.x, &x.x)
		return z
	}
	panic(Errorf("unimplemented Mul(Int, %T)", x))
}

func (i Int) Div(x Value) Value {
	switch x := x.(type) {
	case Int:
		if x.x == 0 {
			panic(Error("division by zero"))
		}
		return i.set(i.x / x.x)
	case BigInt:
		var z BigInt
		z.x.SetInt64(i.x)
		z.x = *z.x.Div(&z.x, &x.x)
		return z
	}
	panic(Errorf("unimplemented Div(Int, %T)", x))
}
