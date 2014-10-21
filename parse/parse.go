// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import (
	"fmt"
	"log"
	"os"
	"text/scanner"

	"code.google.com/p/rspace/ivy/lex"
	"code.google.com/p/rspace/ivy/value"
)

type Expr struct {
	left  value.Value // Always present.
	right value.Value // Present if a binop.
	op    string      // Absent means no op.
}

func (e *Expr) String() string {
	if e == nil {
		return ""
	}
	if e.right == nil {
		return e.op + " " + e.left.String()
	}
	return e.left.String() + " " + e.op + " " + e.right.String()
}

func (e *Expr) Eval() value.Value {
	if e == nil {
		panic(value.Error("nil expression"))
	}
	if e.left == nil {
		panic(value.Error("no left"))
	}
	if e.op == "" {
		return e.left
	}
	if e.right == nil {
		panic(value.Errorf("implemented unop %s", e.op))
	}
	switch e.op {
	case "+":
		return e.left.Add(e.right)
	case "-":
		return e.left.Sub(e.right)
	case "*":
		return e.left.Mul(e.right)
	case "/":
		return e.left.Div(e.right)
	}
	panic(value.Errorf("no implementation of binop %s", e.op))
}

type Parser struct {
	lexer      lex.TokenReader
	lineNum    int
	errorLine  int // Line number of last error.
	errorCount int // Number of errors.
}

func NewParser(lexer lex.TokenReader) *Parser {
	return &Parser{
		lexer: lexer,
	}
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

func (p *Parser) Line() (*Expr, bool) {
	var expr *Expr
	tok := p.lexer.Next()
Loop:
	for {
		// We save the line number here so error messages from this line
		// are labeled with this source line. Otherwise we complain after we've absorbed
		// the terminating newline and the line numbers are off by one in errors.
		p.lineNum = p.lexer.Line()
		text := p.lexer.Text()
		switch tok {
		case '\n':
			break Loop
		case scanner.EOF:
			return expr, false
		case '+', '-', '*', '/':
			if expr == nil {
				expr = new(Expr)
			}
			if expr.op != "" {
				panic(value.Errorf("syntax error in operand: %s", text))
			}
			expr.op = text
			tok = p.lexer.Next()
			continue
		case scanner.Int:
			var v value.Value
			v, tok = p.operand()
			if expr == nil {
				expr = new(Expr)
			}
			if expr.left == nil {
				expr.left = v
			} else if expr.right == nil {
				expr.right = v
			} else {
				panic(value.Errorf("not yet with expr tree: %s", text))
			}
			continue
		default:
			p.errorf("unexpected %q", p.lexer.Text())
		}
		break
	}
	if p.errorCount > 0 {
		return nil, true
	}
	return expr, true
}

func (p *Parser) operand() (value.Value, lex.Token) {
	var v []value.Value
	var tok lex.Token
	for {
		text := p.lexer.Text()
		x, ok := value.Set(text)
		if !ok {
			panic(value.Errorf("syntax error in number: %s", text))
		}
		v = append(v, x)
		tok = p.lexer.Next()
		if tok != scanner.Int {
			break
		}
	}
	if len(v) == 1 {
		return v[0], tok
	}
	return value.SetVector(v), tok
}

/*
// WORD op {, op} '\n'
func (p *Parser) line() bool {
	// Skip newlines.
	var tok Token
	for {
		tok = p.lex.Next()
		// We save the line number here so error messages from this instruction
		// are labeled with this line. Otherwise we complain after we've absorbed
		// the terminating newline and the line numbers are off by one in errors.
		p.lineNum = p.lex.Line()
		switch tok {
		case '\n':
			continue
		case scanner.EOF:
			return false
		}
		break
	}
	// First item must be an identifier.
	if tok != scanner.Ident {
		p.errorf("expected identifier, found %q", p.lex.Text())
		return false // Might as well stop now.
	}
	word := p.lex.Text()
	operands := make([][]LexToken, 0, 3)
	// Zero or more comma-separated operands, one per loop.
	first := true // Permit ':' to define this as a label.
	for tok != '\n' && tok != ';' {
		// Process one operand.
		items := make([]LexToken, 0, 3)
		for {
			tok = p.lex.Next()
			if first {
				if tok == ':' {
					p.pendingLabels = append(p.pendingLabels, word)
					return true
				}
				first = false
			}
			if tok == scanner.EOF {
				p.errorf("unexpected EOF")
				return false
			}
			if tok == '\n' || tok == ';' || tok == ',' {
				break
			}
			items = append(items, LexToken{tok, p.lex.Text()})
		}
		if len(items) > 0 {
			operands = append(operands, items)
		} else if len(operands) > 0 {
			// Had a comma but nothing after.
			p.errorf("missing operand")
		}
	}
	i := p.arch.pseudos[word]
	if i != 0 {
		p.pseudo(i, word, operands)
		return true
	}
	i = p.arch.instructions[word]
	if i != 0 {
		p.instruction(i, word, operands)
		return true
	}
	p.errorf("unrecognized instruction %s", word)
	return true
}
*/

/*
func (p *Parser) instruction(op int, word string, operands [][]LexToken) {
	p.addr = p.addr[0:0]
	for _, op := range operands {
		p.addr = append(p.addr, p.address(op))
	}
	// Is it a jump? TODO
	if word[0] == 'J' || word == "CALL" {
		p.asmJump(op, p.addr)
		return
	}
	p.asmInstruction(op, p.addr)
}


func (p *Parser) start(operand []LexToken) {
	p.input = operand
	p.inputPos = 0
}

// address parses the operand into a link address structure.
func (p *Parser) address(operand []LexToken) Addr {
	p.start(operand)
	addr := Addr{}
	p.operand(&addr)
	return addr
}

// parse (R). The opening paren is known to be there.
// The return value states whether it was a scaled mode.
func (p *Parser) parenRegister(a *Addr) bool {
	p.next()
	tok := p.next()
	if tok.Token != scanner.Ident {
		p.errorf("expected register, got %s", tok.text)
	}
	r, present := p.arch.registers[tok.text]
	if !present {
		p.errorf("expected register, found %s", tok.text)
	}
	a.isIndirect = true
	scaled := p.peek() == '*'
	if scaled {
		// (R*2)
		p.next()
		tok := p.get(scanner.Int)
		a.scale = p.scale(tok.text)
		a.index = r
	} else {
		if a.hasRegister {
			p.errorf("multiple indirections")
		}
		a.hasRegister = true
		a.register = r
	}
	p.expect(')')
	p.next()
	return scaled
}

// scale converts a decimal string into a valid scale factor.
func (p *Parser) scale(s string) int8 {
	switch s {
	case "1", "2", "4", "8":
		return int8(s[0] - '0')
	}
	p.errorf("bad scale: %s", s)
	return 0
}

// parse (R) or (R)(R*scale). The opening paren is known to be there.
func (p *Parser) addressMode(a *Addr) {
	scaled := p.parenRegister(a)
	if !scaled && p.peek() == '(' {
		p.parenRegister(a)
	}
}

// operand parses a general operand and stores the result in *a.
func (p *Parser) operand(a *Addr) bool {
	if len(p.input) == 0 {
		p.errorf("empty operand: cannot happen")
		return false
	}
	switch p.peek() {
	case '$':
		p.next()
		switch p.peek() {
		case scanner.Ident:
			a.isImmediateAddress = true
			p.operand(a) // TODO
		case scanner.String:
			a.isImmediateConstant = true
			a.hasString = true
			a.string = p.atos(p.next().text)
		case scanner.Int, scanner.Float, '+', '-', '~', '(':
			a.isImmediateConstant = true
			if p.have(scanner.Float) {
				a.hasFloat = true
				a.float = p.floatExpr()
			} else {
				a.hasOffset = true
				a.offset = int64(p.expr())
			}
		default:
			p.errorf("illegal %s in immediate operand", p.next().text)
		}
	case '*':
		p.next()
		tok := p.next()
		r, present := p.arch.registers[tok.text]
		if !present {
			p.errorf("expected register; got %s", tok.text)
		}
		a.hasRegister = true
		a.register = r
	case '(':
		p.next()
		if p.peek() == scanner.Ident {
			p.back()
			p.addressMode(a)
			break
		}
		p.back()
		fallthrough
	case '+', '-', '~', scanner.Int, scanner.Float:
		if p.have(scanner.Float) {
			a.hasFloat = true
			a.float = p.floatExpr()
		} else {
			a.hasOffset = true
			a.offset = int64(p.expr())
		}
		if p.peek() != scanner.EOF {
			p.expect('(')
			p.addressMode(a)
		}
	case scanner.Ident:
		tok := p.next()
		// Either R or (most general) ident<>+4(SB)(R*scale).
		if r, present := p.arch.registers[tok.text]; present {
			a.hasRegister = true
			a.register = r
			// Possibly register pair: DX:AX.
			if p.peek() == ':' {
				p.next()
				tok = p.get(scanner.Ident)
				a.hasRegister2 = true
				a.register2 = p.arch.registers[tok.text]
			}
			break
		}
		// Weirdness with statics: Might now have "<>".
		if p.peek() == '<' {
			p.next()
			p.get('>')
			a.isStatic = true
		}
		if p.peek() == '+' || p.peek() == '-' {
			a.hasOffset = true
			a.offset = int64(p.expr())
		}
		a.symbol = tok.text
		if p.peek() == scanner.EOF {
			break
		}
		// Expect (SB) or (FP)
		p.expect('(')
		p.parenRegister(a)
		if a.register != rSB && a.register != rFP && a.register != rSP {
			p.errorf("expected SB, FP, or SP offset for %s", tok.text)
		}
		// Possibly have scaled register (CX*8).
		if p.peek() != scanner.EOF {
			p.expect('(')
			p.addressMode(a)
		}
	default:
		p.errorf("unexpected %s in operand", p.next().text)
	}
	p.expect(scanner.EOF)
	return true
}

// expr = term | term '+' term
func (p *Parser) expr() uint64 {
	value := p.term()
	for {
		switch p.peek() {
		case '+':
			p.next()
			x := p.term()
			if addOverflows(x, value) {
				p.errorf("overflow in %d+%d", value, x)
			}
			value += x
		case '-':
			p.next()
			value -= p.term()
		case '|':
			p.next()
			value |= p.term()
		case '^':
			p.next()
			value ^= p.term()
		default:
			return value
		}
	}
}

// floatExpr = fconst | '-' floatExpr | '+' floatExpr | '(' floatExpr ')'
func (p *Parser) floatExpr() float64 {
	tok := p.next()
	switch tok.Token {
	case '(':
		v := p.floatExpr()
		if p.next().Token != ')' {
			p.errorf("missing closing paren")
		}
		return v
	case '+':
		return +p.floatExpr()
	case '-':
		return -p.floatExpr()
	case scanner.Float:
		return p.atof(tok.text)
	}
	p.errorf("unexpected %s evaluating float expression", tok.text)
	return 0
}

// term = const | term '*' term | '(' expr ')'
func (p *Parser) term() uint64 {
	tok := p.next()
	switch tok.Token {
	case '(':
		v := p.expr()
		if p.next().Token != ')' {
			p.errorf("missing closing paren")
		}
		return v
	case '+':
		return +p.term()
	case '-':
		return -p.term()
	case '~':
		return ^p.term()
	case scanner.Int:
		value := p.atoi(tok.text)
		for {
			switch p.peek() {
			case '*':
				p.next()
				value *= p.term() // OVERFLOW?
			case '/':
				p.next()
				value /= p.term()
			case '%':
				p.next()
				value %= p.term()
			case LSH:
				p.next()
				shift := p.term()
				if shift < 0 {
					p.errorf("negative left shift %d", shift)
				}
				value <<= uint(shift)
			case RSH:
				p.next()
				shift := p.term()
				if shift < 0 {
					p.errorf("negative right shift %d", shift)
				}
				value >>= uint(shift)
			case '&':
				p.next()
				value &= p.term()
			default:
				return value
			}
		}
	}
	p.errorf("unexpected %s evaluating expression", tok.text)
	return 0
}

func (p *Parser) atoi(str string) uint64 {
	value, err := strconv.ParseUint(str, 0, 64)
	if err != nil {
		p.errorf("%s", err)
	}
	return value
}

func (p *Parser) atof(str string) float64 {
	value, err := strconv.ParseFloat(str, 64)
	if err != nil {
		p.errorf("%s", err)
	}
	return value
}

func (p *Parser) atos(str string) string {
	value, err := strconv.Unquote(str)
	if err != nil {
		p.errorf("%s", err)
	}
	return value
}

var end = LexToken{scanner.EOF, "end"}

func (p *Parser) next() LexToken {
	if !p.more() {
		return end
	}
	tok := p.input[p.inputPos]
	p.inputPos++
	return tok
}

func (p *Parser) back() {
	p.inputPos--
}

func (p *Parser) peek() Token {
	if p.more() {
		return p.input[p.inputPos].Token
	}
	return scanner.EOF
}

func (p *Parser) more() bool {
	return p.inputPos < len(p.input)
}

// get verifies that the next item has the expected type and returns it.
func (p *Parser) get(expected Token) LexToken {
	p.expect(expected)
	return p.next()
}

// expect verifies that the next item has the expected type. It does not consume it.
func (p *Parser) expect(expected Token) {
	if p.peek() != expected {
		p.errorf("expected %s, found %s", expected, p.next().text)
	}
}

// have reports whether the remaining tokens contain the specified token.
func (p *Parser) have(token Token) bool {
	for i := p.inputPos; i < len(p.input); i++ {
		if p.input[i].Token == token {
			return true
		}
	}
	return false
}
*/
