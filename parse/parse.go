// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import (
	"fmt"

	"code.google.com/p/rspace/ivy/lex"
	"code.google.com/p/rspace/ivy/scan"
	"code.google.com/p/rspace/ivy/value"
)

type Unary struct {
	op    string
	right value.Expr
}

func (u *Unary) String() string {
	return u.op + " " + u.right.String()
}

func (u *Unary) Eval() value.Value {
	return value.Unary(u.op, u.right.Eval())
}

type Binary struct {
	op    string
	left  value.Expr
	right value.Expr
}

func (b *Binary) String() string {
	return b.left.String() + " " + b.op + " " + b.right.String()
}

func (b *Binary) Eval() value.Value {
	return value.Binary(b.left.Eval(), b.op, b.right.Eval())
}

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
	case *Unary:
		return fmt.Sprintf("(%s %s)", e.op, Tree(e.right))
	case *Binary:
		return fmt.Sprintf("(%s %s %s)", Tree(e.left), e.op, Tree(e.right))
	default:
		return fmt.Sprintf("%T", e)
	}
}

type Parser struct {
	lexer      lex.TokenReader
	lineNum    int
	errorCount int // Number of errors.
	peekTok    scan.Token
	vars       map[string]value.Value
}

var zero, _ = value.ValueString("0")

func NewParser(lexer lex.TokenReader) *Parser {
	return &Parser{
		lexer:   lexer,
		lineNum: 1,
		vars:    make(map[string]value.Value),
	}
}

func (p *Parser) Next() scan.Token {
	tok := p.peekTok
	if tok.Type != scan.EOF {
		p.peekTok = scan.Token{Type: scan.EOF}
	} else {
		tok = p.lexer.Next()
	}
	return tok
}

func (p *Parser) Back(tok scan.Token) {
	p.peekTok = tok
}

func (p *Parser) Peek() scan.Token {
	tok := p.peekTok
	if tok.Type != scan.EOF {
		return tok
	}
	p.peekTok = p.lexer.Next()
	return p.peekTok
}

func (p *Parser) errorf(format string, args ...interface{}) {
	// Flush to newline.
	for p.Next().Type != scan.Newline {
	}
	// Put file and line information on head of message.
	format = "%s:%d: " + format + "\n"
	args = append([]interface{}{p.lexer.FileName(), p.lineNum}, args...)
	panic(value.Errorf(format, args...))
}

// Line:
//	EOF
//	'\n'
//	var ':=' Expr
//	Expr '\n'
func (p *Parser) Line() (value.Value, bool) {
	tok := p.Next()
	variable := ""
	switch tok.Type {
	case scan.EOF:
		return nil, false
	case scan.Error:
		p.errorf("%q", tok)
		return nil, false
	case scan.Newline:
		return nil, true
	case scan.Identifier:
		next := p.Peek()
		if next.Type == scan.ColonEquals {
			p.Next()
			variable = tok.Text
			tok = p.Next()
		}
		fallthrough
	default:
		x := p.Expr(tok)
		if x == nil {
			return nil, true
		}
		tok = p.Next()
		if tok.Type != scan.Newline {
			p.errorf("unexpected %q", tok)
		}
		fmt.Println(Tree(x))
		expr := x.Eval()
		p.vars["_"] = expr
		if variable != "" {
			p.vars[variable] = expr
		}
		return expr, true
	}
}

// Expr
//	Operand
//	Operand binop Expr
func (p *Parser) Expr(tok scan.Token) value.Expr {
	expr := p.Operand(tok)
	switch p.Peek().Type {
	case scan.Newline, scan.RightParen:
		return expr
	case scan.Operator:
		// Binary.
		tok = p.Next()
		return &Binary{
			left:  expr,
			op:    tok.Text,
			right: p.Expr(p.Next()),
		}
	}
	p.errorf("unexpected %s after expression", p.Peek())
	return nil
}

// Operand
//	( Expr )
//	Number
//	Rational
//	Vector
//	variable
//	unop Expr
func (p *Parser) Operand(tok scan.Token) value.Expr {
	var expr value.Expr
	switch tok.Type {
	case scan.Operator:
		// Unary.
		op := tok.Text
		if p.Peek().Text == `\` {
			// Reduce operation.
			op += p.Next().Text
		}
		expr = &Unary{
			op:    op,
			right: p.Expr(p.Next()),
		}
	case scan.LeftParen:
		expr = p.Expr(p.Next())
		tok := p.Next()
		if tok.Type != scan.RightParen {
			p.errorf("expected right paren, found %s", tok)
		}
	case scan.Number, scan.Rational:
		expr = p.NumberOrVector(tok)
	case scan.Identifier:
		expr = p.vars[tok.Text]
		if expr == nil {
			p.errorf("%s undefined", tok.Text)
		}
	default:
		p.errorf("unexpected %s", tok)
	}
	return expr
}

// Number turns the token into a singleton numeric Value.
func (p *Parser) Number(tok scan.Token) value.Value {
	x, err := value.ValueString(tok.Text)
	if err != nil {
		p.errorf("%s: %s", tok.Text, err)
	}
	return x
}

// NumberOrVector turns the token and what follows into a numeric Value, possibly a vector.
func (p *Parser) NumberOrVector(tok scan.Token) value.Value {
	x := p.Number(tok)
	typ := p.Peek().Type
	if typ != scan.Number && typ != scan.Rational {
		return x
	}
	v := []value.Value{x}
	for typ == scan.Number || typ == scan.Rational {
		v = append(v, p.Number(p.Next()))
		typ = p.Peek().Type
	}
	return value.ValueSlice(v)
}
