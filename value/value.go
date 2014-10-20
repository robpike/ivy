// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "fmt"

type Value interface {
	String() string

	// Operators, all of the form Value += operand
	Add(Value) Value
	Sub(Value) Value
	Mul(Value) Value
	Div(Value) Value
}

type Error string

func (err Error) Error() string {
	return string(err)
}

func Errorf(format string, args ...interface{}) Error {
	return Error(fmt.Sprintf("ivy: "+format, args...))
}

// The unimplemented type provides a failing implementation for every operation.
// This makes it easy to bootstrap by embedding.

type unimplemented struct{}

func (unimplemented) String() string {
	panic("String unimplemented")
}

func (unimplemented) Copy() Value {
	panic("Copy unimplemented")
}

func (unimplemented) Add(Value) Value {
	panic("Add unimplemented")
}

func (unimplemented) Sub(Value) Value {
	panic("Sub unimplemented")
}

func (unimplemented) Mul(Value) Value {
	panic("Mul unimplemented")
}

func (unimplemented) Div(Value) Value {
	panic("Div unimplemented")
}

type ParseState int

const (
	Valid ParseState = iota
	Retry
	Fail
)

func Set(s string) (Value, bool) {
	// start small
	i, state := SetInt(s)
	if state != Retry {
		return i, true
	}
	b, state := SetBigInt(s)
	return b, state == Valid
}
