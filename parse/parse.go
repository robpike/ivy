// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse // import "robpike.io/ivy/parse"

import (
	"bytes"
	"fmt"
	"strconv"
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
	case sliceExpr:
		s := "<"
		for i, x := range e {
			if i > 0 {
				s += " "
			}
			s += x.ProgString()
		}
		s += ">"
		return s
	case variableExpr:
		return fmt.Sprintf("<var %s>", e.name)
	case *unary:
		return fmt.Sprintf("(%s %s)", e.op, tree(e.right))
	case *binary:
		return fmt.Sprintf("(%s %s %s)", tree(e.left), e.op, tree(e.right))
	case conditional:
		return tree(e.binary)
	case *index:
		s := fmt.Sprintf("(%s[", tree(e.left))
		for i, v := range e.right {
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

// sliceExpr holds a syntactic vector to be verified and evaluated.
type sliceExpr []value.Expr

func (s sliceExpr) Eval(context value.Context) value.Value {
	v := make([]value.Value, len(s))
	// First do all assignments. These two vectors are legal.
	// y (y=3) and (y=3) y.
	for i, x := range s {
		if bin, ok := x.(*binary); ok && bin.op == "=" {
			s[i] = x.Eval(context)
		}
	}
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

var charEscape = map[rune]string{
	'\\': "\\\\",
	'\'': "\\'",
	'\a': "\\a",
	'\b': "\\b",
	'\f': "\\f",
	'\n': "\\n",
	'\r': "\\r",
	'\t': "\\t",
	'\v': "\\v",
}

func (s sliceExpr) ProgString() string {
	var b bytes.Buffer
	// If it's all Char, we can do a prettier job.
	if s.allChars() {
		b.WriteRune('\'')
		for _, v := range s {
			c := rune(v.(value.Char))
			esc := charEscape[c]
			if esc != "" {
				b.WriteString(esc)
				continue
			}
			if !strconv.IsPrint(c) {
				if c <= 0xFFFF {
					fmt.Fprintf(&b, "\\u%04x", c)
				} else {
					fmt.Fprintf(&b, "\\U%08x", c)
				}
				continue
			}
			b.WriteRune(c)
		}
		b.WriteRune('\'')
	} else {
		for i, v := range s {
			if i > 0 {
				b.WriteRune(' ')
			}
			if isCompound(v) {
				b.WriteString("(" + v.ProgString() + ")")
			} else {
				b.WriteString(v.ProgString())
			}
		}
	}
	return b.String()
}

func (s sliceExpr) allChars() bool {
	for _, c := range s {
		if _, ok := c.(value.Char); !ok {
			return false
		}
	}
	return true
}

// variableExpr identifies a variable to be looked up and evaluated.
type variableExpr struct {
	name string
}

func (e variableExpr) Eval(context value.Context) value.Value {
	v := context.Lookup(e.name)
	if v == nil {
		value.Errorf("undefined variable %q", e.name)
	}
	return v
}

func (e variableExpr) ProgString() string {
	return e.name
}

// isCompound reports whether the item is a non-trivial expression tree, one that
// may require parentheses around it when printed to maintain correct evaluation order.
func isCompound(x interface{}) bool {
	switch x := x.(type) {
	case value.Char, value.Int, value.BigInt, value.BigRat, value.BigFloat, value.Vector, value.Matrix:
		return false
	case sliceExpr, variableExpr:
		return false
	case *index:
		return isCompound(x.left)
	default:
		return true
	}
}

type unary struct {
	op    string
	right value.Expr
}

func (u *unary) ProgString() string {
	return fmt.Sprintf("%s %s", u.op, u.right.ProgString())
}

func (u *unary) Eval(context value.Context) value.Value {
	return context.EvalUnary(u.op, u.right.Eval(context).Inner())
}

type binary struct {
	op    string
	left  value.Expr
	right value.Expr
}

func (b *binary) ProgString() string {
	var left string
	if isCompound(b.left) {
		left = fmt.Sprintf("(%s)", b.left.ProgString())
	} else {
		left = b.left.ProgString()
	}
	return fmt.Sprintf("%s %s %s", left, b.op, b.right.ProgString())
}

func (b *binary) Eval(context value.Context) value.Value {
	if b.op == "=" {
		return assignment(context, b)
	}
	rhs := b.right.Eval(context).Inner()
	lhs := b.left.Eval(context)
	return context.EvalBinary(lhs, b.op, rhs)
}

type index struct {
	op    string
	left  value.Expr
	right []value.Expr
}

func (x *index) ProgString() string {
	var s strings.Builder
	if isCompound(x.left) {
		s.WriteString("(")
		s.WriteString(x.left.ProgString())
		s.WriteString(")")
	} else {
		s.WriteString(x.left.ProgString())
	}
	s.WriteString("[")
	for i, v := range x.right {
		if i > 0 {
			s.WriteString("; ")
		}
		s.WriteString(v.ProgString())
	}
	s.WriteString("]")
	return s.String()
}

func (x *index) Eval(context value.Context) value.Value {
	return value.Index(context, x, x.left, x.right)
}

// conditional is a conditional executor: expression ":" expression
type conditional struct {
	*binary // Implements Expr through embedding.
}

var _ = value.Decomposable(conditional{})

func (c conditional) Operator() string {
	return ":"
}

func (c conditional) Operands() (left, right value.Expr) {
	return c.left, c.right
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
// The context must have have been created by this package's NewContext function.
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

// Line reads a line of input and returns the values it evaluates.
// A nil returned slice means there were no values.
// The boolean reports whether the line is valid.
//
// Line
//	) special command '\n'
//	def function defintion
//	expressionList '\n'
func (p *Parser) Line() ([]value.Expr, bool) {
	var ok bool
	if !p.readTokensToNewline() {
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
		p.functionDefn()
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
func (p *Parser) readTokensToNewline() bool {
	p.tokens = p.tokenBuf[:0]
	for {
		tok := p.scanner.Next()
		switch tok.Type {
		case scan.Error:
			p.errorf("%s", tok)
		case scan.Newline:
			return true
		case scan.EOF:
			return len(p.tokens) > 0
		}
		p.tokens = append(p.tokens, tok)
	}
}

// expressionList:
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
	if len(exprs) > 0 && p.context.Config().Debug("parse") {
		p.Println(tree(exprs))
	}
	return exprs, ok
}

// statementList:
//	expr [':' expr] [';' statementList]
func (p *Parser) statementList() ([]value.Expr, bool) {
	expr := p.expr()
	if expr != nil && p.peek().Type == scan.Colon {
		tok := p.next()
		expr = conditional{
			&binary{
				left:  expr,
				op:    tok.Text,
				right: p.expr(),
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
//	operand
//	operand binop expr
func (p *Parser) expr() value.Expr {
	tok := p.next()
	expr := p.operand(tok, true)
	tok = p.peek()
	switch tok.Type {
	case scan.EOF, scan.RightParen, scan.RightBrack, scan.Semicolon, scan.Colon:
		return expr
	case scan.Identifier:
		if p.context.DefinedBinary(tok.Text) {
			p.next()
			return &binary{
				left:  expr,
				op:    tok.Text,
				right: p.expr(),
			}
		}
	case scan.Assign:
		p.next()
		switch lhs := expr.(type) {
		case variableExpr, *index:
			return &binary{
				left:  lhs,
				op:    tok.Text,
				right: p.expr(),
			}
		}
		p.errorf("cannot assign to %s", expr.ProgString())
	case scan.Operator:
		p.next()
		return &binary{
			left:  expr,
			op:    tok.Text,
			right: p.expr(),
		}
	}
	p.errorf("after expression: unexpected %s", p.peek())
	return nil
}

// operand
//	number
//	char constant
//	string constant
//	vector
//	operand [ Expr ]...
//	unop Expr
func (p *Parser) operand(tok scan.Token, indexOK bool) value.Expr {
	var expr value.Expr
	switch tok.Type {
	case scan.Operator:
		expr = &unary{
			op:    tok.Text,
			right: p.expr(),
		}
	case scan.Identifier:
		if p.context.DefinedUnary(tok.Text) {
			expr = &unary{
				op:    tok.Text,
				right: p.expr(),
			}
			break
		}
		fallthrough
	case scan.Number, scan.Rational, scan.String, scan.LeftParen:
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
		list := []value.Expr{p.expr()}
		tok := p.next()
		for tok.Type == scan.Semicolon {
			list = append(list, p.expr())
			tok = p.next()
		}
		if tok.Type != scan.RightBrack {
			p.errorf("expected right bracket, found %s", tok)
		}
		expr = &index{
			left:  expr,
			right: list,
		}
	}
	return expr
}

// number
//	integer
//	rational
//	string
//	variable
//	'(' Expr ')'
// If the value is a string, value.Expr is nil.
func (p *Parser) number(tok scan.Token) (expr value.Expr, str string) {
	var err error
	text := tok.Text
	switch tok.Type {
	case scan.Identifier:
		expr = p.variable(text)
	case scan.String:
		str = value.ParseString(text)
	case scan.Number, scan.Rational:
		expr, err = value.Parse(p.context.Config(), text)
	case scan.LeftParen:
		expr = p.expr()
		tok := p.next()
		if tok.Type != scan.RightParen {
			p.errorf("expected right paren, found %s", tok)
		}
	}
	if err != nil {
		p.errorf("%s: %s", text, err)
	}
	return expr, str
}

// numberOrVector turns the token and what follows into a numeric Value, possibly a vector.
// numberOrVector
//	number
//	string
//	numberOrVector...
func (p *Parser) numberOrVector(tok scan.Token) value.Expr {
	expr, str := p.number(tok)
	done := true
	switch p.peek().Type {
	case scan.Number, scan.Rational, scan.String, scan.Identifier, scan.LeftParen:
		// Further vector elements follow.
		done = false
	}
	var slice sliceExpr
	if expr == nil {
		// Must be a string.
		slice = append(slice, evalString(str)...)
	} else {
		slice = sliceExpr{expr}
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
			case scan.Number, scan.Rational, scan.String:
				expr, str = p.number(p.next())
				if expr == nil {
					// Must be a string.
					slice = append(slice, evalString(str)...)
					continue
				}
			default:
				break Loop
			}
			slice = append(slice, expr)
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

func (p *Parser) variable(name string) variableExpr {
	return variableExpr{
		name: name,
	}
}

// evalString turns a parsed string constant into a slice of
// value.Exprs each of which is a value.Char.
func evalString(str string) []value.Expr {
	r := ([]rune)(str)
	v := make([]value.Expr, len(r))
	for i, c := range r {
		v[i] = value.Char(c)
	}
	return v
}
