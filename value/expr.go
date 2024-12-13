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

// CondExpr is a CondExpr executor: expression ":" expression
type CondExpr struct {
	Cond *BinaryExpr
}

func (c *CondExpr) ProgString() string         { return c.Cond.ProgString() }
func (c *CondExpr) Eval(context Context) Value { return c.Cond.Eval(context) }

var _ = Decomposable(&CondExpr{})

func (c *CondExpr) Operator() string {
	return ":"
}

func (c *CondExpr) Operands() (left, right Expr) {
	return c.Cond.Left, c.Cond.Right
}

// VectorExpr holds a syntactic vector to be verified and evaluated.
type VectorExpr []Expr

func (e VectorExpr) Eval(context Context) Value {
	v := make([]Value, len(e))
	// Evaluate right to left, as is the usual rule.
	// This also means things like
	//	x=1000; x + x=2
	// (yielding 4) work.
	for i := len(e) - 1; i >= 0; i-- {
		v[i] = e[i].Eval(context)
	}
	return NewVector(v)
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
	s.WriteString("]")
	return s.String()
}

func (x *IndexExpr) Eval(context Context) Value {
	return Index(context, x, x.Left, x.Right)
}

// VarExpr identifies a variable to be looked up and evaluated.
type VarExpr struct {
	Name  string
	Local int // local index, or 0 for global
}

func NewVar(name string) *VarExpr {
	return &VarExpr{Name: name}
}

func (e *VarExpr) Eval(context Context) Value {
	var v Value
	if e.Local >= 1 {
		v = context.Local(e.Local)
	} else {
		v = context.Global(e.Name)
	}
	if v == nil {
		kind := "global"
		if e.Local >= 1 {
			kind = "local"
		}
		Errorf("undefined %s variable %q", kind, e.Name)
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
	case VectorExpr, *VarExpr:
		return false
	case *IndexExpr:
		return IsCompound(x.Left)
	default:
		return true
	}
}
