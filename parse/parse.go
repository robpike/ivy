// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import (
	"fmt"
	"log"
	"os"

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
	switch u.op {
	case "+":
		return u.right.Eval()
	case "-":
		return u.right.Eval().Neg()
	case "iota":
		return u.right.Eval().Iota()
	}
	panic(value.Errorf("no implementation of unary %s", u.op))
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
	switch b.op {
	case "+":
		return b.left.Eval().Add(b.right.Eval())
	case "-":
		return b.left.Eval().Sub(b.right.Eval())
	case "*":
		return b.left.Eval().Mul(b.right.Eval())
	case "/":
		return b.left.Eval().Div(b.right.Eval())
	case "**":
		return b.left.Eval().Pow(b.right.Eval())
	case "<<":
		return b.left.Eval().Lsh(b.right.Eval())
	case ">>":
		return b.left.Eval().Rsh(b.right.Eval())
	case "&":
		return b.left.Eval().And(b.right.Eval())
	case "|":
		return b.left.Eval().Or(b.right.Eval())
	case "^":
		return b.left.Eval().Xor(b.right.Eval())
	}
	panic(value.Errorf("binary %s unimplemented", b.op))
}

func Tree(e value.Expr) string {
	switch e := e.(type) {
	case nil:
		return ""
	case value.BigInt:
		return fmt.Sprintf("<big %s>", e)
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
	errorLine  int // Line number of last error.
	errorCount int // Number of errors.
	peekTok    scan.Token
}

func NewParser(lexer lex.TokenReader) *Parser {
	return &Parser{
		lexer: lexer,
	}
}

func (p *Parser) Next() scan.Token {
	tok := p.peekTok
	if tok.Type != scan.Nothing {
		p.peekTok = scan.Token{Type: scan.Nothing}
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
	if tok.Type != scan.Nothing {
		return tok
	}
	p.peekTok = p.lexer.Next()
	return p.peekTok
}

func (p *Parser) errorf(format string, args ...interface{}) {
	if p.lineNum == p.errorLine {
		// Only one error per line.
		return
	}
	p.errorLine = p.lineNum
	// Put file and line information on head of message.
	format = "%s:%d: " + format + "\n"
	args = append([]interface{}{p.lexer.FileName(), p.lineNum}, args...)
	fmt.Fprintf(os.Stderr, format, args...)
	p.errorCount++
	if p.errorCount > 10 {
		log.Fatal("too many errors")
	}
}

// Line:
//	EOF
//	'\n'
//	Expr '\n'
func (p *Parser) Line() (value.Expr, bool) {
	tok := p.Next()
	switch tok.Type {
	case scan.EOF:
		return nil, false
	case scan.Newline:
		return nil, true
	default:
		x := p.Expr(tok)
		tok = p.Next()
		if tok.Type != scan.Newline {
			p.errorf("unexpected %q", tok)
		}
		return x, true
	}
}

// Expr
//	Operand
//	Operand binop Expr
// Left associative, so "1+2+3" is "(1+2)+3".
func (p *Parser) Expr(tok scan.Token) value.Expr {
	expr := p.Operand(tok)
Loop:
	for {
		switch p.Peek().Type {
		case scan.Newline, scan.RightParen:
			break Loop
		case scan.Char, scan.LeftShift, scan.RightShift, scan.Exponent:
			// Binary.
			tok = p.Next()
			op := tok.Text
			switch op {
			case "+", "-", "*", "/", "**", "<<", ">>", "&", "|", "^":
			default:
				p.errorf("unexpected %q", tok)
			}
			expr = &Binary{
				left:  expr,
				op:    op,
				right: p.Operand(p.Next()),
			}
		default:
			panic(value.Errorf("unexpected %s after expression", p.Peek()))
		}
	}
	return expr
}

// Operand
//	( Expr )
//	Number
//	Vector
//	unop Operand
func (p *Parser) Operand(tok scan.Token) value.Expr {
	var expr value.Expr
	switch tok.Type {
	case scan.Char:
		// Unary.
		op := tok.Text
		switch op {
		case "+", "-":
		default:
			p.errorf("unexpected %q", tok)
		}
		expr = &Unary{
			op:    op,
			right: p.Operand(p.Next()),
		}
	case scan.Identifier:
		// Magic words.
		op := tok.Text
		switch tok.Text {
		case "iota":
		default:
			p.errorf("unexpected %q", tok)
		}
		expr = &Unary{
			op:    op,
			right: p.Operand(p.Next()),
		}
	case scan.LeftParen:
		expr = p.Expr(p.Next())
		tok := p.Next()
		if tok.Type != scan.RightParen {
			p.errorf("expected right paren, found", tok)
		}
	case scan.Number:
		expr = p.NumberOrVector(tok)
	default:
		panic(value.Errorf("unexpected %s", tok))
	}
	return expr
}

// Number turns the token into a singleton numeric Value.
func (p *Parser) Number(tok scan.Token) value.Value {
	x, ok := value.ValueString(tok.Text)
	if !ok {
		panic(value.Errorf("syntax error in number: %s", tok.Text))
	}
	return x
}

// NumberOrVector turns the token and what follows into a numeric Value, possibly a vector.
func (p *Parser) NumberOrVector(tok scan.Token) value.Value {
	x := p.Number(tok)
	if p.Peek().Type != scan.Number {
		return x
	}
	v := []value.Value{x}
	for p.Peek().Type == scan.Number {
		v = append(v, p.Number(p.Next()))
	}
	return value.ValueSlice(v)
}
