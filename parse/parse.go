// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse // import "robpike.io/ivy/parse"

import (
	"fmt"
	"slices"
	"strings"

	"robpike.io/ivy/exec"
	"robpike.io/ivy/scan"
	"robpike.io/ivy/value"
)

// tree formats an expression in an unambiguous form for debugging.
func tree(e interface{}) string {
	switch e := e.(type) {
	case value.Int:
		return fmt.Sprintf("<int %s>", e)
	case value.BigInt:
		return fmt.Sprintf("<bigint %s>", e)
	case value.BigRat:
		return fmt.Sprintf("<rat %s>", e)
	case value.BigFloat:
		return fmt.Sprintf("<float %s>", e)
	case value.Complex:
		return fmt.Sprintf("<complex %s>", e)
	case value.VectorExpr:
		s := "<"
		for i, x := range e {
			if i > 0 {
				s += " "
			}
			s += x.ProgString()
		}
		s += ">"
		return s
	case *value.VarExpr:
		return fmt.Sprintf("<var %s>", e.Name)
	case *value.UnaryExpr:
		return fmt.Sprintf("(%s %s)", e.Op, tree(e.Right))
	case *value.BinaryExpr:
		return fmt.Sprintf("(%s %s %s)", tree(e.Left), e.Op, tree(e.Right))
	case *value.CondExpr:
		return tree(e.Cond)
	case *value.IndexExpr:
		s := fmt.Sprintf("(%s[", tree(e.Left))
		for i, v := range e.Right {
			if i > 0 {
				s += "; "
			}
			s += tree(v)
		}
		s += "])"
		return s
	case []value.Expr:
		if len(e) == 1 {
			return tree(e[0])
		}
		s := "<"
		for i, expr := range e {
			if i > 0 {
				s += "; "
			}
			s += tree(expr)
		}
		s += ">"
		return s
	default:
		return fmt.Sprintf("%T", e)
	}
}

// Parser stores the state for the ivy parser.
type Parser struct {
	scanner  *scan.Scanner
	tokens   []scan.Token    // Points to tokenBuf.
	tokenBuf [100]scan.Token // Reusable.
	fileName string
	lineNum  int
	context  *exec.Context
}

// NewParser returns a new parser that will read from the scanner.
// The context must have been created by this package's NewContext function.
func NewParser(fileName string, scanner *scan.Scanner, context value.Context) *Parser {
	return &Parser{
		scanner:  scanner,
		fileName: fileName,
		context:  context.(*exec.Context),
	}
}

// Printf formats the args and writes them to the configured output writer.
func (p *Parser) Printf(format string, args ...interface{}) {
	fmt.Fprintf(p.context.Config().Output(), format, args...)
}

// Println prints the args and writes them to the configured output writer.
func (p *Parser) Println(args ...interface{}) {
	fmt.Fprintln(p.context.Config().Output(), args...)
}

// Print prints the args and writes them to the configured output writer.
func (p *Parser) Print(args ...interface{}) {
	fmt.Fprint(p.context.Config().Output(), args...)
}

func (p *Parser) next() scan.Token {
	tok := p.peek()
	if tok.Type != scan.EOF {
		p.tokens = p.tokens[1:]
		p.lineNum = tok.Line // This gives us the line number before the newline.
	}
	if tok.Type == scan.Error {
		p.errorf("%s", tok)
	}
	return tok
}

func (p *Parser) peek() scan.Token {
	if len(p.tokens) == 0 {
		return scan.Token{Type: scan.EOF}
	}
	return p.tokens[0]
}

var eof = scan.Token{
	Type: scan.EOF,
}

// Loc returns the current input location in the form "name:line: ".
// If the name is <stdin>, it returns the empty string.
func (p *Parser) Loc() string {
	if p.fileName == "<stdin>" {
		return ""
	}
	return fmt.Sprintf("%s:%d: ", p.fileName, p.lineNum)
}

func (p *Parser) errorf(format string, args ...interface{}) {
	p.tokens = p.tokenBuf[:0]
	value.Errorf(format, args...)
}

// source returns the source code spanning the start and end lines.
func (p *Parser) source(start, end int) string {
	src := strings.Builder{}
	for _, s := range p.scanner.History()[start:end] {
		src.WriteString(s)
		src.WriteByte('\n')
	}
	return src.String()
}

// Line reads a line of input and returns the values it evaluates.
// A nil returned slice means there were no values.
// The boolean reports whether the line is valid.
//
// Line
//
//	) special command '\n'
//	op function definition
//	expressionList '\n'
func (p *Parser) Line() ([]value.Expr, bool) {
	var ok bool
	start := len(p.scanner.History()) // Remember this location before any leading comments.
	if !p.readTokensToNewline(false) {
		return nil, false
	}
	tok := p.peek()
	switch tok.Type {
	case scan.EOF:
		return nil, true
	case scan.RightParen:
		p.special()
		p.context.SetConstants()
		return nil, true
	case scan.Op:
		p.functionDefn(start)
		return nil, true
	}
	exprs, ok := p.expressionList()
	if !ok {
		return nil, false
	}
	return exprs, true
}

// readTokensToNewline returns the next line of input.
// The boolean is false at EOF.
// We read all tokens before parsing for easy error recovery
// if an error occurs mid-line. It also gives us lookahead
// for parsing, which we may use one day.
func (p *Parser) readTokensToNewline(inFunction bool) bool {
	p.tokens = p.tokenBuf[:0]
	for {
		tok := p.scanner.Next()
		switch tok.Type {
		case scan.Error:
			p.errorf("%s", tok)
		case scan.Newline:
			// Need a truly blank line to terminate the function body.
			if !inFunction || len(tok.Text) <= 1 || len(p.tokens) > 0 {
				return true
			}
			continue
		case scan.EOF:
			if inFunction && len(p.tokens) == 0 {
				// EOF is fine for terminating a function.
				return true
			}
			return len(p.tokens) > 0
		}
		p.tokens = append(p.tokens, tok)
	}
}

// expressionList:
//
//	statementList <eol>
func (p *Parser) expressionList() ([]value.Expr, bool) {
	exprs, ok := p.statementList()
	if !ok {
		return nil, false
	}
	tok := p.next()
	switch tok.Type {
	case scan.EOF: // Expect to be at end of line.
	default:
		p.errorf("unexpected %s", tok)
	}
	if len(exprs) > 0 && p.context.Config().Debug("parse") > 0 {
		p.Println(tree(exprs))
	}
	return exprs, ok
}

// statementList:
//
//	expr [':' expr] [';' statementList]
func (p *Parser) statementList() ([]value.Expr, bool) {
	expr := p.expr()
	if expr != nil && p.peek().Type == scan.Colon {
		tok := p.next()
		expr = &value.CondExpr{
			Cond: &value.BinaryExpr{
				Left:  expr,
				Op:    tok.Text,
				Right: p.expr(),
			},
		}
	}
	var exprs []value.Expr
	if expr != nil {
		exprs = []value.Expr{expr}
	}
	if p.peek().Type == scan.Semicolon {
		p.next()
		more, ok := p.statementList()
		if ok {
			exprs = append(exprs, more...)
		}
	}
	return exprs, true
}

// expr
//
//	operand
//	operand binop expr
func (p *Parser) expr() value.Expr {
	tok := p.next()
	expr := p.operand(tok)
	tok = p.peek()
	switch tok.Type {
	case scan.EOF, scan.RightParen, scan.RightBrack, scan.Semicolon, scan.Colon:
		return expr
	case scan.Identifier:
		if p.context.DefinedBinary(tok.Text) {
			p.next()
			return &value.BinaryExpr{
				Left:  expr,
				Op:    tok.Text,
				Right: p.expr(),
			}
		}
	case scan.Assign:
		p.next()
		p.checkAssign(expr)
		return &value.BinaryExpr{
			Left:  expr,
			Op:    tok.Text,
			Right: p.expr(),
		}
	case scan.Operator:
		p.next()
		return &value.BinaryExpr{
			Left:  expr,
			Op:    tok.Text,
			Right: p.expr(),
		}
	}
	p.errorf("after expression: unexpected %s", p.peek())
	return nil
}

// checkAssign checks that e is assignable.
func (p *Parser) checkAssign(e value.Expr) {
	switch e := e.(type) {
	default:
		p.errorf("cannot assign to %s", e.ProgString())
	case *value.VarExpr:
		// ok
	case *value.IndexExpr:
		switch e.Left.(type) {
		case *value.VarExpr:
			// ok
		case *value.IndexExpr:
			// Old x[i][j]. Show new syntax.
			var list []value.Expr
			var last value.Expr
			for x := e; x != nil; x, _ = x.Left.(*value.IndexExpr) {
				list = append(list, x.Right...)
				last = x.Left
			}
			slices.Reverse(list)
			fixed := &value.IndexExpr{Left: last, Right: list}
			value.Errorf("cannot assign to %s; use %v", e.ProgString(), fixed.ProgString())
		}
	case value.VectorExpr:
		for _, elem := range e {
			p.checkAssign(elem)
		}
	}
}

// operand
//
//	number
//	char constant
//	string constant
//	vector
//	unop Expr
func (p *Parser) operand(tok scan.Token) value.Expr {
	var expr value.Expr
	switch tok.Type {
	case scan.Operator:
		expr = &value.UnaryExpr{
			Op:    tok.Text,
			Right: p.expr(),
		}
	case scan.Identifier:
		if p.context.DefinedUnary(strings.Trim(tok.Text, "@")) {
			expr = &value.UnaryExpr{
				Op:    tok.Text,
				Right: p.expr(),
			}
			break
		}
		fallthrough
	case scan.Number, scan.Rational, scan.Complex, scan.String, scan.LeftParen:
		expr = p.numberOrVector(tok)
	default:
		p.errorf("unexpected %s", tok)
	}
	return expr
}

// index
//
//	expr
//	expr [ expr ]
//	expr [ expr ] [ expr ] ....
func (p *Parser) index(expr value.Expr) value.Expr {
	for p.peek().Type == scan.LeftBrack {
		p.next()
		list := p.indexList()
		tok := p.next()
		if tok.Type != scan.RightBrack {
			p.errorf("expected right bracket, found %s", tok)
		}
		expr = &value.IndexExpr{
			Left:  expr,
			Right: list,
		}
	}
	return expr
}

// indexList
//
//	[[expr] [';' [expr]] ...]
func (p *Parser) indexList() []value.Expr {
	list := []value.Expr{}
	exprSeen := false // Previous element contained an expression.
	for {
		tok := p.peek()
		switch tok.Type {
		case scan.RightBrack:
			if !exprSeen {
				list = append(list, nil) // "v[]" means all of v.
			}
			return list
		case scan.Semicolon:
			p.next()
			if !exprSeen {
				list = append(list, nil)
			}
			exprSeen = false
		default:
			list = append(list, p.expr())
			exprSeen = true
		}
	}
}

// number
//
//	integer
//	rational
//	string
//	variable
//	'(' ')'
//	'(' Expr ')'
//
// If the value is a string, value.Expr is nil.
func (p *Parser) number(tok scan.Token) (expr value.Expr, str string) {
	var err error
	text := tok.Text
	switch tok.Type {
	case scan.Identifier:
		expr = p.variable(text)
	case scan.String:
		str = value.ParseString(text)
	case scan.Number, scan.Rational, scan.Complex:
		expr, err = value.Parse(p.context.Config(), text)
	case scan.LeftParen:
		if p.peek().Type == scan.RightParen {
			p.next()
			expr = value.VectorExpr{}
		} else {
			expr = p.expr()
			tok := p.next()
			if tok.Type != scan.RightParen {
				p.errorf("expected right paren, found %s", tok)
			}
		}
	}
	if err != nil {
		p.errorf("%s: %s", text, err)
	}
	return expr, str
}

// numberOrVector turns the token and what follows into a numeric Value, possibly a vector.
// numberOrVector
//
//	number
//	string
//	numberOrVector '[' Expr ']'
//	numberOrVector...
func (p *Parser) numberOrVector(tok scan.Token) value.Expr {
	expr, str := p.number(tok)
	done := true
	switch p.peek().Type {
	case scan.Number, scan.Rational, scan.Complex, scan.String, scan.Identifier, scan.LeftParen, scan.LeftBrack:
		// Further work follows.
		done = false
	}
	var slice value.VectorExpr
	if expr == nil {
		// Must be a string.
		slice = value.VectorExpr{p.index(evalString(str))}
	} else {
		slice = value.VectorExpr{p.index(expr)}
	}
	if !done {
	Loop:
		for {
			tok = p.peek()
			switch tok.Type {
			case scan.LeftParen:
				fallthrough
			case scan.Identifier:
				if p.context.DefinedOp(tok.Text) {
					break Loop
				}
				fallthrough
			case scan.Number, scan.Rational, scan.Complex, scan.String:
				expr, str = p.number(p.next())
				if expr == nil {
					// Must be a string.
					expr = evalString(str)
				}
			default:
				break Loop
			}
			slice = append(slice, expr)
			if p.peek().Type == scan.LeftBrack {
				// Replace the whole slice so far with the index expression slice[next expression].
				expr = p.index(slice)
				slice = append(value.VectorExpr{}, expr)
			}
		}
	}
	if len(slice) == 1 {
		return slice[0] // Just a singleton.
	}
	return slice
}

func isScalar(v value.Value) bool {
	return v.Rank() == 0
}

func (p *Parser) variable(name string) *value.VarExpr {
	return &value.VarExpr{
		Name: name,
	}
}

// evalString turns a string constant into an Expr
// that is either a single Char or a slice of Chars.
func evalString(str string) value.Expr {
	r := ([]rune)(str)
	if len(r) == 1 {
		return value.Char(r[0])
	}
	v := make([]value.Expr, len(r))
	for i, c := range r {
		v[i] = value.Char(c)
	}
	return value.VectorExpr(v)
}
