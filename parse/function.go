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
//	"op" name arg '=' body <eol>
//	"op" arg name arg '=' body <eol>
//
// body:
//	statementList
//	'\n' (statementList '\n')+ '\n' # For multiline definition, ending with blank line.
//
func (p *Parser) functionDefn(start int) {
	p.context.SetPos(p.fileName, p.lineNum, p.offset)
	p.inOperator = true
	defer func() { p.inOperator = false }()
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
		p.errorf("invalid function name: %v", value.DebugProgString(nameArg))
	}

	// Prepare to declare arguments.
	varNames := make(map[string]bool)
	declare := func(x *value.VarExpr) {
		if x.Name == fn.Name {
			p.errorf("argument name %q is function name", fn.Name)
		}
		if x.Name == "_" {
			return
		}
		if varNames[x.Name] {
			p.errorf("multiple arguments named %q", x.Name)
		}
		varNames[x.Name] = true
	}

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
				p.context.SetPos(p.fileName, p.lineNum, p.offset)
				fn.Body = append(fn.Body, p.statementList()...)
				if !p.readTokensToNewline() {
					p.errorf("invalid function definition")
				}
			}
			p.next() // Consume final newline.
		} else {
			// Single line.
			fn.Body = p.statementList()
		}
		if len(fn.Body) == 0 {
			p.errorf("missing function body")
		}
	default:
		p.errorf("expected definition after function declaration, found %s", tok)
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
	funcVars(p.context, varNames, fn)
	// Have we added a new operator? If so, must flush saved parses because they
	// may now parse differently.
	if installMap[fn.Name] == nil {
		p.context.FlushSavedParses()
	}
	p.context.Define(fn)
	if p.context.Config().Debug("parse") > 0 {
		left := ""
		if fn.Left != nil {
			left = fn.Left.ProgString()
		}
		p.Printf("op %s %s %s = %s\n", left, fn.Name, fn.Right.ProgString(), tree(p.context, fn.Body))
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

// funcVars collects the list of identifiers in the body. We don't now yet whether
// they are variables, or whether they are local or global; that happens at
// execution. See value.VarState, value.Assign and value.VarExpr.Eval for details.
// A function that wants to guarantee a variable is global can do a throwaway read,
// as in
//
//	_ = x # global x
//	x = 1
//
// It also marks whether we have a :ret (see value.EvalFunctionBody).
func funcVars(c value.Context, varNames map[string]bool, fn *exec.Function) {
	// We know the body is an StatementList of Statements.
	for _, s := range fn.Body {
		s := s.(*value.Statement)
		vars, hasRet := s.VarsAndRet()
		if hasRet {
			fn.HasRet = true
		}
		for _, name := range vars {
			varNames[name] = true
		}
	}
	for name := range varNames {
		fn.Variables = append(fn.Variables, name)
	}
	return
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
