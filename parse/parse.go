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
	}
	panic(value.Errorf("no implementation of binary %s", b.op))
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

func (p *Parser) Line() (value.Expr, bool) {
	var expr value.Expr
Loop:
	for {
		// We save the line number here so error messages from this line
		// are labeled with this source line. Otherwise we complain after we've absorbed
		// the terminating newline and the line numbers are off by one in errors.
		p.lineNum = p.lexer.Line()
		tok := p.Next()
		switch tok.Type {
		case scan.Newline:
			break Loop
		case scan.Space:
			continue
		case scan.EOF:
			return expr, false
		case scan.Identifier:
			op := tok.Text
			switch tok.Text {
			case "iota":
			default:
				p.errorf("unexpected %q", tok)
			}
			tok = p.Next()
			if expr == nil {
				expr = &Unary{
					op:    "iota",
					right: p.operand(tok),
				}
			} else {
				expr = &Binary{
					op:    op,
					left:  expr,
					right: p.operand(tok),
				}
			}
			continue
		case scan.Char:
			op := tok.Text
			switch op {
			case "+", "-", "*", "/":
			default:
				p.errorf("unexpected %q", tok)
			}
			tok = p.Next()
			if expr == nil {
				expr = &Unary{
					op:    op,
					right: p.operand(tok),
				}
			} else {
				expr = &Binary{
					op:    op,
					left:  expr,
					right: p.operand(tok),
				}
			}
			continue
		case scan.Number:
			expr = p.operand(tok)
			continue
		}
		p.errorf("unexpected %s", tok)
	}
	if p.errorCount > 0 {
		return nil, true
	}
	return expr, true
}

// sitting on the first number.
func (p *Parser) operand(tok scan.Token) value.Value {
	var v []value.Value
	for {
		for tok.Type == scan.Space {
			tok = p.Next()
		}
		if tok.Type != scan.Number {
			p.Back(tok)
			break
		}
		x, ok := value.Set(tok.Text)
		if !ok {
			panic(value.Errorf("syntax error in number: %s", tok.Text))
		}
		v = append(v, x)
		tok = p.Next()
	}
	if len(v) == 1 {
		return v[0]
	}
	return value.SetVector(v)
}
