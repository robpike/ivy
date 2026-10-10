// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"
	"strings"
)

// Operator is the most general interface that represents an operator, unary or binary.
type Operator interface {
	String() string
	ProgString() string
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

func (o BinaryBuiltinOp) ProgString() string { return string(o) }

func (o BinaryBuiltinOp) EvalBinary(c Context, left, right Value) Value {
	TraceBinary(c, 1, left, o.String(), right)
	builtin := BinaryBuiltins[o.String()]
	if builtin == nil {
		c.Errorf("builtin binary %q not implemented", o.String())
	}
	return builtin.EvalBinary(c, left, right)
}

// UnaryBuiltinOp is the interface that represents all unary builtins.
type UnaryBuiltinOp string

func (o UnaryBuiltinOp) String() string { return string(o) }

func (o UnaryBuiltinOp) ProgString() string { return string(o) }

func (o UnaryBuiltinOp) EvalUnary(c Context, right Value) Value {
	TraceUnary(c, 1, o.String(), right)
	builtin := UnaryBuiltins[o.String()]
	if builtin == nil {
		c.Errorf("builtin unary %q not implemented", o.String())
	}
	return builtin.EvalUnary(c, right)
}

// Some predefined binary ops used in the code.
var (
	plus   = BinaryBuiltinOp("+")
	minus  = BinaryBuiltinOp("-")
	times  = BinaryBuiltinOp("*")
	divide = BinaryBuiltinOp("/")
	pow    = BinaryBuiltinOp("**")
	bmod   = BinaryBuiltinOp("mod")

	EQ  = BinaryBuiltinOp("==")
	NE  = BinaryBuiltinOp("!=")
	LT  = BinaryBuiltinOp("<")
	LEQ = BinaryBuiltinOp("<=")
	GT  = BinaryBuiltinOp(">")
	GEQ = BinaryBuiltinOp(">=")
)

// Some predefined unary ops used in the code.
var (
	uplus  = UnaryBuiltinOp("+")
	uminus = UnaryBuiltinOp("-")
	urecip = UnaryBuiltinOp("/")
	ufloor = UnaryBuiltinOp("floor")
	usgn   = UnaryBuiltinOp("sgn")
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

func (o *ReduceOp) ProgString() string {
	slash := "/"
	if o.isScan {
		slash = "\\"
	}
	if o.firstAxis {
		slash += "%"
	}
	return fmt.Sprintf("%s%s", o.op.ProgString(), slash)
}

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

func (o *OuterOp) ProgString() string {
	return "o." + o.op.ProgString()
}

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

func (o *InnerOp) ProgString() string {
	return o.leftOp.ProgString() + "." + o.rightOp.ProgString()
}

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

func (o *UnaryEachOp) ProgString() string { return o.ProgString() + ats(o.numAts) }

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

func (o *BinaryEachOp) ProgString() string {
	return ats(o.nLeftAts) + o.ProgString() + ats(o.nRightAts)
}

func (o *BinaryEachOp) EvalBinary(c Context, left, right Value) Value {
	TraceBinary(c, 2, left, o.String(), right)
	return BinaryEach(c, left, o.nLeftAts, o.op, o.nRightAts, right)
}

// UnaryNonceOp represents an in-line unary operator construct.
type UnaryNonceOp struct {
	str   string
	right Expr
	body  StatementList
	op    Unary
}

func NewUnaryNonceOp(c Context, right Expr, body StatementList, source string) *UnaryNonceOp {
	return &UnaryNonceOp{
		str:   source,
		op:    c.NewNonceOp(false, nil, right, body, source).(Unary),
		right: right,
		body:  body,
	}
}

func (o *UnaryNonceOp) String() string { return o.str }

func (o *UnaryNonceOp) ProgString() string {
	return fmt.Sprintf("{op %s = %s}", o.right.ProgString(), o.body.ProgString())
}

func (o *UnaryNonceOp) EvalUnary(c Context, right Value) Value {
	TraceUnary(c, 2, o.String(), right)
	return o.op.EvalUnary(c, right)
}

// BinaryNonceOp represents an in-line unary operator construct.
type BinaryNonceOp struct {
	str   string
	op    Binary
	left  Expr
	right Expr
	body  StatementList
}

func NewBinaryNonceOp(c Context, left, right Expr, body StatementList, source string) *BinaryNonceOp {
	return &BinaryNonceOp{
		str:   source,
		op:    c.NewNonceOp(true, left, right, body, source).(Binary),
		left:  left,
		right: right,
		body:  body,
	}
}

func (o *BinaryNonceOp) String() string { return o.str }

func (o *BinaryNonceOp) ProgString() string {
	return fmt.Sprintf("{%s op %s = %s}", o.left.ProgString(), o.right.ProgString(), o.body.ProgString())
}

func (o *BinaryNonceOp) EvalBinary(c Context, left, right Value) Value {
	TraceBinary(c, 2, left, o.String(), right)
	return o.op.EvalBinary(c, left, right)
}

// OpExpr holds an operator value. It evaluates to itself and satisfies both Expr and Value.
type OpExpr struct {
	isBinary bool
	unary    Unary
	binary   Binary
}

func NewOpExpr(unary Unary, binary Binary) *OpExpr {
	return &OpExpr{
		unary:  unary,
		binary: binary,
	}
}

func (o *OpExpr) HasUnary() bool  { return o.unary != nil }
func (o *OpExpr) HasBinary() bool { return o.binary != nil }

// Unpack unpacks an OpExpr and explains what it holds.
// For operators such as (+), both unary and binary may exist.
func (o *OpExpr) Unpack() (Unary, Binary) {
	return o.unary, o.binary
}

func (o *OpExpr) ProgString() string {
	var op Operator = o.binary
	if op == nil {
		op = o.unary
	}
	switch op.(type) {
	case *UnaryNonceOp, *BinaryNonceOp:
		return op.ProgString()
	case *OpExpr:
		return op.ProgString()
	}
	// Wrapping a builtin or name op; add parens.
	return "(" + op.String() + ")"
}

func (o *OpExpr) Eval(c Context) Value {
	return o
}

func (o *OpExpr) String() string {
	var op Operator = o.binary
	if op == nil {
		op = o.unary
	}
	switch op.(type) {
	case *UnaryNonceOp, *BinaryNonceOp:
		return op.String()
	case *OpExpr:
		return op.String()
	}
	// Wrapping a builtin or name op; add parens.
	return "(" + op.String() + ")"
}

func (o *OpExpr) Sprint(c Context) string {
	return o.String() // Ignores context, but many types do.
}

func (o *OpExpr) Inner() Value {
	return o
}

func (o *OpExpr) Rank() int {
	return 0
}

func (o *OpExpr) shrink() Value {
	return o
}

func (o *OpExpr) toType(op string, c Context, which valueType) Value {
	switch which {
	case operatorType:
		return o
	case vectorType:
		return oneElemVector(o)
	case matrixType:
		return NewMatrix(c, []int{1, 1}, NewVector(o))
	}
	c.Errorf("%s: cannot convert operator to %s", op, which)
	return nil
}

// HasOpExprs reports whether the expression contains any operator
// expressions. If it does, we must flush parse state.
func HasOpExprs(v Expr) bool {
	switch v := v.(type) {
	case *OpExpr:
		return true
	case VectorExpr:
		for _, e := range v {
			if HasOpExprs(e) {
				return true
			}
		}
	}
	return false
}
