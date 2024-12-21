// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "robpike.io/ivy/config"

// Decomposable allows one to pull apart a parsed expression.
// Only implemented by Expr types that need to be decomposed
// in function evaluation.
type Decomposable interface {
	// Operator returns the operator, or "" for a singleton.
	Operator() string

	// Operands returns the left and right operands, or nil if absent.
	// For singletons, both will be nil, but ProgString can
	// give the underlying name or value.
	Operands() (left, right Expr)
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
	// Config returns the configuration state for evaluation.
	Config() *config.Config

	// Local returns the value of the i'th local variable.
	Local(i int) Value

	// AssignLocal assigns to the i'th local variable.
	AssignLocal(i int, value Value)

	// Global returns the value of the named global variable.
	Global(name string) Value

	// AssignGlobal assigns to the named global variable.
	AssignGlobal(name string, value Value)

	// Eval evaluates a list of expressions.
	Eval(exprs []Expr) []Value

	// EvalUnary evaluates a unary operator.
	EvalUnary(op string, right Value) Value

	// EvalBinary evaluates a binary operator.
	EvalBinary(left Value, op string, right Value) Value

	// UserDefined reports whether the specified op is user-defined.
	UserDefined(op string, isBinary bool) bool
}
