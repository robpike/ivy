// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import (
	"fmt"

	"robpike.io/ivy/scan"
	"robpike.io/ivy/value"
)

type function struct {
	isBinary bool
	name     string
	left     string
	right    string
	body     []value.Expr
}

func (fn *function) String() string {
	left := ""
	if fn.isBinary {
		left = fn.left + " "
	}
	s := fmt.Sprintf("op %s%s %s =", left, fn.name, fn.right)
	if len(fn.body) == 1 {
		return s + " " + fn.body[0].ProgString()
	}
	for _, stmt := range fn.body {
		s += "\n\t" + stmt.ProgString()
	}
	return s
}

// function definition
//
//	"op" name arg '\n'
//	"op" name arg '=' statements '\n'
//	"op" arg name arg '=' statements '\n'
//
// statements:
//	expressionList
//	'\n' (expressionList '\n')+ '\n' # For multiline definition, ending with blank line.
//
func (p *Parser) functionDefn() {
	p.need(scan.Op)
	fn := new(function)
	// Two identifiers means: op arg.
	// Three identifiers means: arg op arg.
	idents := make([]string, 2, 3)
	idents[0] = p.need(scan.Identifier).Text
	idents[1] = p.need(scan.Identifier).Text
	if p.peek().Type == scan.Identifier {
		idents = append(idents, p.next().Text)
	}
	tok := p.next()
	// Install the function in the symbol table so recursive ops work. (As if.)
	var installMap map[string]*function
	if len(idents) == 3 {
		fn.isBinary = true
		fn.left = idents[0]
		fn.name = idents[1]
		fn.right = idents[2]
		installMap = p.context.binaryFn
	} else {
		fn.name = idents[0]
		fn.right = idents[1]
		installMap = p.context.unaryFn
	}
	if fn.name == fn.left || fn.name == fn.right {
		p.errorf("argument name %q is function name", fn.name)
	}
	// Define it, but prepare to undefine if there's trouble.
	p.context.define(fn)
	succeeded := false
	prevDefn := installMap[fn.name]
	defer func() {
		if !succeeded {
			if prevDefn == nil {
				delete(installMap, fn.name)
			} else {
				installMap[fn.name] = prevDefn
			}
		}
	}()

	switch tok.Type {
	case scan.Assign:
		// Either one line:
		//	op x a = expression
		// or multiple lines terminated by a blank line:
		//	op x a =
		//	expression
		//	expression
		//
		if p.peek().Type == scan.Newline {
			// Multiline.
			p.next()
			for p.peek().Type != scan.Newline {
				x, ok := p.expressionList()
				if !ok {
					p.errorf("invalid function definition")
				}
				fn.body = append(fn.body, x...)
			}
			p.next() // Consume final newline.
		} else {
			// Single line.
			var ok bool
			fn.body, ok = p.expressionList()
			if !ok {
				p.errorf("invalid function definition")
			}
		}
		if len(fn.body) == 0 {
			p.errorf("missing function body")
		}
	case scan.Newline:
	default:
		p.errorf("expected newline after function declaration, found %s", tok)
	}
	p.context.define(fn)
	succeeded = true
	for _, ref := range fn.references() {
		// One day this will work, but until we have ifs and such, warn.
		if ref.name == fn.name && ref.isBinary == fn.isBinary {
			p.Printf("warning: definition of %s is recursive\n", fn.name)
		}
	}
	if p.config.Debug("parse") {
		p.Printf("op %s %s %s = %s\n", fn.left, fn.name, fn.right, tree(fn.body))
	}
}

// references returns a list, in appearance order, of the user-defined ops
// referenced by this function. Only the first appearance creates an
// entry in the list.
func (fn *function) references() []opDef {
	var refs []opDef
	for _, expr := range fn.body {
		doReferences(&refs, expr)
	}
	return refs
}

func doReferences(refs *[]opDef, expr value.Expr) {
	switch e := expr.(type) {
	case *unary:
		// Operators are not user-defined so are not references in this sense.
		doReferences(refs, e.right)
	case *binary:
		doReferences(refs, e.left)
		doReferences(refs, e.right)
	case variableExpr:
	case sliceExpr:
		for _, v := range e {
			doReferences(refs, v)
		}
	case *assignment:
		doReferences(refs, e.expr)
	case *binaryCall:
		addReference(refs, e.name, true)
		doReferences(refs, e.left)
		doReferences(refs, e.right)
	case *unaryCall:
		addReference(refs, e.name, false)
		doReferences(refs, e.arg)
	case value.Char:
	case value.Int:
	case value.BigInt:
	case value.BigFloat:
	case value.BigRat:
	case value.Vector:
	case value.Matrix:
	default:
		fmt.Printf("unknown %T\n", e)
	}
}

func addReference(refs *[]opDef, name string, isBinary bool) {
	// If it's already there, ignore. This is n^2 but n is tiny.
	for _, ref := range *refs {
		if ref.name == name && ref.isBinary == isBinary {
			return
		}
	}
	*refs = append(*refs, opDef{name, isBinary})
}

type unaryCall struct {
	name string
	arg  value.Expr
}

func (u *unaryCall) Eval(context value.Context) value.Value {
	arg := u.arg.Eval(context)
	context.Push()
	defer context.Pop()
	exec := context.(*execContext) // Sigh.
	fn := exec.unaryFn[u.name]
	if fn == nil || fn.body == nil {
		value.Errorf("unary %q undefined", u.name)
	}
	context.AssignLocal(fn.right, arg)
	var v value.Value
	for _, e := range fn.body {
		v = e.Eval(context)
	}
	if v == nil {
		value.Errorf("no value returned by %q", u.name)
	}
	return v
}

func (u *unaryCall) ProgString() string {
	return fmt.Sprintf("(%s %s)", u.name, u.arg.ProgString())
}

type binaryCall struct {
	name  string
	left  value.Expr
	right value.Expr
}

func (b *binaryCall) Eval(context value.Context) value.Value {
	left := b.left.Eval(context)
	right := b.right.Eval(context)
	context.Push()
	defer context.Pop()
	exec := context.(*execContext) // Sigh.
	fn := exec.binaryFn[b.name]
	if fn == nil || fn.body == nil {
		value.Errorf("binary %q undefined", b.name)
	}
	context.AssignLocal(fn.left, left)
	context.AssignLocal(fn.right, right)
	var v value.Value
	for _, e := range fn.body {
		v = e.Eval(context)
	}
	if v == nil {
		value.Errorf("no value returned by %q", b.name)
	}
	return v
}

func (b *binaryCall) ProgString() string {
	return fmt.Sprintf("(%s %s %s)", b.left.ProgString(), b.name, b.right.ProgString())
}
