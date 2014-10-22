// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "fmt"

type Expr interface {
	String() string

	Eval() Value
}

type Value interface {
	String() string
	Eval() Value

	ToType(valueType) Value
}

type Error string

func (err Error) Error() string {
	return string(err)
}

func Errorf(format string, args ...interface{}) Error {
	return Error(fmt.Sprintf("ivy: "+format, args...))
}

type ParseState int

const (
	Valid ParseState = iota
	Retry
	Fail
)

func ValueString(s string) (Value, bool) {
	// start small
	i, state := SetIntString(s)
	if state != Retry {
		return i, true
	}
	b, state := SetBigIntString(s)
	return b, state == Valid
}

func valueInt64(x int64) Value {
	if minInt <= x && x <= maxInt {
		return Int{x: x}
	}
	return bigInt64(x)
}

func bigInt64(x int64) BigInt {
	var z BigInt
	z.x.SetInt64(x)
	return z
}
