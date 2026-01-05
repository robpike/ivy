// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// Expr is the interface for a parsed expression.
// Also implemented by Value.
type Expr interface {
	// ProgString returns the unambiguous representation of the
	// expression to be used in program source.
	ProgString() string

	Eval(Context) Value
}

type UnaryExpr struct {
	Op    string
	Right Expr
}

func (u *UnaryExpr) ProgString() string {
	return fmt.Sprintf("%s %s", u.Op, u.Right.ProgString())
}

func (u *UnaryExpr) Eval(context Context) Value {
	return context.EvalUnary(u.Op, u.Right.Eval(context).Inner())
}

type BinaryExpr struct {
	Op    string
	Left  Expr
	Right Expr
}

func (b *BinaryExpr) ProgString() string {
	var left string
	if IsCompound(b.Left) {
		left = fmt.Sprintf("(%s)", b.Left.ProgString())
	} else {
		left = b.Left.ProgString()
	}
	return fmt.Sprintf("%s %s %s", left, b.Op, b.Right.ProgString())
}

func (b *BinaryExpr) Eval(context Context) Value {
	if b.Op == "=" {
		return assign(context, b)
	}
	rhs := b.Right.Eval(context).Inner()
	lhs := b.Left.Eval(context)
	return context.EvalBinary(lhs, b.Op, rhs)
}

type ExprList []Expr

func (e ExprList) ProgString() string {
	var b strings.Builder
	for _, expr := range e {
		fmt.Fprintln(&b, expr.ProgString())
	}
	return b.String()
}

func (e ExprList) Eval(context Context) Value {
	v, _ := evalExpressionList(context, "expression list", empty, e)
	return v
}

// ColonExpr is a conditional executor: expression ":" expression. It shortcuts
// execution of an ExprList.
type ColonExpr struct {
	Cond  Expr
	Value Expr
}

func (c *ColonExpr) ProgString() string { return c.Cond.ProgString() + " : " + c.Value.ProgString() }

func (c *ColonExpr) Eval(context Context) Value {
	v := Value(empty)
	if isTrue(context, ":", c.Cond.Eval(context)) {
		if c.Value != nil {
			v = c.Value.Eval(context)
		}
	}
	return v
}

// WhileExpr is a loop expression: ":while" expression; expressionList; ":end"
type WhileExpr struct {
	Cond Expr
	Body ExprList
}

func (w *WhileExpr) ProgString() string {
	s := ":while "
	s += w.Cond.ProgString()
	s += "; "
	s += w.Body.ProgString()
	s += ":end;"
	return s
}

func (w *WhileExpr) Eval(context Context) Value {
	v := Value(empty)
	done := false
	for !done && isTrue(context, ":while", w.Cond.Eval(context)) {
		if w.Body != nil {
			v, done = evalExpressionList(context, ":while", empty, w.Body)
		}
	}
	return v
}

// IfExpr is a conditional expression: ":if" expression; expressionList [":else" expressionList] ":end"
// If there is an ":elif", it has been parsed into a properly nested ":else" ":if".
type IfExpr struct {
	Cond     Expr
	Body     ExprList
	ElseBody ExprList
}

func (i *IfExpr) ProgString() string {
	s := ":if "
	s += i.Cond.ProgString()
	s += "; "
	s += i.Body.ProgString()
	if i.ElseBody != nil {
		s += ":else "
		s += i.ElseBody.ProgString()
		s += "; "

	}
	s += ":end;"
	return s
}

func (i *IfExpr) Eval(context Context) Value {
	v := Value(empty)
	if isTrue(context, ":if", i.Cond.Eval(context)) {
		if i.Body != nil {
			v = EvalBlock(context, ":if", i.Body)
		}
	} else if i.ElseBody != nil {
		v = EvalBlock(context, ":if", i.ElseBody)
	}
	return v
}

// RetExpr is an early return from a function. See EvalFunctionBody.
type RetExpr struct {
	Expr  Expr  // In the parse tree.
	Value Value // After evaluation.
}

func (r *RetExpr) ProgString() string {
	return ":ret " + r.Expr.ProgString()
}

func (r *RetExpr) Eval(context Context) Value {
	r.Value = r.Expr.Eval(context)
	panic(r) // Stop execution of innermost function; see EvalFunctionBody.
}

// VectorExpr holds a syntactic vector to be verified and evaluated.
type VectorExpr []Expr

func (e VectorExpr) Eval(context Context) Value {
	v := newVectorEditor(len(e), nil)
	// Evaluate right to left, as is the usual rule.
	// This also means things like
	//	x=1000; x + x=2
	// (yielding 4) work.
	for i := len(e) - 1; i >= 0; i-- {
		v.Set(i, e[i].Eval(context))
	}
	return v.Publish()
}

var charEscape = map[rune]string{
	'\\': "\\\\",
	'\'': "\\'",
	'\a': "\\a",
	'\b': "\\b",
	'\f': "\\f",
	'\n': "\\n",
	'\r': "\\r",
	'\t': "\\t",
	'\v': "\\v",
}

func (e VectorExpr) ProgString() string {
	var b bytes.Buffer
	// If it's all Char, we can do a prettier job.
	if e.allChars() {
		b.WriteRune('\'')
		for _, v := range e {
			c := rune(v.(Char))
			esc := charEscape[c]
			if esc != "" {
				b.WriteString(esc)
				continue
			}
			if !strconv.IsPrint(c) {
				if c <= 0xFFFF {
					fmt.Fprintf(&b, "\\u%04x", c)
				} else {
					fmt.Fprintf(&b, "\\U%08x", c)
				}
				continue
			}
			b.WriteRune(c)
		}
		b.WriteRune('\'')
	} else {
		for i, v := range e {
			if i > 0 {
				b.WriteRune(' ')
			}
			if IsCompound(v) {
				b.WriteString("(" + v.ProgString() + ")")
			} else {
				b.WriteString(v.ProgString())
			}
		}
	}
	return b.String()
}

func (e VectorExpr) allChars() bool {
	for _, c := range e {
		if _, ok := c.(Char); !ok {
			return false
		}
	}
	return true
}

type IndexExpr struct {
	Op    string
	Left  Expr
	Right []Expr
}

func (x *IndexExpr) ProgString() string {
	var s strings.Builder
	s.WriteString("(") // Always parenthesize an index expression to avoid binding ambiguity.
	if IsCompound(x.Left) {
		s.WriteString("(")
		s.WriteString(x.Left.ProgString())
		s.WriteString(")")
	} else {
		s.WriteString(x.Left.ProgString())
	}
	s.WriteString("[")
	for i, v := range x.Right {
		if i > 0 {
			s.WriteString("; ")
		}
		if v != nil {
			s.WriteString(v.ProgString())
		}
	}
	s.WriteString("])")
	return s.String()
}

func (x *IndexExpr) Eval(context Context) Value {
	return Index(context, x, x.Left, x.Right)
}

// VarExpr identifies a variable to be looked up and evaluated.
type VarExpr struct {
	Name  string
	Local bool // local, not global
}

func NewVarExpr(name string) *VarExpr {
	return &VarExpr{Name: name}
}

func (e *VarExpr) Eval(c Context) Value {
	var v Value
	if e.Local {
		v = c.Local(e.Name).Value()
	} else {
		if g := c.Global(e.Name); g != nil {
			v = g.Value()
		}
	}
	if v == nil {
		kind := "global"
		if e.Local {
			kind = "local"
		}
		c.Errorf("undefined %s variable %q", kind, e.Name)
	}
	return v
}

func (e *VarExpr) ProgString() string {
	return e.Name
}

// IsCompound reports whether the item is a non-trivial expression tree, one that
// may require parentheses around it when printed to maintain correct evaluation order.
func IsCompound(x interface{}) bool {
	switch x := x.(type) {
	case Char, Int, BigInt, BigRat, BigFloat, Complex, *Vector, *Matrix:
		return false
	case *VarExpr:
		return false
	case VectorExpr:
		return true
	case *IndexExpr:
		return IsCompound(x.Left)
	default:
		return true
	}
}
