// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

// Frame holds the local execution context for a user-defined op.
// Used to print helpful error tracebacks. May one day hold the
// local variables themselves.
type Frame struct {
	Op       string
	IsBinary bool
	Left     Expr
	Right    Expr
	Inited   bool // Until set, tracebacks will not attempt to evaluate this frame.
	Vars     Symtab
}

// Symtab is a symbol table, a map of names to variables.
type Symtab map[string]*Var
