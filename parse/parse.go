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

type unary struct {
	op    string
	right value.Expr
}

func (u *unary) String() string {
	return u.op + " " + u.right.String()
}

func (u *unary) Eval() value.Value {
	return value.Unary(u.op, u.right.Eval())
}

type binary struct {
	op    string
	left  value.Expr
	right value.Expr
}

func (b *binary) String() string {
	return b.left.String() + " " + b.op + " " + b.right.String()
}

func (b *binary) Eval() value.Value {
	return value.Binary(b.left.Eval(), b.op, b.right.Eval())
}

// Tree prints a representation of the expression tree e.
func Tree(e value.Expr) string {
	switch e := e.(type) {
	case nil:
		return ""
	case value.BigInt:
		return fmt.Sprintf("<big %s>", e)
	case value.BigRat:
		return fmt.Sprintf("<rat %s>", e)
	case value.Int:
		return fmt.Sprintf("<%s>", e)
	case value.Vector:
		return fmt.Sprintf("<vec %s>", e)
	case *unary:
		return fmt.Sprintf("(%s %s)", e.op, Tree(e.right))
	case *binary:
		return fmt.Sprintf("(%s %s %s)", Tree(e.left), e.op, Tree(e.right))
	default:
		return fmt.Sprintf("%T", e)
	}
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
}

var zero, _ = value.Parse("0")

// NewParser returns a new parser that will read from the scanner.
func NewParser(conf *config.Config, fileName string, scanner *scan.Scanner) *Parser {
	return &Parser{
		scanner:  scanner,
		config:   conf,
		fileName: fileName,
		vars:     make(map[string]value.Value),
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
// Line:
//	'\n'
//	) special command '\n'
//	statementList '\n'
func (p *Parser) Line() ([]value.Value, bool) {
	tok := p.next()
	switch tok.Type {
	case scan.Error:
		p.errorf("%q", tok)
	case scan.EOF:
		return nil, false
	case scan.Newline:
		return nil, true
	case scan.RightParen:
		p.special()
		return nil, true
	}
	values, ok := p.statementList(tok)
	if !ok {
		return values, false
	}
	tok = p.next()
	switch tok.Type {
	case scan.Error:
		p.errorf("%q", tok)
	case scan.EOF, scan.Newline:
	default:
		p.errorf("unexpected %q", tok)
	}
	return values, ok
}

// statementList:
//	statement
//	statement ';' statement
//
// statement:
//	var ':=' Expr
//	Expr
func (p *Parser) statementList(tok scan.Token) ([]value.Value, bool) {
	v, ok := p.statement(tok)
	if !ok {
		return nil, false
	}
	var values []value.Value
	if v != nil {
		values = []value.Value{v}
	}
	if p.peek().Type == scan.Semicolon {
		p.next()
		more, ok := p.statementList(p.next())
		if ok {
			values = append(values, more...)
		}
	}
	return values, true
}

// statement:
//	var ':=' Expr
//	Expr
func (p *Parser) statement(tok scan.Token) (value.Value, bool) {
	variableName := ""
	if tok.Type == scan.Identifier {
		next := p.peek()
		if next.Type == scan.Assign {
			p.next()
			variableName = tok.Text
			tok = p.next()
		}
	}
	x := p.expr(tok)
	if x == nil {
		return nil, true
	}
	if p.config.Debug("parse") {
		fmt.Println(Tree(x))
	}
	expr := x.Eval()
	p.vars["_"] = expr // Will end up assigned to last expression on line.
	if variableName != "" {
		p.vars[variableName] = expr
		return nil, true // No value returned.
	}
	return expr, true
}

// expr
//	operand
//	operand binop expr
func (p *Parser) expr(tok scan.Token) value.Expr {
	expr := p.operand(tok)
	switch p.peek().Type {
	case scan.Newline, scan.EOF, scan.RightParen, scan.RightBrack, scan.Semicolon:
		return expr
	case scan.Operator:
		// Binary.
		tok = p.next()
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
//	( Expr )
//	( Expr ) [ Expr ]...
//	operand
//	number
//	rational
//	vector
//	variable
//	operand [ Expr ]...
//	unop Expr
func (p *Parser) operand(tok scan.Token) value.Expr {
	var expr value.Expr
	switch tok.Type {
	case scan.Operator:
		// Unary.
		expr = &unary{
			op:    tok.Text,
			right: p.expr(p.next()),
		}
	case scan.LeftParen:
		expr = p.expr(p.next())
		tok := p.next()
		if tok.Type != scan.RightParen {
			p.errorf("expected right paren, found %s", tok)
		}
	case scan.Number, scan.Rational:
		expr = p.numberOrVector(tok)
	case scan.Identifier:
		expr = p.vars[tok.Text]
		if expr == nil {
			p.errorf("%s undefined", tok.Text)
		}
	default:
		p.errorf("unexpected %s", tok)
	}
	return p.index(expr)
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

// number turns the token into a singleton numeric Value.
func (p *Parser) number(tok scan.Token) value.Value {
	x, err := value.Parse(tok.Text)
	if err != nil {
		p.errorf("%s: %s", tok.Text, err)
	}
	return x
}

// numberOrVector turns the token and what follows into a numeric Value, possibly a vector.
func (p *Parser) numberOrVector(tok scan.Token) value.Value {
	x := p.number(tok)
	typ := p.peek().Type
	if typ != scan.Number && typ != scan.Rational {
		return x
	}
	v := []value.Value{x}
	for typ == scan.Number || typ == scan.Rational {
		v = append(v, p.number(p.next()))
		typ = p.peek().Type
	}
	return value.NewVector(v)
}
