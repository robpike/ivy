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
	return Error(fmt.Sprintf(format, args...))
}

type ParseState int

func ValueString(s string) (Value, error) {
	// start small
	i, err := SetIntString(s)
	if err == nil {
		return i, nil
	}
	b, err := SetBigIntString(s)
	if err == nil {
		return b.shrink(), nil
	}
	r, err := SetBigRatString(s)
	if err == nil {
		return r.shrink(), nil
	}
	return nil, err
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

func bigRatInt64(x int64) BigRat {
	var z BigRat
	z.x.SetInt64(x)
	return z
}
