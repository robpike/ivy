// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

// Saving state to a file.

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"robpike.io/ivy/config"
	"robpike.io/ivy/exec"
	"robpike.io/ivy/value"
)

/*
Output is written as source text, preserving the original precision of values when
possible. Configuration is also saved.

Saving ops is more subtle. The root issue is inside an op definition there may be
an expression like
	x y z
that could parse as a any of the following:
	- vector of three values (x y z)
	- binary operator y applied to x and z
	- unary operator x applied to vector (y z)
	- unary operator x applied to unary operator y applied to z
Which of these it correct depends on which of x and y are operators, and whether
they are unary or binary. Thus we need to print the source in a way that recovers
the correct parse.

Ivy will not allow a variable and an operator to have the same name, so it is
sufficient to know when parsing an op that all the operators it depends on have
already been defined. To do this, we can just print the operator definitions in
the order they originally appeared: if a is an operator, the parse of a is determined
only by what operators have already been defined.  If none of the identifiers
mentioned in the defintion are operators, the parse will take them as variables,
even if those variables are not yet defined.

Thus we can solve the problem by printing all the operator definitions in order,
and then defining all the variables.

Mutually recursive functions are an extra wrinkle but easy to resolve.
*/

// TODO: Find a way to move this into package exec.
// This would require passing the references for each function from
// here to save.

// save writes the state of the workspace to the named file.
// The format of the output is ivy source text.
func save(c *exec.Context, file string) {
	// "<conf.out>" is a special case for testing.
	conf := c.Config()
	out := conf.Output()
	if file != "<conf.out>" {
		fd, err := os.Create(file)
		if err != nil {
			value.Errorf("%s", err)
		}
		defer fd.Close()
		buf := bufio.NewWriter(fd)
		defer buf.Flush()
		out = buf
	}

	// Configuration settings. We will set the base below,
	// after we have printed all numbers in base 10.
	fmt.Fprintf(out, ")prec %d\n", conf.FloatPrec())
	ibase, obase := conf.Base()
	fmt.Fprintf(out, ")maxbits %d\n", conf.MaxBits())
	fmt.Fprintf(out, ")maxdigits %d\n", conf.MaxDigits())
	fmt.Fprintf(out, ")origin %d\n", conf.Origin())
	fmt.Fprintf(out, ")prompt %q\n", conf.Prompt())
	fmt.Fprintf(out, ")format %q\n", conf.Format())
	conf.SetBase(10, 10)

	// Ops.
	printed := make(map[exec.OpDef]bool)
	for _, def := range c.Defs {
		var fn *exec.Function
		if def.IsBinary {
			fn = c.BinaryFn[def.Name]
		} else {
			fn = c.UnaryFn[def.Name]
		}
		for _, ref := range references(c, fn.Body) {
			if !printed[ref] {
				if ref.IsBinary {
					fmt.Fprintf(out, "op _ %s _\n", ref.Name)
				} else {
					fmt.Fprintf(out, "op %s _\n", ref.Name)
				}
				printed[ref] = true
			}
		}
		printed[def] = true
		s := fn.String()
		if strings.Contains(s, "\n") {
			// Multiline def must end in blank line.
			s += "\n"
		}
		fmt.Fprintln(out, s)
	}

	// Global variables.
	syms := c.Stack[0]
	if len(syms) > 0 {
		// Set the base strictly to 10 for output.
		fmt.Fprintf(out, "# Set base 10 for parsing numbers.\n)base 10\n")
		// Sort the names for consistent output.
		sorted := sortSyms(syms)
		for _, sym := range sorted {
			// pi and e are generated
			if sym.name == "pi" || sym.name == "e" {
				continue
			}
			fmt.Fprintf(out, "%s = ", sym.name)
			put(conf, out, sym.val)
			fmt.Fprint(out, "\n")
		}
	}

	// Now we can set the base.
	fmt.Fprintf(out, ")ibase %d\n", ibase)
	fmt.Fprintf(out, ")obase %d\n", obase)

	// Restore the configuration's own base.
	conf.SetBase(ibase, obase)
}

// saveSym holds a variable's name and value so we can sort them for saving.
type saveSym struct {
	name string
	val  value.Value
}

type sortingSyms []saveSym

func (s sortingSyms) Len() int           { return len(s) }
func (s sortingSyms) Less(i, j int) bool { return s[i].name < s[j].name }
func (s sortingSyms) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func sortSyms(syms map[string]value.Value) []saveSym {
	s := make(sortingSyms, len(syms))
	i := 0
	for k, v := range syms {
		s[i] = saveSym{k, v}
		i++
	}
	sort.Sort(s)
	return s
}

// put writes to out a version of the value that will recreate it when parsed.
func put(conf *config.Config, out io.Writer, val value.Value) {
	switch val := val.(type) {
	case value.Char:
		fmt.Fprintf(out, "%q", rune(val))
	case value.Int:
		fmt.Fprintf(out, "%d", int(val))
	case value.BigInt:
		fmt.Fprintf(out, "%d", val.Int)
	case value.BigRat:
		fmt.Fprintf(out, "%d/%d", val.Num(), val.Denom())
	case value.BigFloat:
		if val.Sign() == 0 || val.IsInf() {
			// These have prec 0 and are easy.
			// They shouldn't appear anyway, but be safe.
			fmt.Fprintf(out, "%g", val)
			return
		}
		// TODO The actual value might not have the same prec as
		// the configuration, so we might not get this right
		// Probably not important but it would be nice to fix it.
		digits := int(float64(val.Prec()) * 0.301029995664) // 10 log 2.
		fmt.Fprintf(out, "%.*g", digits+1, val.Float)       // Add another digit to be sure.
	case value.Vector:
		if val.AllChars() {
			fmt.Fprintf(out, "%q", val.Sprint(conf))
			return
		}
		for i, v := range val {
			if i > 0 {
				fmt.Fprint(out, " ")
			}
			put(conf, out, v)
		}
	case *value.Matrix:
		put(conf, out, value.NewIntVector(val.Shape()))
		fmt.Fprint(out, " rho ")
		put(conf, out, val.Data())
	default:
		value.Errorf("internal error: can't save type %T", val)
	}
}
