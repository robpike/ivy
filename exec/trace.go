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

// short returns its argument, truncating if it's too long.
func short(s string) string {
	if len(s) > 50 {
		s = s[:50] + "..."
	}
	return s
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
			s = v.Sprint(c)
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
	return fmt.Sprintf("\t\t%s = %s\n", name, short(v.Sprint(c)))
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
