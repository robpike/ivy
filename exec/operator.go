// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

import (
	"robpike.io/ivy/value"
)

// Predefined reports whether the operator is predefined, a built-in.
func Predefined(op string) bool {
	return value.BinaryOps[op] != nil || value.UnaryOps[op] != nil
}

// DefinedOp reports whether the operator is known.
func (c *Context) DefinedOp(op string) bool {
	if c.isVariable(op) {
		return false
	}
	return Predefined(op) || c.BinaryFn[op] != nil || c.UnaryFn[op] != nil
}

// DefinedBinary reports whether the operator is a known binary.
func (c *Context) DefinedBinary(op string) bool {
	if c.isVariable(op) {
		return false
	}
	return c.BinaryFn[op] != nil || value.BinaryOps[op] != nil
}

// DefinedUnary reports whether the operator is a known unary.
func (c *Context) DefinedUnary(op string) bool {
	if c.isVariable(op) {
		return false
	}
	return c.UnaryFn[op] != nil || value.UnaryOps[op] != nil
}
