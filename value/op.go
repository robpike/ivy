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

type initialized interface {
	InitString() string
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

func (o *UnaryEachOp) ProgString() string { return o.op.ProgString() + ats(o.numAts) }

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
	return ats(o.nLeftAts) + o.op.ProgString() + ats(o.nRightAts)
}

func (o *BinaryEachOp) EvalBinary(c Context, left, right Value) Value {
	TraceBinary(c, 2, left, o.String(), right)
	return BinaryEach(c, left, o.nLeftAts, o.op, o.nRightAts, right)
}

// UnaryNonceOp represents an in-line unary operator construct.
type UnaryNonceOp struct {
	str          string
	op           Unary
	right        Expr
	body         StatementList
	initializers []*Var
	initString   string
}

func NewUnaryNonceOp(c Context, right Expr, body StatementList, source string) *UnaryNonceOp {
	return &UnaryNonceOp{
		str:   source,
		op:    c.NewNonceOp(false, nil, right, body, source).(Unary),
		right: right,
		body:  body,
	}
}

func (o *UnaryNonceOp) flush() {
	FlushState(o.body)
}

func (o *UnaryNonceOp) String() string { return o.str }

func (o *UnaryNonceOp) ProgString() string {
	return fmt.Sprintf("{op %s = %s}", o.right.ProgString(), o.body.ProgString())
}

func (o *UnaryNonceOp) Initializers() []*Var {
	return o.initializers
}

func (o *UnaryNonceOp) bindInitializers(c Context, locals map[string]Value) {
	if len(locals) == 0 {
		return
	}
	o.initializers = toVarSlice(locals)
	o.initString = toString(locals)
	c.BindInitializers(o.op, o.initializers)
}

func (o *UnaryNonceOp) InitString() string {
	return o.initString
}

// toVarSlices creates a slice of *Var from a map of names and values.
func toVarSlice(locals map[string]Value) (slice []*Var) {
	for name, val := range locals {
		slice = append(slice, NewVar(name, val, LocalVar))
	}
	return slice
}

// toString creates a string representation of the initializers.
// If the value is a string, it is printed as $1324 to avoid
// accidentally adding a value that could alias an assignment.
func toString(locals map[string]Value) string {
	var b strings.Builder
	for name, val := range locals {
		fmt.Fprintf(&b, "%s=", name)
		if vec, ok := val.(*Vector); ok && vec.AllChars() {
			fmt.Fprintf(&b, "$%x;", vec)
		} else {
			fmt.Fprintf(&b, "%s;", val.ProgString())
		}
	}
	return b.String()
}

func (o *UnaryNonceOp) EvalUnary(c Context, right Value) Value {
	TraceUnary(c, 2, o.String(), right)
	return o.op.EvalUnary(c, right)
}

// BinaryNonceOp represents an in-line unary operator construct.
type BinaryNonceOp struct {
	str          string
	op           Binary
	left         Expr
	right        Expr
	body         StatementList
	initializers []*Var
	initString   string
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

func (o *BinaryNonceOp) flush() {
	FlushState(o.body)
}

func (o *BinaryNonceOp) String() string { return o.str }

func (o *BinaryNonceOp) ProgString() string {
	return fmt.Sprintf("{%s op %s = %s}", o.left.ProgString(), o.right.ProgString(), o.body.ProgString())
}

func (o *BinaryNonceOp) Initializers() []*Var {
	return o.initializers
}

func (o *BinaryNonceOp) bindInitializers(c Context, locals map[string]Value) {
	if len(locals) == 0 {
		return
	}
	o.initializers = toVarSlice(locals)
	o.initString = toString(locals)
	c.BindInitializers(o.op, o.initializers)
}

func (o *BinaryNonceOp) InitString() string {
	return o.initString
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

func (o *OpExpr) Flush() {
	opFlush(o.unary)
	opFlush(o.binary)
}

func opFlush(op Operator) {
	if op == nil {
		return
	}
	switch nonce := op.(type) {
	case *UnaryNonceOp:
		nonce.flush()
	case *BinaryNonceOp:
		nonce.flush()
	default:
		// Anything to do?
	}
}
func (o *OpExpr) ProgString() string {
	var op Operator = o.binary
	if op == nil {
		op = o.unary
	}
	return op.ProgString()
}

func (o *OpExpr) Eval(c Context) Value {
	return o
}

func (o *OpExpr) String() string {
	var op Operator = o.binary
	if op == nil {
		op = o.unary
	}
	return op.String()
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

// localsUsed winnows the set of variables to those mentioned in
// the statement list.
func localsUsed(s StatementList, vars []*Var) map[string]Value {
	bindings := map[string]Value{}
	for _, expr := range s {
		if stmt, ok := expr.(*Statement); ok {
			names, _ := stmt.VarsAndRet()
			for _, name := range names {
				for _, v := range vars {
					// Should always be a local but check.
					if v.Name() == name && v.State() == LocalVar {
						if val := v.Value(); val != nil {
							bindings[name] = val
						}
						break
					}
				}
			}
		}
	}
	return bindings
}

// bindLocals sets initializers for the operator, if it is a NonceOp,
// to remember the value of any locals mentioned in the body.
func bindLocals(c Context, v Operator, locals []*Var) {
	switch o := v.(type) {
	case *UnaryNonceOp:
		o.bindInitializers(c, localsUsed(o.body, locals))
	case *BinaryNonceOp:
		o.bindInitializers(c, localsUsed(o.body, locals))
	}
}

// BindOpExprLocals checks if there are OpExprs in v that mention local variables.
// If so, they are attached to the op as intializers so their value is availabe at
// execution. The variables are known to be local to the current Function.
func BindOpExprLocals(c Context, v Expr, locals []*Var) {
	switch v := v.(type) {
	case *OpExpr:
		bindLocals(c, v.unary, locals)
		bindLocals(c, v.binary, locals)
	case VectorExpr:
		for _, e := range v {
			BindOpExprLocals(c, e, locals)
		}
	}
}
