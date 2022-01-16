// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import (
	"fmt"

	"robpike.io/ivy/exec"
	"robpike.io/ivy/scan"
	"robpike.io/ivy/value"
)

// function definition
//
//	"op" name arg <eol>
//	"op" name arg '=' statements <eol>
//	"op" arg name arg '=' statements <eol>
//
// statements:
//	expressionList
//	'\n' (expressionList '\n')+ '\n' # For multiline definition, ending with blank line.
//
func (p *Parser) functionDefn() {
	p.need(scan.Op)
	fn := new(exec.Function)
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
	var installMap map[string]*exec.Function
	if len(idents) == 3 {
		if idents[1] == "o" { // Poor choice due to outer product syntax.
			p.errorf(`"o" is not a valid name for a binary operator`)
		}
		fn.IsBinary = true
		fn.Left = idents[0]
		fn.Name = idents[1]
		fn.Right = idents[2]
		p.context.Declare(fn.Left)
		p.context.Declare(fn.Right)
		installMap = p.context.BinaryFn
	} else {
		fn.Name = idents[0]
		fn.Right = idents[1]
		p.context.Declare(fn.Right)
		installMap = p.context.UnaryFn
	}
	if fn.Name == fn.Left || fn.Name == fn.Right {
		p.errorf("argument name %q is function name", fn.Name)
	}
	// Define it, but prepare to undefine if there's trouble.
	p.context.Define(fn)
	defer p.context.ForgetAll()
	succeeded := false
	prevDefn := installMap[fn.Name]
	defer func() {
		if !succeeded {
			if prevDefn == nil {
				delete(installMap, fn.Name)
			} else {
				installMap[fn.Name] = prevDefn
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
		if p.peek().Type == scan.EOF {
			// Multiline.
			p.next() // Skip newline; not stritly necessary.
			if !p.readTokensToNewline() {
				p.errorf("invalid function definition")
			}
			for p.peek().Type != scan.EOF {
				x, ok := p.expressionList()
				if !ok {
					p.errorf("invalid function definition")
				}
				fn.Body = append(fn.Body, x...)
				if !p.readTokensToNewline() {
					p.errorf("invalid function definition")
				}
			}
			p.next() // Consume final newline.
		} else {
			// Single line.
			var ok bool
			fn.Body, ok = p.expressionList()
			if !ok {
				p.errorf("invalid function definition")
			}
		}
		if len(fn.Body) == 0 {
			p.errorf("missing function body")
		}
	case scan.EOF:
	default:
		p.errorf("expected newline after function declaration, found %s", tok)
	}
	p.context.Define(fn)
	succeeded = true
	if p.context.Config().Debug("parse") {
		p.Printf("op %s %s %s = %s\n", fn.Left, fn.Name, fn.Right, tree(fn.Body))
	}
}

// references returns a list, in appearance order, of the user-defined ops
// referenced by this function body. Only the first appearance creates an
// entry in the list.
func references(c *exec.Context, body []value.Expr) []exec.OpDef {
	var refs []exec.OpDef
	for _, expr := range body {
		doReferences(c, &refs, expr)
	}
	return refs
}

func doReferences(c *exec.Context, refs *[]exec.OpDef, expr value.Expr) {
	switch e := expr.(type) {
	case *unary:
		if c.UnaryFn[e.op] != nil {
			addReference(refs, e.op, false)
		}
		doReferences(c, refs, e.right)
	case conditional:
		doBinaryReferences(c, refs, e.binary)
	case *binary:
		doBinaryReferences(c, refs, e)
	case *index:
		doReferences(c, refs, e.left)
		for _, v := range e.right {
			doReferences(c, refs, v)
		}
	case variableExpr:
	case sliceExpr:
		for _, v := range e {
			doReferences(c, refs, v)
		}
	case value.Char:
	case value.Int:
	case value.BigInt:
	case value.BigFloat:
	case value.BigRat:
	case value.Vector:
	case *value.Matrix:
	default:
		fmt.Printf("unknown %T in references\n", e)
	}
}

func doBinaryReferences(c *exec.Context, refs *[]exec.OpDef, e *binary) {
	if c.BinaryFn[e.op] != nil {
		addReference(refs, e.op, true)
	}
	doReferences(c, refs, e.left)
	doReferences(c, refs, e.right)
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
