// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"robpike.io/ivy/scan"
)

// A Statement is an expression, typically a line of input or element of a
// user-defined op. Unlike other expressions, it does not parse when created.
// Instead, only when first evaluated (through the Eval method) does it turn into a
// parsed expression. This is because the parse itself depends on external context
// that may be different at evaluation time than creation time, perhaps due to
// originally undefined variables or operators that have been defined since
// creation. One way this can arise is through mutual recursion:
//
// op foo x = x==0: 1; bar x-1
// op bar z = z==0: 0; foo z-1
//
// Another issue is that whether an identifier is a variable or op may also depend
// on when it is evaluated:
//
// 1 j 2  # j is a binary operator; result 1j2
// j = 5  # j is now a variable
// 1 j 2  # result: 1 5 2
//
// Earlier versions of ivy parsed immediately, which caused a number of minor but
// niggling issues, including not being able to use j as a variable.
//
type Statement struct {
	tokens     []scan.Token // Tokens that built this Statement. Locked at creation time.
	c          Context      // Set only during parse.
	pos        int          // Token index in pTokens.
	last       scan.Token   // Last scanned token, even from peek.
	inOperator bool         // Part of a function body, for :ret.
	fileName   string       // Name of input stream.
	parsed     Expr         // Saved parse, flushed when global state changes.
}

var _ Expr = (*Statement)(nil)

// NewStatement creates a statement (expression) defined by the tokens.
// inOperator records whether the statement is part of a user-defined op,
// which when true admits :ret expressions.
func NewStatement(tokens []scan.Token, fileName string, inOperator bool) *Statement {
	return &Statement{
		tokens:     tokens,
		inOperator: inOperator,
		fileName:   fileName,
	}
}

// VarsAndRet returns a list of identifiers (which may or may not be variables;
// we find out when evaluating) in the body of the statement. It also reports
// whther the statement has a :ret.
func (s *Statement) VarsAndRet() ([]string, bool) {
	varNames := make(map[string]bool)
	hasRet := false
	for _, t := range s.tokens {
		switch {
		case t.Type == scan.Ret:
			hasRet = true
		case t.Type == scan.Identifier:
			varNames[t.Text] = true
		case strings.HasPrefix(t.Text, "o.") && identifierLike(t.Text[2:]):
			varNames[t.Text[2:]] = true
		case strings.Contains(t.Text, "."):
			dot := strings.IndexRune(t.Text, '.')
			l, r := t.Text[:dot], t.Text[dot+1:]
			switch {
			case identifierLike(l):
				varNames[l] = true
			case identifierLike(r):
				varNames[r] = true
			}

		}
	}
	vars := make([]string, 0, len(varNames))
	for s := range varNames {
		vars = append(vars, s)
	}
	return vars, hasRet
}

func (s *Statement) ProgString() string {
	// Not a great solution, but this is only needed for debugging.
	str := "("
	for i, t := range s.tokens {
		if i > 0 {
			str += " "
		}
		str += fmt.Sprint(t.Text)
	}
	return str + ")"
}

func (s *Statement) Eval(c Context) Value {
	e := s.Parse(c)
	v := e.Eval(c)
	return v
}

// Called only during parsing.
func (s *Statement) Errorf(format string, args ...interface{}) {
	s.c.SetPos(s.fileName, s.last.Line, s.last.Offset)
	s.c.Errorf(format, args...)
}

// Parse parses the Statement into an Expr.
// Because it's the APL way, we parse right to left.
func (s *Statement) Parse(c Context) Expr {
	if s.parsed != nil {
		return s.parsed
	}
	s.c = c
	defer func() { s.c = nil }()
	s.pos = len(s.tokens)
	expr := s.parseStatement(c)
	if s.peek().Type != scan.EOF {
		s.Errorf("extra %q at beginning of expression", s.peek().Text)
	}
	s.parsed = expr
	return expr
}

func (s *Statement) eofTok() scan.Token {
	return scan.Token{
		Type:   scan.EOF,
		Line:   s.last.Line,
		Offset: s.last.Offset,
		Text:   "EOF",
	}
}

// peek returns the token to the left without consuming it.
func (s *Statement) peek() scan.Token {
	tok := s.prev()
	if tok.Type != scan.EOF {
		s.pos++
	}
	return tok
}

// prev returns the token to the left and consumes it.
func (s *Statement) prev() scan.Token {
	if s.pos == 0 {
		return s.eofTok()
	}
	s.pos--
	tok := s.tokens[s.pos]
	s.last = tok
	return tok
}

//
// statement
//	expr
//	expr ":" expr
//
func (s *Statement) parseStatement(c Context) Expr {
	expr := s.expr(c)
	if expr != nil && s.peek().Type == scan.Colon {
		s.prev()
		c := &ColonExpr{
			Cond:  s.expr(c),
			Value: expr,
		}
		expr = c
	}
	return expr
}

// expr
//
//	operand
//	unaryOp expr
//	operand binaryOp expr
//
func (s *Statement) expr(c Context) Expr {
	expr := s.operand(c)
	for {
		tok := s.peek()
		switch tok.Type {
		case scan.EOF, scan.LeftParen, scan.LeftBrack, scan.Colon, scan.Semicolon, scan.If, scan.Elif, scan.Else, scan.While:
			return expr
		case scan.Assign:
			s.prev() // Eat the =
			vars := s.operand(c)
			s.checkAssign(c, vars)
			expr = &BinaryExpr{
				file:   s.fileName,
				line:   tok.Line,
				offset: tok.Offset,
				Left:   vars,
				Op:     tok.Text,
				Right:  expr,
			}
		case scan.Operator, scan.Identifier:
			op := s.buildOperator(c)
			if op.isBinary {
				expr = &BinaryExpr{
					file:   s.fileName,
					line:   tok.Line,
					offset: tok.Offset,
					Left:   s.operand(c),
					Op:     op.str,
					Right:  expr,
				}
			} else {
				expr = &UnaryExpr{
					file:   s.fileName,
					line:   tok.Line,
					offset: tok.Offset,
					Op:     op.str,
					Right:  expr,
				}
			}
		case scan.Ret:
			if !s.inOperator {
				s.Errorf(":ret outside operator definition")
			}
			s.prev()
			expr = &RetExpr{
				Expr: expr,
			}
		default:
			s.Errorf("expression syntax error")
		}
	}
}

// operand
//
//	element
//	element [element...]
//
func (s *Statement) operand(c Context) Expr {
	expr := s.element(c)
	if !s.atOperand(c) {
		return expr
	}
	slice := []Expr{expr}
	for s.atOperand(c) {
		slice = append(slice, s.element(c))
	}
	// Reverse the slice. With a lot more code (or an n² in the previous loop)
	// we could avoid this but the reversal is cheap.
	for i, j := 0, len(slice)-1; i < j; i, j = i+1, j-1 {
		slice[i], slice[j] = slice[j], slice[i]
	}
	return VectorExpr(slice)
}

// element
//
//	number
//	identifier
//	string
//	expr "[" [[expr] [";"]]... "]"
//	":if" expr ";" statementList [":elif" expr ';' statementList]... [":else" statementList] ":end"
//	":while" expr ";" statementList ":end"
//	'(' expr ')'
//
//	statementList: [";"] statement [";" statement [";"]]...
//
//	Newlines are converted to semicolons in Parser.statementList.
//
func (s *Statement) element(c Context) Expr {
	tok := s.prev()
	switch tok.Type {
	default:
		s.Errorf("expected operand; found %q", tok.Text)
	case scan.Number:
		expr, err := Parse(c, tok.Text)
		if err != nil {
			s.Errorf("%s", err)
		}
		return expr
	case scan.Identifier:
		return &VarExpr{
			file:   s.fileName,
			line:   tok.Line,
			offset: tok.Offset,
			Name:   tok.Text,
		}
	case scan.String:
		str, err := ParseString(c, tok.Text)
		if err != nil {
			s.Errorf("%s", err)
		}
		return stringToValue(c, str)
	case scan.RightParen:
		var expr Expr = VectorExpr{}
		if s.peek().Type != scan.LeftParen { // Empty parens are valid expression.
			expr = s.expr(c)
			if s.peek().Type != scan.LeftParen {
				s.Errorf("missing left parenthesis")
			}
		}
		s.prev()
		return expr
	case scan.RightBrack:
		var indexes = []Expr{}
		if s.peek().Type != scan.LeftBrack {
			if s.peek().Type != scan.Semicolon {
				indexes = []Expr{s.expr(c)}
			}
			for s.peek().Type == scan.Semicolon {
				var ix Expr
				s.prev()
				if s.atOperand(c) {
					ix = s.expr(c)
				} else {
					ix = nil
				}
				indexes = append([]Expr{ix}, indexes...) // n squared but n is tiny
			}
		}
		if tok := s.prev(); tok.Type != scan.LeftBrack {
			s.Errorf("missing left bracket at %q", tok.Text)
		}
		vars := s.operand(c)
		i := &IndexExpr{
			Left:  vars,
			Right: indexes,
		}
		return i
	case scan.End:
		// For these control structures, the elements are statements, not expressions,
		// so we can have colon expressions within.
		s.optionalSemicolons()
		exprs := []Expr{s.parseStatement(c)}
		var elseBody StatementList
		for {
			s.optionalSemicolons()
			switch s.peek().Type {
			case scan.EOF:
				s.Errorf("if/while syntax error")
			case scan.If:
				s.prev() // Eat the :if.
				cond, body := s.condBody(c, exprs, tok.Text)
				return &IfExpr{
					Cond:     cond,
					Body:     body,
					ElseBody: elseBody,
				}
			case scan.Else:
				if elseBody != nil {
					s.Errorf("syntax error at :else")
				}
				s.prev() // Eat the :else.
				elseBody = exprs
				exprs = nil
			case scan.Elif:
				s.prev() // Eat the :elif.
				cond, body := s.condBody(c, exprs, tok.Text)
				ifExpr := &IfExpr{
					Cond:     cond,
					Body:     body,
					ElseBody: elseBody,
				}
				elseBody = []Expr{ifExpr}
				exprs = nil
			case scan.While:
				if elseBody != nil {
					s.Errorf("syntax error at :while")
				}
				s.prev() // Eat the :while.
				cond, body := s.condBody(c, exprs, tok.Text)
				return &WhileExpr{
					Cond: cond,
					Body: body,
				}
			}
			s.optionalSemicolons()
			exprs = append([]Expr{s.parseStatement(c)}, exprs...)
		}
	}
	panic("not reached")
}

// adjacent reports whether the next token on the left abuts the argument token.
func (s *Statement) adjacent(right scan.Token) bool {
	left := s.peek()
	return left.Offset+len(left.Text) == right.Offset
}

// operator is the object returned by buildOperator. It holds an operator
// we should be able to finally evaluate, including all decorators.
type operator struct {
	rhs      scan.Token // Starting token (after @s), no decorators.
	str      string
	isBinary bool
}

// isProduct looks for an inner or our outer product, and updates op if found.
func (s *Statement) isProduct(c Context, op *operator) bool {
	if !s.adjacent(op.rhs) { // Not necessary now, but has always been required in ivy.
		return false
	}
	// Outer product
	if s.peek().Text == "o." {
		op.str = s.prev().Text + op.str
		if !isBinaryOp(c, op.rhs.Text) {
			s.Errorf("outer product requires binary operator: %s", op.str)
		}
		op.isBinary = true
		return true
	}
	// Inner product.
	if s.peek().Text != "." {
		return false
	}
	dot := s.prev()
	if !s.adjacent(dot) {
		s.Errorf("inner product syntax: no operator next to '.'")
	}
	lhs := s.prev().Text
	op.str = lhs + "." + op.str
	if !isBinaryOp(c, lhs) || !isBinaryOp(c, op.rhs.Text) {
		s.Errorf("inner product requires binary operators: %s", op.str)
	}
	op.isBinary = true
	return true

}

// buildOperator constructs an operator from the token stream, taking into
// account @ iteration, reductions and scans, and the arity of operators.
// The result depends on the current execution state and in general cannot
// be done by the scanner.
// It's a monster with many things to consider. It's also in delicate
// balance with the scanner.
func (s *Statement) buildOperator(c Context) operator {
	post := ""
	tok := s.peek()
	for ; tok.Text == "@"; tok = s.peek() {
		post += s.prev().Text
		if !s.adjacent(tok) {
			s.Errorf("%q not attached to operator", post)
		}
	}
	tok = s.prev()
	offset := tok.Offset
	op := operator{
		rhs:      tok,
		str:      tok.Text + post,
		isBinary: false,
	}
	switch {
	case tok.Text == "%":
		peek := s.peek()
		if peek.Text != "/" && peek.Text != "\\" || !s.adjacent(tok) { // ",%" comes as a unit; it's binary.
			break
		}
		tok = s.prev()
		op.str = tok.Text + op.str
		fallthrough
	case tok.Text == "/", tok.Text == "\\":
		// Might be a reduction or expansion.
		isOp := false
		if s.adjacent(tok) {
			tok = s.peek()
			switch tok.Type {
			case scan.EOF, scan.LeftParen: // TODO MORE?
				// Out of room, must be unary inverse.
				op.isBinary = false
				isOp = true
			case scan.Operator:
				// A reduction if this is a binary operator.
				if BinaryOps[tok.Text] != nil {
					op.isBinary = false
					s.prev()
					op.str = tok.Text + op.str
					isOp = true
				}
			case scan.Identifier:
				if isVariable(c, tok.Text) { // Just a division.
					break
				}
				// A (unary) reduction/expansion if this is a binary.
				if isBinaryOp(c, tok.Text) {
					op.isBinary = false
					s.prev()
					op.str = tok.Text + op.str
					isOp = true
				}
			}
		}
		if !isOp {
			// Just a division.
			op.isBinary = s.atOperand(c)
		}
	case tok.Type == scan.Identifier, tok.Type == scan.Operator:
		if s.isProduct(c, &op) {
			break
		}
		lhsAt := s.peek().Text == "@" && s.adjacent(tok)
		switch {
		case tok.Type == scan.Identifier && isVariable(c, tok.Text): // Just a variable on the lhs.
		case isBinaryOp(c, tok.Text) && (lhsAt || s.atOperand(c)):
			op.isBinary = true
		case isUnaryOp(c, tok.Text):
			op.isBinary = false
		case isBinaryOp(c, tok.Text):
			// We have a binary but the syntax needs a unary.
			s.Errorf("%q is not a unary operator", tok.Text)
		default:
			s.Errorf("%q is not an operator", tok.Text)
		}
	}
	peekTok := s.peek()
	if peekTok.Text == "@" && peekTok.Offset == offset-1 { // Don't attach non-adjacent @s.
		pre := ""
		for s.peek().Text == "@" {
			pre += s.prev().Text
		}
		op.str = pre + op.str
	}
	return op
}

// checkAssign checks that e is an assignable value.
func (s *Statement) checkAssign(c Context, e Expr) {
	switch e := e.(type) {
	default:
		s.Errorf("cannot assign to %s", e.ProgString())
	case *VarExpr:
		// ok
	case *IndexExpr:
		switch e.Left.(type) {
		case *VarExpr:
			// ok
		case *IndexExpr:
			// Old x[i][j]. Show new syntax.
			var list []Expr
			var last Expr
			for x := e; x != nil; x, _ = x.Left.(*IndexExpr) {
				list = append(list, x.Right...)
				last = x.Left
			}
			slices.Reverse(list)
			fixed := &IndexExpr{Left: last, Right: list}
			s.Errorf("cannot assign to %s; use %s", e.ProgString(), fixed.ProgString())
		}
	case VectorExpr:
		for _, elem := range e {
			s.checkAssign(c, elem)
		}
	}
}

func isBinaryOp(c Context, id string) bool {
	return BinaryOps[id] != nil || c.UserDefined(id, true)
}

func isUnaryOp(c Context, id string) bool {
	return UnaryOps[id] != nil || c.UserDefined(id, false)
}

func isVariable(c Context, id string) bool {
	if c.Global(id) != nil {
		return true
	}
	return c.IsLocal(id)
}

// identifierLike reports whether s looks like an identifier. It does not check the symbol table.
// Used only in VarsAndRet.
func identifierLike(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, r := range s {
		isDigit := unicode.IsDigit(r)
		if r != '_' && !unicode.IsLetter(r) && !isDigit {
			return false
		}
		if i == 0 && isDigit {
			return false
		}
	}
	return true
}

// atOperand reports whether the scanning position has an operand on the left.
func (s *Statement) atOperand(c Context) bool {
	tok := s.peek()
	switch tok.Type {
	case scan.Number, scan.String:
		return true
	case scan.RightParen, scan.RightBrack, scan.End:
		return true
	case scan.Identifier:
		// Can't be an operator.
		if isVariable(c, tok.Text) {
			return true
		}
		if isBinaryOp(c, tok.Text) || isUnaryOp(c, tok.Text) {
			return false
		}
		return true
	}
	return false
}

// optionalSemicolons elides needless semicolons in :if and :while statements.
// No need to be fussy about separator vs. terminator and empty statements.
func (s *Statement) optionalSemicolons() {
	for s.peek().Type == scan.Semicolon {
		s.prev()
	}
}

// condBody takes the statement list from inside an :if or :while and separates the first
// item, the condition, from the rest of the body, returning both.
func (s *Statement) condBody(c Context, stmts StatementList, word string) (Expr, StatementList) {
	if len(stmts) == 0 {
		s.Errorf("missing condition for %s", word)
	}
	return stmts[0], stmts[1:]
}

// stringToValue turns a string constant into a Value that is either a single Char or a vector of Chars.
func stringToValue(c Context, str string) Value {
	r := ([]rune)(str)
	if len(r) == 1 {
		return Char(r[0])
	}
	v := make([]Value, len(r))
	for i, c := range r {
		v[i] = Char(c)
	}
	return NewVector(v...)
}
