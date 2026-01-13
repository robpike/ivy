// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

// Saving state to a file.

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"

	"robpike.io/ivy/value"
)

// Save writes the state of the workspace to the named file.
// The format of the output is ivy source text.
//
// Output is written as source text, preserving the original precision of values when
// possible. Configuration is also saved. For ops, we can print the original source.
// Because binding of names to ops is lazy, the order we print ops is irrelevant.
func Save(context value.Context, file string) {
	c := context.(*Context)
	// "<conf.out>" is a special case for testing.
	conf := c.Config()
	out := conf.Output()
	if file != "<conf.out>" {
		fd, err := os.Create(file)
		if err != nil {
			c.Errorf("%s", err)
		}
		defer fd.Close()
		buf := bufio.NewWriter(fd)
		defer buf.Flush()
		out = buf
	}

	ibase, obase := conf.Base() // What user has.
	curIbase := ibase           // Current setting in save file.
	setIbase := func(base int) {
		if base != curIbase {
			fmt.Fprintf(out, ")ibase %d\n", base)
			curIbase = base
		}
	}

	// Configuration settings. Must use base 10 (a.k.a. 0, the default) to input settings correctly.
	fmt.Fprintf(out, ")ibase 0\n")
	fmt.Fprintf(out, ")prec %d\n", conf.FloatPrec())
	fmt.Fprintf(out, ")maxbits %d\n", conf.MaxBits())
	fmt.Fprintf(out, ")maxdigits %d\n", conf.MaxDigits())
	fmt.Fprintf(out, ")origin %d\n", conf.Origin())
	fmt.Fprintf(out, ")prompt %q\n", conf.Prompt())
	fmt.Fprintf(out, ")format %q\n", conf.Format())

	// Return to user's base if needed.
	setIbase(ibase)
	fmt.Fprintf(out, ")obase %d\n", obase)

	// Ops.
	for _, def := range c.Defs {
		var fn *Function
		if def.IsBinary {
			fn = c.BinaryFn[def.Name]
		} else {
			fn = c.UnaryFn[def.Name]
		}
		setIbase(fn.Ibase)
		fmt.Fprintln(out, fn.Source)
	}

	// Return to user's base if needed.
	setIbase(ibase)

	// Global variables.
	syms := c.Globals
	if len(syms) > 0 {
		// Sort the names for consistent output.
		sorted := sortVars(syms)
		for _, sym := range sorted {
			fmt.Fprintf(out, "%s = ", sym.Name())
			Put(c, out, sym.Value(), false)
			fmt.Fprint(out, "\n")
		}
	}
}

// saveVar lets us sort the variables by name for saving.
type saveVar []*value.Var

func sortVars(syms map[string]*value.Var) []*value.Var {
	s := make(saveVar, len(syms))
	i := 0
	for _, v := range syms {
		s[i] = v
		i++
	}
	sort.Slice(s, func(i, j int) bool { return s[i].Name() < s[j].Name() })
	return s
}

// Put writes to out a version of the value that will recreate it when parsed.
// Its output depends on the context, unlike that of DebugProgString.
func Put(c value.Context, out io.Writer, val value.Value, withParens bool) {
	if withParens {
		fmt.Fprint(out, "(")
	}
	switch val := val.(type) {
	case value.Char:
		fmt.Fprintf(out, "%q", rune(val))
	case value.Int:
		fmt.Fprintf(out, "%s", val.Sprint(c))
	case value.BigInt:
		fmt.Fprintf(out, "%s", val.Sprint(c))
	case value.BigRat:
		fmt.Fprintf(out, "%s", val.Sprint(c))
	case value.BigFloat:
		if val.Sign() == 0 || val.IsInf() {
			// These have prec 0 and are easy.
			// They shouldn't appear anyway, but be safe.
			fmt.Fprintf(out, "%g", val)
			break
		}
		// TODO The actual value might not have the same prec as
		// the configuration, so we might not get this right
		// Probably not important but it would be nice to fix it.
		digits := int(float64(val.Prec()) * 0.301029995664) // 10 log 2.
		fmt.Fprintf(out, "%.*g", digits+1, val.Float)       // Add another digit to be sure.
	case value.Complex:
		real, imag := val.Components()
		Put(c, out, real, false)
		fmt.Fprintf(out, "j")
		Put(c, out, imag, false)
	case *value.Vector:
		if val.AllChars() {
			fmt.Fprintf(out, "%q", val.Sprint(c))
			break
		}
		for i, v := range val.All() {
			if i > 0 {
				fmt.Fprint(out, " ")
			}
			Put(c, out, v, !value.IsScalarType(c, v))
		}
	case *value.Matrix:
		Put(c, out, value.NewIntVector(val.Shape()...), false)
		fmt.Fprint(out, " rho ")
		Put(c, out, val.Data(), false)
	default:
		c.Errorf("internal error: can't save type %T", val)
	}
	if withParens {
		fmt.Fprint(out, ")")
	}
}
