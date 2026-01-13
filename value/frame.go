// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"
	"strings"
)

// Frame holds the local execution context for a user-defined op.
// Used to print helpful error tracebacks. May one day hold the
// local variables themselves.
type Frame struct {
	Op       string
	IsBinary bool
	Left     Expr
	Right    Expr
	Locals   []Variable
	Globals  []Variable
	Inited   bool // Until set, tracebacks will not attempt to evaluate this frame.
	Vars     Symtab
}

// Symtab is a symbol table, a slice of variables.
// Once placed in a Frame, it is of fixed size.
type Symtab []*Var

func (s Symtab) String() string {
	var b strings.Builder
	for _, v := range s {
		fmt.Fprintf(&b, "{%s: %v} ", v.name, v.value)
	}
	fmt.Fprint(&b, "\n")
	return b.String()
}

func (f *Frame) String() string {
	return fmt.Sprintf("%#v\n", f)
}
