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

// StackTrace prints the execution stack.
// There may be conditions under which it will cause trouble
// by printing invalid values, but it tries to be safe.
func (c *Context) StackTrace() {
	const max = 25
	// We need to pop the stack to print the values, but we don't want to
	// necessarily wipe the stack. Also there is possibly other pfor code
	// running even at failure. So use a copy of the context for now.
	//TODO: Can we guarantee pfor is done before we get here?
	nc := &Context{}
	*nc = *c
	c = nc
	n := len(c.Stack)
	skip := 0
	if n > max {
		skip = n - max
		n = max
	}
	// We need to print the innermost, then pop it and go around.
	// But we want the output to be innermost last, so save and reverse.
	lines := []string{}
	for i := range n {
		if len(c.Stack) == 0 {
			break
		}
		if skip > 0 && i == max/2 {
			for range skip {
				c.pop()
			}
			lines = append(lines, fmt.Sprintf("\t--- stack too deep; skipping %d frames\n", skip))
		}
		f := c.TopOfStack()
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
			for _, v := range f.Vars {
				if !slices.Contains(args, v.Name()) {
					frame += c.LocalPrint(v.Name())
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

func parens(s string, t bool) string {
	if t {
		return "(" + s + ")"
	}
	return s
}

func needParens(c *Context, v value.Value) bool {
	if value.IsScalarType(c, v) {
		return false
	}
	if x, ok := v.(*value.Vector); ok && x.AllChars() {
		return false
	}
	return true
}

// tracePrint prints the value in a manner that is likely useful for reuse. Strings
// are quoted, matrices are printed in rho form, and so on. It is imperfect: for
// example, it truncates values and doesn't guarantee floats show as floats,
// let alone have all their bits, but it's good enough.
func (c *Context) tracePrint(val value.Value) string {
	s := ""
	switch v := val.(type) {
	case nil:
		return "" // No parens.
	default:
		s = fmt.Sprintf("%T %s", v, v.Sprint(c))
	case value.Int, value.BigInt, value.BigRat, value.BigFloat, value.Complex:
		s = short(v.Sprint(c))
	case value.Char:
		s = fmt.Sprintf("%q", v.Sprint(c))
	case *value.Vector:
		switch {
		case v.Len() == 0:
			s = "()"
		case v.AllChars():
			s = fmt.Sprintf("%q", short(v.Sprint(c)))
		default:
			for i, elem := range v.All() {
				if i > 0 {
					s += " "
				}
				s += parens(short(c.tracePrint(elem)), needParens(c, elem))
			}
		}
	case *value.Matrix:
		s += fmt.Sprint(v.Shape())
		s = s[1 : len(s)-1]
		s += " rho "
		s += c.tracePrint(v.Data())
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
		s = fmt.Sprintf("%T %s", a, value.DebugProgString(a))
	case *value.VarExpr, value.VectorExpr:
		if v := a.Eval(c); v != nil {
			s = c.tracePrint(v)
		}
	}
	return parens(s, true)
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
