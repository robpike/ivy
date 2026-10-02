// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"strings"
)

// Operator is the most general interface that represents an operator, unary or binary.
type Operator interface {
	String() string
}

// Unary represents an operator with one argument.
type Unary interface {
	Operator
	EvalUnary(c Context, right Value) Value
}

// Binary represents an operator with two arguments.
type Binary interface {
	Operator
	EvalBinary(c Context, left, right Value) Value
}

// BinaryBuiltinOp is the interface that represents all binary builtins.
type BinaryBuiltinOp string

func (o BinaryBuiltinOp) String() string { return string(o) }

func (o BinaryBuiltinOp) EvalBinary(c Context, left, right Value) Value {
	TraceBinary(c, 1, left, o.String(), right)
	builtin := BinaryBuiltins[o.String()]
	if builtin == nil {
		c.Errorf("builtin binary %q not implemented", o.String())
	}
	return builtin.EvalBinary(c, left, right)
}

// BOP wraps a string into a BinaryBuiltinOp.
// Removes clutter compared to a declaration.
func BOP(s string) BinaryBuiltinOp { return BinaryBuiltinOp(s) }

// UnaryBuiltinOp is the interface that represents all unary builtins.
type UnaryBuiltinOp string

func (o UnaryBuiltinOp) String() string { return string(o) }

func (o UnaryBuiltinOp) EvalUnary(c Context, right Value) Value {
	TraceUnary(c, 1, o.String(), right)
	builtin := UnaryBuiltins[o.String()]
	if builtin == nil {
		c.Errorf("builtin unary %q not implemented", o.String())
	}
	return builtin.EvalUnary(c, right)
}

// UOP wraps a string into a UnaryBuiltinOp.
// Removes clutter compared to a declaration.
func UOP(s string) UnaryBuiltinOp { return UnaryBuiltinOp(s) }

// Some predefined binary ops used in the code.
var (
	plus   = BOP("+")
	minus  = BOP("-")
	times  = BOP("*")
	divide = BOP("/")
	pow    = BOP("**")
	bmod   = BOP("mod")

	EQ  = BOP("==")
	NE  = BOP("!=")
	LT  = BOP("<")
	LEQ = BOP("<=")
	GT  = BOP(">")
	GEQ = BOP(">=")
)

// Some predefined unary ops used in the code.
var (
	uplus  = UOP("+")
	uminus = UOP("-")
	urecip = UOP("/")
	ufloor = UOP("floor")
	usgn   = UOP("sgn")
)

// ReduceOp represents a reduction or scan.
type ReduceOp struct {
	str       string
	op        Binary
	isScan    bool
	firstAxis bool // If true, the operator iterates on the first axis; default is the last.
}

func NewReduceOp(op Binary, decorator string) *ReduceOp {
	return &ReduceOp{
		str:       op.String() + decorator,
		op:        op,
		isScan:    strings.HasPrefix(decorator, "\\"),
		firstAxis: strings.HasSuffix(decorator, "%"),
	}
}

func (o *ReduceOp) String() string { return o.str }

func (o *ReduceOp) EvalUnary(c Context, right Value) Value {
	TraceUnary(c, 2, o.String(), right)
	if o.isScan {
		if o.firstAxis {
			return ScanFirst(c, o.op, right)
		}
		return Scan(c, o.op, right)
	}
	if o.firstAxis {
		return ReduceFirst(c, o.op, right)
	}
	return Reduce(c, o.op, right)
}

// OuterOp represents an outer product.
type OuterOp struct {
	str string
	op  Binary
}

func NewOuterOp(op Binary) *OuterOp {
	return &OuterOp{
		str: "o." + op.String(),
		op:  op,
	}
}

func (o *OuterOp) String() string { return o.str }

func (o *OuterOp) EvalBinary(c Context, left, right Value) Value {
	TraceBinary(c, 2, left, o.String(), right)
	return outerProduct(c, left, o.op, right)
}

// InnerOp represents an inner product.
type InnerOp struct {
	str     string
	leftOp  Binary
	rightOp Binary
}

func NewInnerOp(leftOp, rightOp Binary) *InnerOp {
	return &InnerOp{
		str:     leftOp.String() + "." + rightOp.String(),
		leftOp:  leftOp,
		rightOp: rightOp,
	}
}

func (o *InnerOp) String() string { return o.str }

func (o *InnerOp) EvalBinary(c Context, left, right Value) Value {
	TraceBinary(c, 2, left, o.String(), right)
	return innerProduct(c, left, o.leftOp, o.rightOp, right)
}

// atString and ats are used to avoid an allocation when building strings for Each ops.
var atString = "@@@@"

func ats(n int) string {
	if len(atString) < n {
		atString = strings.Repeat("@", n+10)
	}
	return atString[:n]
}

// UnaryEachOp represents a unary operator with trailing @s.
type UnaryEachOp struct {
	str    string
	op     Unary
	numAts int
}

func NewUnaryEachOp(op Unary, numAts int) *UnaryEachOp {
	return &UnaryEachOp{
		str:    op.String() + ats(numAts),
		op:     op,
		numAts: numAts,
	}
}

func (o *UnaryEachOp) String() string { return o.str }

func (o *UnaryEachOp) EvalUnary(c Context, right Value) Value {
	TraceUnary(c, 2, o.String(), right)
	return UnaryEach(c, o.op, o.numAts, right)
}

// BinaryEachOp represents a binary operator with leading and/or trailing @s.
type BinaryEachOp struct {
	str       string
	op        Binary
	nLeftAts  int
	nRightAts int
}

func NewBinaryEachOp(op Binary, nLeftAts, nRightAts int) *BinaryEachOp {
	return &BinaryEachOp{
		str:       ats(nLeftAts) + op.String() + ats(nRightAts),
		op:        op,
		nLeftAts:  nLeftAts,
		nRightAts: nRightAts,
	}
}

func (o *BinaryEachOp) String() string { return o.str }

func (o *BinaryEachOp) EvalBinary(c Context, left, right Value) Value {
	TraceBinary(c, 2, left, o.String(), right)
	return BinaryEach(c, left, o.nLeftAts, o.op, o.nRightAts, right)
}
