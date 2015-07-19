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
	left     variableExpr
	right    variableExpr
	body     []value.Expr
}

func (fn *function) String() string {
	left := ""
	if fn.isBinary {
		left = fn.left.name + " "
	}
	s := fmt.Sprintf("op %s%s %s =", left, fn.name, fn.right.name)
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
	if len(idents) == 3 {
		fn.isBinary = true
		fn.left = p.variable(idents[0])
		fn.name = idents[1]
		fn.right = p.variable(idents[2])
		p.context.noVar(fn.name)
		p.context.binaryFn[fn.name] = fn
	} else {
		fn.name = idents[0]
		fn.right = p.variable(idents[1])
		p.context.noVar(fn.name)
		p.context.unaryFn[fn.name] = fn
	}
	if fn.name == fn.left.name || fn.name == fn.right.name {
		p.errorf("argument name %q is function name", fn.name)
	}
	if p.config.Debug("parse") {
		p.Printf("op %s %s %s = %s\n", fn.left, fn.name, fn.right, tree(fn.body))
	}
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
	context.AssignLocal(fn.right.name, arg)
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
	context.AssignLocal(fn.left.name, left)
	context.AssignLocal(fn.right.name, right)
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
