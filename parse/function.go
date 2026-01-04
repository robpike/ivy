// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import (
	"fmt"
	"strings"

	"robpike.io/ivy/exec"
	"robpike.io/ivy/scan"
	"robpike.io/ivy/value"
)

// function [un]definition
//
//	"op" name arg <eol>
//	"op" name arg '=' statements <eol>
//	"op" arg name arg '=' statements <eol>
//
// statements:
//
//	expressionList
//	'\n' (expressionList '\n')+ '\n' # For multiline definition, ending with blank line.
func (p *Parser) functionDefn(start int) {
	p.InOperator(true)
	defer p.InOperator(false)
	tok := p.need(scan.Op)
	fn := new(exec.Function)
	// Two identifiers means: op arg.
	// Three identifiers means: arg op arg.
	// arg can be name or parenthesized list of args.
	// We scan the op as an arg too, because we're not sure which one it is.
	args := make([]value.Expr, 2, 3)
	args[0] = p.funcArg()
	args[1] = p.funcArg()
	nameArg := args[0]
	if p.peek().Type == scan.Identifier || p.peek().Type == scan.LeftParen {
		nameArg = args[1]
		args = append(args, p.funcArg())
	}
	if x, ok := nameArg.(*value.VarExpr); ok {
		fn.Name = x.Name
	} else {
		p.errorf("invalid function name: %v", nameArg.ProgString())
	}

	// Prepare to declare arguments.
	argNames := make(map[string]bool)
	declare := func(x *value.VarExpr) {
		if x.Name == fn.Name {
			p.errorf("argument name %q is function name", fn.Name)
		}
		if x.Name == "_" {
			return
		}
		if argNames[x.Name] {
			p.errorf("multiple arguments named %q", x.Name)
		}
		argNames[x.Name] = true
		p.context.Declare(x.Name)
	}

	// Install the function in the symbol table so recursive ops work. (As if.)
	var installMap map[string]*exec.Function
	if len(args) == 3 {
		if fn.Name == "o" { // Poor choice due to outer product syntax.
			p.errorf(`"o" is not a valid name for a binary operator`)
		}
		fn.IsBinary = true
		fn.Left = args[0]
		fn.Right = args[2]
		walkVars(fn.Left, declare)
		walkVars(fn.Right, declare)
		installMap = p.context.BinaryFn
	} else {
		fn.Right = args[1]
		walkVars(fn.Right, declare)
		installMap = p.context.UnaryFn
	}

	// Define it, but prepare to undefine if there's trouble.
	prevIndex, _ := p.context.LookupFn(fn.Name, fn.IsBinary)
	prevDefn := installMap[fn.Name]
	p.context.Define(fn) // Source will come at the end.
	defer p.context.ForgetAll()
	succeeded := false
	defer func() {
		if !succeeded {
			fixed := p.context.UndefineOp(fn.Name, fn.IsBinary)
			if fixed && prevDefn != nil {
				fixed = p.context.RestoreOp(prevIndex, prevDefn)
			}
			if !fixed {
				value.Errorf("internal error: redefinition failure for %q", fn.Name)
			}
		}
	}()

	tok = p.next()
	switch tok.Type {
	case scan.Assign:
		// Either one line:
		//	op x a = expression
		// or multiple lines terminated by a blank line:
		//	op x a =
		//	expression
		//	expression
		//
		if p.peek().Type == scan.EOF {
			// Multiline.
			p.next() // Skip newline; not strictly necessary.
			if !p.readTokensToNewline() {
				p.errorf("invalid function definition")
			}
			for p.peek().Type != scan.EOF {
				fn.Body = append(fn.Body, p.expressionList()...)
				if !p.readTokensToNewline() {
					p.errorf("invalid function definition")
				}
			}
			p.next() // Consume final newline.
		} else {
			// Single line.
			fn.Body = p.expressionList()
		}
		if len(fn.Body) == 0 {
			p.errorf("missing function body")
		}
	case scan.EOF:
	default:
		p.errorf("expected newline after function declaration, found %s", tok)
	}
	// Was there a leading comment? If so, bind it to the saved textual definition
	history := p.scanner.History()
	for start > 0 {
		if !strings.HasPrefix(strings.TrimSpace(history[start-1]), "#") {
			break
		}
		start--
	}
	fn.Source = p.source(start, len(history))
	// Remember the base so we can parse the source text again after a save.
	fn.Ibase, _ = p.context.Config().Base()
	funcVars(fn)
	succeeded = true
	p.context.Define(fn)
	if p.context.Config().Debug("parse") > 0 {
		left := ""
		if fn.Left != nil {
			left = fn.Left.ProgString()
		}
		p.Printf("op %s %s %s = %s\n", left, fn.Name, fn.Right.ProgString(), tree(fn.Body))
	}
}

// function argument
//
//	name | '(' args ')'
func (p *Parser) funcArg() value.Expr {
	tok := p.next()
	if tok.Type == scan.Identifier {
		return value.NewVarExpr(tok.Text)
	}
	if tok.Type != scan.LeftParen {
		p.errorf("invalid function argument syntax at %s", tok.Text)
	}
	var v value.VectorExpr
	for p.peek().Type != scan.RightParen {
		v = append(v, p.funcArg())
	}
	p.next()
	return v
}

// references returns a list, in appearance order, of the user-defined ops
// referenced by this function body. Only the first appearance creates an
// entry in the list.
func references(c *exec.Context, body []value.Expr) []exec.OpDef {
	var refs []exec.OpDef
	for _, expr := range body {
		walk(expr, false, func(expr value.Expr, _ bool) {
			switch e := expr.(type) {
			case *value.UnaryExpr:
				if c.UnaryFn[e.Op] != nil {
					addReference(&refs, e.Op, false)
				}
			case *value.BinaryExpr:
				if c.BinaryFn[e.Op] != nil {
					addReference(&refs, e.Op, true)
				}
			}
		})
	}
	return refs
}

func addReference(refs *[]exec.OpDef, name string, isBinary bool) {
	// If it's already there, ignore. This is n^2 but n is tiny.
	for _, ref := range *refs {
		if ref.Name == name && ref.IsBinary == isBinary {
			return
		}
	}
	def := exec.OpDef{
		Name:     name,
		IsBinary: isBinary,
	}
	*refs = append(*refs, def)
}

// funcVars sets fn.Locals and fn.Globals
// to the lists of variables that are local versus global.
// A variable assigned to before any read is a local.
// A variable read before any assignment to is a global.
//
// A function that wants to assign blindly to a global
// can first do a throwaway read, as in
//
//	_ = x # global x
//	x = 1
func funcVars(fn *exec.Function) {
	known := make(map[string]int)
	addLocal := func(e *value.VarExpr) {
		fn.Locals = append(fn.Locals, e.Name)
		known[e.Name] = len(fn.Locals)
	}
	f := func(expr value.Expr, assign bool) {
		switch e := expr.(type) {
		case *value.VarExpr:
			x, ok := known[e.Name]
			if !ok {
				if assign {
					addLocal(e)
				} else {
					known[e.Name] = 0
				}
				x = known[e.Name]
			}
			e.Local = x
		}
	}
	if fn.Left != nil {
		walk(fn.Left, true, f)
	}
	if fn.Right != nil {
		walk(fn.Right, true, f)
	}
	for _, e := range fn.Body {
		walk(e, false, f)
	}
	return
}

// walk traverses expr in right-to-left order,
// calling f on all children, with the boolean argument
// specifying whether the expression is being assigned to,
// after which it calls f(expr, assign).
func walk(expr value.Expr, assign bool, f func(value.Expr, bool)) {
	switch e := expr.(type) {
	case *value.UnaryExpr:
		walk(e.Right, false, f)
	case value.ExprList:
		for _, v := range e {
			walk(v, false, f)
		}
	case *value.ColonExpr:
		walk(e.Cond, false, f)
		walk(e.Value, false, f)
	case *value.IfExpr:
		walk(e.Cond, false, f)
		walk(e.Body, false, f)
		walk(e.ElseBody, false, f)
	case *value.WhileExpr:
		walk(e.Cond, false, f)
		walk(e.Body, false, f)
	case *value.RetExpr:
		walk(e.Expr, false, f)
	case *value.BinaryExpr:
		walk(e.Right, false, f)
		walk(e.Left, e.Op == "=", f)
	case *value.IndexExpr:
		for i := len(e.Right) - 1; i >= 0; i-- {
			x := e.Right[i]
			if x != nil { // Not a placeholder index.
				walk(e.Right[i], false, f)
			}
		}
		walk(e.Left, false, f)
	case *value.VarExpr:
	case value.VectorExpr:
		for i := len(e) - 1; i >= 0; i-- {
			walk(e[i], assign, f)
		}
	case value.Char:
	case value.Int:
	case value.BigInt:
	case value.BigRat:
	case value.BigFloat:
	case value.Complex:
	case *value.Vector:
	case *value.Matrix:
	default:
		fmt.Printf("unknown %T in walk\n", e)
	}
	f(expr, assign)
}

func walkVars(expr value.Expr, f func(*value.VarExpr)) {
	switch e := expr.(type) {
	case *value.VarExpr:
		f(e)
	case value.VectorExpr:
		for i := len(e) - 1; i >= 0; i-- {
			walkVars(e[i], f)
		}
	default:
		fmt.Printf("unknown %T in variable list\n", e)
	}
}
