// Copyright 2014 Rob Pike. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse // import "robpike.io/ivy/parse"

import (
	"fmt"

	"robpike.io/ivy/config"
	"robpike.io/ivy/scan"
	"robpike.io/ivy/value"
)

type assignment struct {
	variable variableExpr
	expr     value.Expr
}

func (a *assignment) Eval(context *value.Context) value.Value {
	context.Assign(a.variable.name, a.expr.Eval(context))
	return nil
}

func (a *assignment) String() string {
	return fmt.Sprintf("<%s = %s>", a.variable.name, a.expr)
}

// sliceExpr holds a syntactic vector to be verified and evaluated.
type sliceExpr []value.Expr

func (s sliceExpr) Eval(context *value.Context) value.Value {
	v := make([]value.Value, len(s))
	for i, x := range s {
		elem := x.Eval(context)
		// Each element must be a singleton.
		if !isScalar(elem) {
			value.Errorf("vector element must be scalar; have %s", elem)
		}
		v[i] = elem
	}
	return value.NewVector(v)
}

func (s sliceExpr) String() string {
	str := "<"
	for i, v := range s {
		if i > 0 {
			str += " "
		}
		str += v.String()
	}
	return str + ">"
}

// variableExpr identifies a variable to be looked up and evaluated.
type variableExpr struct {
	name string
}

func (e variableExpr) Eval(context *value.Context) value.Value {
	v := context.Lookup(e.name)
	if v == nil {
		value.Errorf("undefined variable %q", e.name)
	}
	return v
}

func (e variableExpr) String() string {
	return fmt.Sprintf("<var %s>", e.name)
}

type unary struct {
	op    string
	right value.Expr
}

func (u *unary) String() string {
	return fmt.Sprintf("(%s %s)", u.op, u.right)
}

func (u *unary) Eval(context *value.Context) value.Value {
	return value.Unary(u.op, u.right.Eval(context))
}

type binary struct {
	op    string
	left  value.Expr
	right value.Expr
}

func (b *binary) String() string {
	return fmt.Sprintf("(%s %s %s)", b.left, b.op, b.right)
}

func (b *binary) Eval(context *value.Context) value.Value {
	return value.Binary(b.left.Eval(context), b.op, b.right.Eval(context))
}

// Parser stores the state for the ivy parser.
type Parser struct {
	scanner    *scan.Scanner
	config     *config.Config
	fileName   string
	lineNum    int
	errorCount int // Number of errors.
	peekTok    scan.Token
	vars       map[string]value.Value
	curTok     scan.Token // most recent token from scanner
	unaryFn    map[string]*function
	binaryFn   map[string]*function
}

var zero, _ = value.Parse("0")

// NewParser returns a new parser that will read from the scanner.
func NewParser(conf *config.Config, fileName string, scanner *scan.Scanner) *Parser {
	return &Parser{
		scanner:  scanner,
		config:   conf,
		fileName: fileName,
		vars:     make(map[string]value.Value),
		unaryFn:  make(map[string]*function),
		binaryFn: make(map[string]*function),
	}
}

func (p *Parser) next() scan.Token {
	tok := p.peekTok
	if tok.Type != scan.EOF {
		p.peekTok = scan.Token{Type: scan.EOF}
	} else {
		tok = <-p.scanner.Tokens
	}
	p.curTok = tok
	if tok.Type != scan.Newline {
		// Show the line number before we hit the newline.
		p.lineNum = tok.Line
	}
	return tok
}

func (p *Parser) peek() scan.Token {
	tok := p.peekTok
	if tok.Type != scan.EOF {
		return tok
	}
	p.peekTok = <-p.scanner.Tokens
	return p.peekTok
}

// Loc returns the current input location in the form name:line.
func (p *Parser) Loc() string {
	return fmt.Sprintf("%s:%d", p.fileName, p.lineNum)
}

func (p *Parser) errorf(format string, args ...interface{}) {
	// Flush to newline.
	for p.curTok.Type != scan.Newline && p.curTok.Type != scan.EOF {
		p.next()
	}
	p.peekTok = scan.Token{Type: scan.EOF}
	value.Errorf(format, args...)
}

// Line reads a line of input and returns the values it evaluates.
// A nil returned slice means there were no values.
// The boolean reports whether the line is valid.
//
// Line
//	) special command '\n'
//	def function defintion
//	expressionList '\n'
func (p *Parser) Line() ([]value.Expr, bool) {
	tok := p.peek()
	switch tok.Type {
	case scan.RightParen:
		p.special()
		return nil, true
	case scan.Def:
		p.functionDefn()
		return nil, true
	}
	exprs, ok := p.expressionList()
	if !ok {
		return nil, false
	}
	return exprs, true
}

// expressionList:
//	'\n'
//	statementList '\n'
func (p *Parser) expressionList() ([]value.Expr, bool) {
	tok := p.next()
	switch tok.Type {
	case scan.Error:
		p.errorf("%q", tok)
	case scan.EOF:
		return nil, false
	case scan.Newline:
		return nil, true
	}
	exprs, ok := p.statementList(tok)
	if !ok {
		return nil, false
	}
	tok = p.next()
	switch tok.Type {
	case scan.Error:
		p.errorf("%q", tok)
	case scan.EOF, scan.Newline:
	default:
		p.errorf("unexpected %q", tok)
	}
	return exprs, ok
}

// statementList:
//	statement
//	statement ';' statement
//
// statement:
//	var ':=' Expr
//	Expr
func (p *Parser) statementList(tok scan.Token) ([]value.Expr, bool) {
	expr, ok := p.statement(tok)
	if !ok {
		return nil, false
	}
	var exprs []value.Expr
	if expr != nil {
		exprs = []value.Expr{expr}
	}
	if p.peek().Type == scan.Semicolon {
		p.next()
		more, ok := p.statementList(p.next())
		if ok {
			exprs = append(exprs, more...)
		}
	}
	return exprs, true
}

// statement:
//	var '=' Expr
//	Expr
func (p *Parser) statement(tok scan.Token) (value.Expr, bool) {
	variableName := ""
	if tok.Type == scan.Identifier {
		if p.peek().Type == scan.Assign {
			p.next()
			variableName = tok.Text
			tok = p.next()
		}
	}
	expr := p.expr(tok)
	if expr == nil {
		return nil, true
	}
	if variableName != "" {
		expr = &assignment{
			variable: p.variable(variableName),
			expr:     expr,
		}
	}
	if p.config.Debug("parse") {
		fmt.Println(expr)
	}
	return expr, true
}

// expr
//	operand
//	operand binop expr
func (p *Parser) expr(tok scan.Token) value.Expr {
	expr := p.operand(tok, true)
	tok = p.peek()
	switch tok.Type {
	case scan.Newline, scan.EOF, scan.RightParen, scan.RightBrack, scan.Semicolon:
		return expr
	case scan.Identifier:
		function := p.binaryFn[tok.Text]
		if function != nil {
			p.next()
			// User-defined binary.
			return &binaryCall{
				fn:    function,
				left:  expr,
				right: p.expr(p.next()),
			}
		}
	case scan.Operator:
		// Binary.
		p.next()
		return &binary{
			left:  expr,
			op:    tok.Text,
			right: p.expr(p.next()),
		}
	}
	p.errorf("after expression: unexpected %s", p.peek())
	return nil
}

// operand
//	number
//	vector
//	operand [ Expr ]...
//	unop Expr
func (p *Parser) operand(tok scan.Token, indexOK bool) value.Expr {
	var expr value.Expr
	switch tok.Type {
	case scan.Operator:
		// Unary.
		expr = &unary{
			op:    tok.Text,
			right: p.expr(p.next()),
		}
	case scan.Identifier:
		function := p.unaryFn[tok.Text]
		if function != nil {
			// User-defined unary.
			expr = &unaryCall{
				fn:  function,
				arg: p.expr(p.next()),
			}
			break
		}
		fallthrough
	case scan.Number, scan.Rational, scan.LeftParen:
		expr = p.numberOrVector(tok)
	default:
		p.errorf("unexpected %s", tok)
	}
	if indexOK {
		expr = p.index(expr)
	}
	return expr
}

// index
//	expr
//	expr [ expr ]
//	expr [ expr ] [ expr ] ....
func (p *Parser) index(expr value.Expr) value.Expr {
	for p.peek().Type == scan.LeftBrack {
		p.next()
		index := p.expr(p.next())
		tok := p.next()
		if tok.Type != scan.RightBrack {
			p.errorf("expected right bracket, found %s", tok)
		}
		expr = &binary{
			op:    "[]",
			left:  expr,
			right: index,
		}
	}
	return expr
}

// number
//	integer
//	rational
//	variable
//	'(' Expr ')'
func (p *Parser) number(tok scan.Token) value.Expr {
	var expr value.Expr
	text := tok.Text
	switch tok.Type {
	case scan.Identifier:
		expr = p.variable(text)
	case scan.Number, scan.Rational:
		var err error
		expr, err = value.Parse(text)
		if err != nil {
			p.errorf("%s: %s", text, err)
		}
	case scan.LeftParen:
		expr = p.expr(p.next())
		tok := p.next()
		if tok.Type != scan.RightParen {
			p.errorf("expected right paren, found %s", tok)
		}
	}
	return expr
}

// numberOrVector turns the token and what follows into a numeric Value, possibly a vector.
// numberOrVector
//	number ...
func (p *Parser) numberOrVector(tok scan.Token) value.Expr {
	expr := p.number(tok)
	switch p.peek().Type {
	case scan.Number, scan.Rational, scan.Identifier, scan.LeftParen:
		// Further vector elements follow.
	default:
		return expr
	}
	slice := sliceExpr{expr}
Loop:
	for {
		tok = p.peek()
		switch tok.Type {
		case scan.LeftParen:
			fallthrough
		case scan.Identifier:
			if p.unaryFn[tok.Text] != nil || p.binaryFn[tok.Text] != nil {
				break Loop
			}
			fallthrough
		case scan.Number, scan.Rational:
			expr = p.number(p.next())
		default:
			break Loop
		}
		slice = append(slice, expr)
	}
	if len(slice) == 1 {
		return slice[0] // Just a singleton.
	}
	return slice
}

func isScalar(v value.Value) bool {
	switch v.(type) {
	case value.Int, value.BigInt, value.BigRat:
		return true
	}
	return false
}

func (p *Parser) variable(name string) variableExpr {
	return variableExpr{
		name: name,
	}
}
