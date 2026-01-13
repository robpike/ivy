// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

import "robpike.io/ivy/value"

// DefinedOp reports whether the operator is known.
func (c *Context) DefinedOp(op string) bool {
	if value.BinaryOps[op] != nil || value.UnaryOps[op] != nil {
		return true
	}
	return c.BinaryFn[op] != nil || c.UnaryFn[op] != nil
}

// DefinedBinary reports whether the operator is a known binary.
func (c *Context) DefinedBinary(op string) bool {
	return c.BinaryFn[op] != nil || value.BinaryOps[op] != nil
}

// DefinedUnary reports whether the operator is a known unary.
func (c *Context) DefinedUnary(op string) bool {
	return c.UnaryFn[op] != nil || value.UnaryOps[op] != nil
}
