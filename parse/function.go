// Copyright 2014 Rob Pike. All rights reserved.
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
	left     *variableExpr
	right    *variableExpr
	body     []Expr
}

// function definition
//
//	"def" name arg '=' expressionlist '\n'
//	"def" arg name arg '=' expressionilst '\n'
func (p *Parser) functionDefn() {
	p.need(scan.Def)
	fn := new(function)
	id1 := p.need(scan.Identifier).Text
	id2 := p.need(scan.Identifier).Text
	tok := p.need(scan.Identifier, scan.Assign)
	if tok.Type == scan.Assign {
		fn.isBinary = false
		fn.name = id1
		fn.right = p.variable(id2)
	} else {
		fn.isBinary = true
		fn.left = p.variable(id1)
		fn.name = id2
		fn.right = p.variable(tok.Text)
		p.need(scan.Assign)
	}
	body, ok := p.expressionList()
	if !ok {
		p.errorf("invalid function definition")
	}
	if len(body) == 0 {
		p.errorf("missing function body")
	}
	fn.body = body
	fmt.Printf("define (%s %s %s) = %s\n", fn.left, fn.name, fn.right, body)
	if fn.isBinary {
		p.binaryFn[fn.name] = fn
	} else {
		p.unaryFn[fn.name] = fn
	}
}

type unaryCall struct {
	fn  *function
	arg Expr
}

func (u *unaryCall) Eval() value.Value {
	// TODO: BAD: arg is a global!
	u.fn.right.symtab[u.fn.right.name] = u.arg.Eval()
	var v value.Value
	for _, e := range u.fn.body {
		v = e.Eval()
	}
	if v == nil {
		value.Errorf("no value returned by %q", u.fn.name)
	}
	return v
}

func (u *unaryCall) String() string {
	return "unary call" // Never called; handled by Tree.
}

type binaryCall struct {
	fn    *function
	left  Expr
	right Expr
}

func (b *binaryCall) Eval() value.Value {
	// TODO: BAD: arg is a global!
	b.fn.left.symtab[b.fn.left.name] = b.left.Eval()
	b.fn.right.symtab[b.fn.right.name] = b.right.Eval()
	var v value.Value
	for _, e := range b.fn.body {
		v = e.Eval()
	}
	if v == nil {
		value.Errorf("no value returned by %q", b.fn.name)
	}
	return v
}

func (b *binaryCall) String() string {
	return "biary call" // Never called; handled by Tree.
}
