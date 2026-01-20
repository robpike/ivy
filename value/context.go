// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"

	"robpike.io/ivy/config"
)

// Pos records a location in the input. It is used by Context.Errorf.
type Pos struct {
	File   string
	Line   int
	Offset int
}

func (p Pos) String() string {
	return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Offset)
}

// UnaryOp is the interface implemented by a simple unary operator.
type UnaryOp interface {
	EvalUnary(c Context, right Value) Value
}

// BinaryOp is the interface implemented by a simple binary operator.
type BinaryOp interface {
	EvalBinary(c Context, right, left Value) Value
}

// A Variable is a name mentioned in a function and records
// the first action done to the associated variable.
type Variable struct {
	Name  string
	State VarState
}

// VarState holds the dynamic state of a variable, which is
// determined by the first action taken with it.
// Assuming we are in a user-defined op:
//
//	Unknown: Nothing has happened yet.
//	LocalVar: The first action was an assignment.
//	GlobalVar: The first action was a read (evaluation).
//
// Within an op, a Var identifying a global variable holds
// only this state. The data itself is in the globals table.
// Implemented by the Assign and VarExpr.Eval functions.
type VarState int

const (
	Unknown VarState = iota
	LocalVar
	GlobalVar
)

// A Var is a named variable in an Ivy execution.
type Var struct {
	name  string
	value Value
	edit  *vectorEditor
	state VarState
}

// Name returns v's name.
func (v *Var) Name() string {
	return v.name
}

// State returns v's state.
func (v *Var) State() VarState {
	return v.state
}

// Value returns v's current value.
func (v *Var) Value() Value {
	if v.edit != nil {
		// Flush edits back into v.value.
		edit := v.edit
		v.edit = nil
		switch val := v.value.(type) {
		default:
			panic(fmt.Sprintf("internal error: misuse of transient for var %s of type %T", v.name, v.value))
		case *Vector:
			v.value = edit.Publish()
		case *Matrix:
			v.value = &Matrix{shape: val.shape, data: edit.Publish()}
		}
	}
	return v.value
}

// Assign assigns value to v.
func (v *Var) Assign(value Value) {
	v.value = value
	v.edit = nil
}

// editor returns a vectorEditor for editing v's underlying data
// (supporting an indexed assignment like v[i] = x).
func (v *Var) editor() *vectorEditor {
	if v.edit == nil {
		switch val := v.value.(type) {
		default:
			panic(fmt.Sprintf("internal error: misuse of transient for var %s of type %T", v.name, v.value))
		case *Vector:
			v.edit = val.edit()
		case *Matrix:
			v.edit = val.data.edit()
		}
	}
	return v.edit
}

// NewVar returns a new Var with the given name and value.
func NewVar(name string, value Value, state VarState) *Var {
	return &Var{name: name, value: value, state: state}
}

// Context is the execution context for evaluation.
// The only implementation is ../exec/Context, but the interface
// is defined separately, here, because of the dependence on Expr
// and the import cycle that would otherwise result.
type Context interface {
	// Config returns the configuration state for evaluation.
	Config() *config.Config

	// Local returns the named local variable.
	Local(name string) *Var

	// IsLocal reports whether the local variable is already defined.
	IsLocal(name string) bool

	// Global returns the named global variable.
	// It returns nil if there is no such variable.
	Global(name string) *Var

	// AssignGlobal assigns to the named global variable,
	// creating it if needed.
	AssignGlobal(name string, value Value)

	// Eval evaluates a list of expressions.
	Eval(exprs []Expr) []Value

	// EvalUnary evaluates a unary operator.
	EvalUnary(op string, right Value) Value

	// EvalBinary evaluates a binary operator.
	EvalBinary(left Value, op string, right Value) Value

	// UserDefined reports whether the specified op is user-defined.
	UserDefined(op string, isBinary bool) bool

	// Errorf reports an execution error and halts execution
	// by panicking with type Error.
	Errorf(format string, args ...interface{})

	// TraceIndent returns an indentation marker showing the depth of the stack.
	TraceIndent() string

	// TopOfStack returns the top frame on the stack, or nil if there is no stack.
	TopOfStack() *Frame

	// StackTrace prints the execution stack.
	StackTrace()

	// DisableTracing can disable tracing for the "failed" error catcher.
	DisableTracing(bool)

	// Pos and SetPos handle recording of source position for error reports.
	Pos() Pos
	SetPos(file string, line, offset int)
}
