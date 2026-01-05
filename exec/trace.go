// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

import (
	"fmt"
	"slices"
	"strings"

	"robpike.io/ivy/value"
)

// StackTrace prints the execution stack, then wipes it.
// There may be conditions under which it will cause trouble
// by printing invalid values, but it tries to be safe.
// TODO: Should be able to do this without wiping the stack.
func (c *Context) StackTrace() {
	const max = 25
	n := len(c.Stack)
	if n > max {
		fmt.Fprintf(c.Config().ErrOutput(), "\t•> stack truncated: %d calls total; showing innermost\n", n)
		n = max
	}
	// We need to print the innermost, then pop it and go around.
	// But we want the output to be innermost last, so save and reverse.
	lines := []string{}
	for range n {
		if len(c.Stack) == 0 {
			break
		}
		f := c.topOfStack()
		if !f.Inited {
			continue
		}
		left := c.ArgPrint(f.Left)
		if left != "" {
			left += " "
		}
		right := c.ArgPrint(f.Right)
		frame := fmt.Sprintf("\t•> %s%s %s\n", left, f.Op, right)
		// Now the locals, if any.
		var fn *Function
		if f.IsBinary {
			fn = c.BinaryFn[f.Op]
		} else {
			fn = c.UnaryFn[f.Op]
		}
		if fn != nil {
			args := argNames(fn)
			for _, l := range fn.Locals {
				if !slices.Contains(args, l) {
					frame += c.LocalPrint(l)
				}
			}
		}
		lines = append(lines, frame)
		c.pop()
	}
	for i := len(lines) - 1; i >= 0; i-- {
		fmt.Fprint(c.Config().ErrOutput(), lines[i])
	}
}

const maxArgSize = 100

// short returns its argument, truncating if it's too long.
func short(s string) string {
	if len(s) > maxArgSize {
		s = s[:maxArgSize] + "..."
	}
	return s
}

// tracePrint prints the value in a manner that is likely useful for reuse. Strings
// are quoted, matrices are printed in rho form, and so on. It could do a perfect
// job by parenthesizing everything and making sure floats are printed as floats
// with all bits, but that seems excessive.
func (c *Context) tracePrint(val value.Value) string {
	s := ""
	switch v := val.(type) {
	case nil:
		return "" // No parens.
	default:
		s = fmt.Sprintf("%T %s", v, v.Sprint(c))
	case value.Int, value.BigInt, value.BigRat, value.BigFloat, value.Complex:
		s = v.Sprint(c)
	case value.Char:
		s = fmt.Sprintf("%q", v.Sprint(c))
	case *value.Vector:
		switch {
		case v.Len() == 0:
			s = "()"
		case v.AllChars():
			s = fmt.Sprintf("%q", v.Sprint(c))
		default:
			for i, elem := range v.All() {
				if len(s) > maxArgSize {
					break
				}
				if i > 0 {
					s += " "
				}
				s += c.tracePrint(elem)
			}
		}
	case *value.Matrix:
		s += fmt.Sprint(v.Shape())
		s = s[1 : len(s)-1]
		s += " rho "
		s += c.tracePrint(v.Data())
	}
	if len(s) > maxArgSize {
		s = s[:maxArgSize] + "..."
	}
	return short(s)
}

// ArgPrint prints the value of an argument.
func (c *Context) ArgPrint(arg value.Expr) string {
	s := "nil"
	switch a := arg.(type) {
	case nil:
		return "" // No parens.
	default:
		s = fmt.Sprintf("%T %s", a, a.ProgString())
	case *value.VarExpr, value.VectorExpr:
		if v := a.Eval(c); v != nil {
			s = c.tracePrint(v)
		}
	}
	return "(" + short(s) + ")"
}

// LocalPrint prints the value of a local variable.
func (c *Context) LocalPrint(name string) string {
	local := c.Local(name)
	if local == nil {
		return ""
	}
	v := local.Value()
	if v == nil {
		return ""
	}
	return fmt.Sprintf("\t\t%s = %s\n", name, c.tracePrint(v))
}

// argNames returns all the argument names for the function.
func argNames(fn *Function) []string {
	return append(namesIn(fn.Left), namesIn(fn.Right)...)
}

// namesIn returns a slice of the names in the argument expression.
func namesIn(e value.Expr) []string {
	switch v := e.(type) {
	case *value.VarExpr:
		return []string{v.Name}
	case value.VectorExpr:
		names := []string{}
		for _, elem := range v {
			names = append(names, namesIn(elem)...)
		}
		return names
	}
	return nil
}

var indent = "| "

// TraceIndent returns an indentation marker showing the depth of the stack.
func (c *Context) TraceIndent() string {
	n := 2 * len(c.Stack)
	if len(indent) < n {
		indent = strings.Repeat("| ", n+10)
	}
	return indent[:n]
}
