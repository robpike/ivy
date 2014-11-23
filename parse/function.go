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
	left     variableExpr
	right    variableExpr
	body     []value.Expr
}

func (fn *function) String() string {
	s := fmt.Sprintf("def %s %s %s = ", fn.left.name, fn.name, fn.right.name)
	for i, stmt := range fn.body {
		if i > 0 {
			s += "; "
		}
		s += stmt.String()
	}
	return s
}

// function definition
//
//	"def" name arg '=' expressionList '\n'
//	"def" arg name arg '=' expressionList '\n'
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
	if fn.name == fn.left.name || fn.name == fn.right.name {
		p.errorf("argument name is function name %q", fn.name)
	}
	body, ok := p.expressionList()
	if !ok {
		p.errorf("invalid function definition")
	}
	if len(body) == 0 {
		p.errorf("missing function body")
	}
	fn.body = body
	if fn.isBinary {
		p.binaryFn[fn.name] = fn
	} else {
		p.unaryFn[fn.name] = fn
	}
	if p.config.Debug("parse") {
		fmt.Printf("def %s %s %s = %s\n", fn.left, fn.name, fn.right, tree(fn.body))
	}
}

type unaryCall struct {
	fn  *function
	arg value.Expr
}

func (u *unaryCall) Eval(context *value.Context) value.Value {
	arg := u.arg.Eval(context)
	context.Push()
	defer context.Pop()
	context.AssignLocal(u.fn.right.name, arg)
	var v value.Value
	for _, e := range u.fn.body {
		v = e.Eval(context)
	}
	if v == nil {
		value.Errorf("no value returned by %q", u.fn.name)
	}
	return v
}

func (u *unaryCall) String() string {
	return fmt.Sprintf("(%s %s)", u.fn.name, u.arg)
}

type binaryCall struct {
	fn    *function
	left  value.Expr
	right value.Expr
}

func (b *binaryCall) Eval(context *value.Context) value.Value {
	left := b.left.Eval(context)
	right := b.right.Eval(context)
	context.Push()
	defer context.Pop()
	context.AssignLocal(b.fn.left.name, left)
	context.AssignLocal(b.fn.right.name, right)
	var v value.Value
	for _, e := range b.fn.body {
		v = e.Eval(context)
	}
	if v == nil {
		value.Errorf("no value returned by %q", b.fn.name)
	}
	return v
}

func (b *binaryCall) String() string {
	return fmt.Sprintf("(%s %s %s)", b.left, b.fn.name, b.right)
}
