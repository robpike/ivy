// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

import (
	"fmt"
	"strings"

	"robpike.io/ivy/value"
)

// Function represents a unary or binary user-defined operator.
type Function struct {
	IsBinary     bool
	Name         string
	Left         value.Expr
	Right        value.Expr
	Body         value.StatementList
	Variables    []string // Names mentioned in the body that could be vars.
	Source       string
	HasRet       bool
	initializers []*value.Var // For OpExprs only, variables to set before execution.
	// At time of definition; needed to parse saved source correctly.
	Ibase int
}

var _ value.Operator = (*Function)(nil)

// Used for debugging. The output is not valid Ivy syntax.
func (fn *Function) ProgString() string {
	var b strings.Builder
	b.WriteString("op ")
	if fn.IsBinary {
		b.WriteString(value.DebugProgString(fn.Left))
		b.WriteRune(' ')
	}
	b.WriteString(fn.Name)
	b.WriteRune(' ')
	b.WriteString(value.DebugProgString(fn.Right))
	b.WriteString(" = {")
	b.WriteString(fn.Source)
	fmt.Fprintf(&b, "} {Variables: %s} ", fn.Variables)
	return b.String()
}

// Needed for the value.Operator interface.
func (fn *Function) String() string {
	return fn.Name
}

func (fn *Function) newFrame() *value.Frame {
	frame := &value.Frame{
		Op:           fn.Name,
		IsBinary:     fn.IsBinary,
		Left:         fn.Left,
		Right:        fn.Right,
		Inited:       false,
		Vars:         value.Symtab{},
		Initializers: fn.initializers,
	}
	frame.Vars = make([]*value.Var, len(fn.Variables))
	for i, id := range fn.Variables {
		frame.Vars[i] = value.NewVar(id, nil, value.Unknown)
	}
	return frame
}

func (fn *Function) EvalUnary(context value.Context, right value.Value) value.Value {
	// It's known to be an exec.Context.
	c := context.(*Context)
	if fn.Body == nil {
		c.Errorf("unary %q undefined", fn.Name)
	}
	if uint(len(c.Stack)) >= c.config.MaxStack() {
		c.Errorf("stack overflow calling %q", fn.Name)
	}
	c.push(fn)
	if value.HasOpExprs(right) {
		value.FlushState(fn.Body)
	}
	for _, val := range fn.initializers {
		value.Assign(context, fn.Name, value.NewVarExpr(val.Name()), nil, val.Value())
	}
	value.Assign(context, fn.Name, fn.Right, right, right)
	c.TopOfStack().Inited = true
	v := value.EvalFunctionBody(c, fn.Name, fn.Body, fn.HasRet)
	if v == nil {
		c.Errorf("no value returned by %q", fn.Name)
	}
	if value.HasOpExprs(v) {
		value.BindOpExprLocals(c, v, c.TopOfStack().Vars)
	}
	c.pop() // Don't defer, so if we get an error we can print the stack.
	return v
}

func (fn *Function) EvalBinary(context value.Context, left, right value.Value) value.Value {
	if fn.Body == nil {
		context.Errorf("binary %q undefined", fn.Name)
	}
	// It's known to be an exec.Context.
	c := context.(*Context)
	if uint(len(c.Stack)) >= c.config.MaxStack() {
		c.Errorf("stack overflow calling %q", fn.Name)
	}
	c.push(fn)
	if value.HasOpExprs(left) || value.HasOpExprs(right) {
		value.FlushState(fn.Body)
	}
	for _, val := range fn.initializers {
		value.Assign(context, fn.Name, value.NewVarExpr(val.Name()), nil, val.Value())
	}
	value.Assign(context, fn.Name, fn.Left, left, left)
	value.Assign(context, fn.Name, fn.Right, right, right)
	c.TopOfStack().Inited = true
	v := value.EvalFunctionBody(c, fn.Name, fn.Body, fn.HasRet)
	if v == nil {
		c.Errorf("no value returned by %q", fn.Name)
	}
	if value.HasOpExprs(v) {
		value.BindOpExprLocals(c, v, c.TopOfStack().Vars)
	}
	c.pop() // Don't defer, so if we get an error we can print the stack.
	return v
}

// WalkVars executes f for each variable mentioned in the expression.
// Used only to declare arguments to ops, so only need vars and vectors.
func WalkVars(c value.Context, expr value.Expr, f func(*value.VarExpr)) {
	switch e := expr.(type) {
	case *value.VarExpr:
		f(e)
	case value.VectorExpr:
		for _, v := range e {
			WalkVars(c, v, f)
		}
	default:
		c.Errorf("internal error: unknown %T in variable list\n", e)
	}
}

// FuncVars collects the list of identifiers in the body. We don't know yet whether
// they are variables, or whether they are local or global; that happens at
// execution. See value.VarState, value.Assign and value.VarExpr.Eval for details.
// A function that wants to guarantee a variable is global can do a throwaway read,
// as in
//
//	_ = x # global x
//	x = 1
//
// It also marks whether we have a :ret (see value.EvalFunctionBody).
func FuncVars(c value.Context, varNames map[string]bool, fn *Function) {
	// We know the body is an StatementList of Statements.
	for _, s := range fn.Body {
		s := s.(*value.Statement)
		vars, hasRet := s.VarsAndRet()
		if hasRet {
			fn.HasRet = true
		}
		for _, name := range vars {
			varNames[name] = true
		}
	}
	for name := range varNames {
		fn.Variables = append(fn.Variables, name)
	}
	return
}

// NewNonceOp creates an anonymous function and returns it as an Operator.
// TODO: If the return value contains an OpExpr, we should create a closure
// for any local variables mentioned within.
func (c *Context) NewNonceOp(isBinary bool, left, right value.Expr, body value.StatementList, source string) value.Operator {
	var fn Function
	fn.Body = body
	fn.Source = source
	// At time of definition; needed to parse saved source correctly.
	fn.Ibase, _ = c.Config().Base()
	varNames := make(map[string]bool)
	fn.Name = source // Works well in traces.
	fn.Left = left   // Will be nil if unary.
	fn.Right = right
	flush := value.HasOpExprs(right)
	if isBinary {
		fn.IsBinary = true
		flush = flush || value.HasOpExprs(left)
	}
	if flush {
		value.FlushState(body)
	}
	declare := func(x *value.VarExpr) {
		if x.Name == "_" {
			return
		}
		if varNames[x.Name] {
			c.Errorf("multiple arguments named %q in nonce op", x.Name)
		}
		varNames[x.Name] = true
	}
	if isBinary {
		WalkVars(c, fn.Left, declare)
	}
	WalkVars(c, fn.Right, declare)
	FuncVars(c, varNames, &fn)
	return &fn
}
