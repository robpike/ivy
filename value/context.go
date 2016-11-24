// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "robpike.io/ivy/config"

// Expr and Context are defined here to avoid import cycles
// between parse and value.

// Expr is the interface for a parsed expression.
type Expr interface {
	// ProgString returns the unambiguous representation of the
	// expression to be used in program source.
	ProgString() string

	Eval(Context) Value
}

// UnaryOp is the interface implemented by a simple unary operator.
type UnaryOp interface {
	EvalUnary(c Context, right Value) Value
}

// BinaryOp is the interface implemented by a simple binary operator.
type BinaryOp interface {
	EvalBinary(c Context, right, left Value) Value
}

// Context is the execution context for evaluation.
// The only implementation is ../exec/Context, but the interface
// is defined separately, here, because of the dependence on Expr
// and the import cycle that would otherwise result.
type Context interface {
	// Lookup returns the configuration state for evaluation.
	Config() *config.Config

	// Lookup returns the value of a symbol.
	Lookup(name string) Value

	// Assign assigns the variable the value. The variable must
	// be defined either in the current function or globally.
	// Inside a function, new variables become locals.
	Assign(name string, value Value)

	// Eval evaluates a list of expressions.
	Eval(exprs []Expr) []Value

	// EvalUnaryFn evaluates a unary operator.
	EvalUnary(op string, right Value) Value

	// EvalBinary evaluates a binary operator.
	EvalBinary(left Value, op string, right Value) Value

	// UserDefined reports whether the specified op is user-defined.
	UserDefined(op string, isBinary bool) bool
}
