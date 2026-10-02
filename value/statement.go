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
type Statement struct {
	tokens     []scan.Token // Tokens that built this Statement. Locked at creation time.
	c          Context      // Set only during parse.
	pos        int          // Token index in pTokens.
	last       scan.Token   // Last scanned token, even from peek.
	opStart    int          // offset of start of operator.
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
// whether the statement has a :ret.
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
		switch s.peek().Type {
		case scan.LeftParen:
			s.Errorf("unclosed parenthesis")
		case scan.LeftBrack:
			s.Errorf("unclosed bracket")
		}
		s.Errorf("extra %q at beginning of statement", s.peek().Text)
	}
	s.parsed = expr
	return expr
}

func (s *Statement) eofTok() scan.Token {
	return scan.Token{
		Type:   scan.EOF,
		Line:   s.last.Line,
		Offset: s.last.Offset,
		Text:   "newline",
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

// statement
//
//	expr
//	expr ":" expr
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
func (s *Statement) expr(c Context) Expr {
	expr := s.operand(c)
	for {
		tok := s.peek()
		switch tok.Type {
		case scan.EOF, scan.LeftParen, scan.LeftBrack, scan.Colon, scan.Semicolon, scan.If, scan.Elif, scan.Else, scan.While:
			return expr
		case scan.Assign:
			s.prev() // Eat the =
			vars := s.assignable(c)
			s.checkAssign(c, vars)
			expr = &BinaryExpr{
				file:   s.fileName,
				line:   tok.Line,
				offset: tok.Offset,
				Left:   vars,
				Op:     BinaryBuiltinOp(tok.Text),
				Right:  expr,
			}
		case scan.Operator, scan.Identifier:
			op, isBinary := s.parseOperator(c)
			if isBinary {
				expr = &BinaryExpr{
					file:   s.fileName,
					line:   tok.Line,
					offset: tok.Offset,
					Left:   s.operand(c),
					Op:     op.(Binary),
					Right:  expr,
				}
			} else {
				expr = &UnaryExpr{
					file:   s.fileName,
					line:   tok.Line,
					offset: tok.Offset,
					Op:     op.(Unary),
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
			s.Errorf("unexpected token %q", tok.Text) // Catch-all, shouldn't happen.
		}
	}
}

// assignable returns a list of identifiers and index expressions to be used as the
// lhs of an assignment. We could (and once did) just call s.operand but this routine
// restricts the results due to a corner case that affects the parse. If the lhs
// contains operators to be used as variables and there are multiple items of any
// kind, the list must be parenthesized to signal that any operators remain simple
// identifiers. Otherwise any operator works as normally. An imperfect
// solution to a rare problem.
// APL does not have this issue because its bulitins are not identifiers.
//
//	cos = 4 # Simple assignment, works as before.
//	sin x = .2 .3  # Parses as sin (x=.2 .3)
//		0.198669330795 0.295520206661
//	(sin x) = .2 .3; sin x # sin is now a variable.
//		1/5 3/10
//
// assignable
//
//	assignableElem [assignableElem ...]
//	"(" assignableElem ... ")"
func (s *Statement) assignable(c Context) Expr {
	parens := false
	if s.peek().Type == scan.RightParen {
		s.prev()
		parens = true
	}
	expr, ok := s.assignableElem(c, false)
	if !ok {
		s.Errorf("unexpected token on lhs of assignment: %q", s.peek().Text)
	}
	slice := []Expr{expr}
	for {
		expr, ok = s.assignableElem(c, !parens)
		if !ok {
			break
		}
		slice = append(slice, expr)
	}
	if parens {
		if s.prev().Type != scan.LeftParen {
			s.Errorf("missing left paren on lhs of assignment")
		}
	}
	if len(slice) == 1 {
		return slice[0]
	}
	return VectorExpr(reverse(slice))
}

// assignableElem
//
//	identifier
//	index expression
//	assignable    - if parenthesized
func (s *Statement) assignableElem(c Context, stopAtOperator bool) (Expr, bool) {
	tok := s.peek()
	switch tok.Type {
	case scan.Identifier:
		if stopAtOperator {
			if isUnaryOp(c, tok.Text) || isBinaryOp(c, tok.Text) {
				return nil, false
			}
		}
		s.prev()
		v := &VarExpr{
			file:   s.fileName,
			line:   tok.Line,
			offset: tok.Offset,
			Name:   tok.Text,
		}
		return v, true
	case scan.RightBrack:
		return s.element(c), true
	case scan.RightParen: // The same rules apply if nested.
		return s.assignable(c), true
	}
	return nil, false
}

// operand
//
//	element
//	element [element...]
func (s *Statement) operand(c Context) Expr {
	expr := s.element(c)
	if !s.atOperand(c) {
		return expr
	}
	slice := []Expr{expr}
	for s.atOperand(c) {
		slice = append(slice, s.element(c))
	}
	return VectorExpr(reverse(slice))
}

// reverse reverses the slice. With a lot more code (or an n² in the generating loops)
// we could avoid this but the reversal is cheap.
func reverse(slice []Expr) []Expr {
	for i, j := 0, len(slice)-1; i < j; i, j = i+1, j-1 {
		slice[i], slice[j] = slice[j], slice[i]
	}
	return slice
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
func (s *Statement) element(c Context) Expr {
	tok := s.prev()
	switch tok.Type {
	default:
		s.Errorf("syntax error at %q", tok.Text)
	case scan.Number:
		expr, err := ParseNumber(c, tok.Text)
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
			s.Errorf("missing left bracket")
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

// parseSimpleOperator parses an operator with no decorations: no @s, not a reduce or scan, not a product.
func (s *Statement) parseSimpleOperator(c Context, tok scan.Token) (op Operator, isBinary bool) {
	lhsAt := s.peek().Text == "@" && s.adjacent(tok)
	canParseBinary := (lhsAt || s.atOperand(c))
	switch {
	case tok.Type == scan.Identifier && isVariable(c, tok.Text): // TODO: Can this happen? (Residue of old code.)
		c.Errorf("can't do var %s yet")
	case c.UserDefined(tok.Text, true) && canParseBinary:
		s.opStart = tok.Offset
		return c.NewUserDefinedBinaryOp(tok.Text), true
	case c.UserDefined(tok.Text, false):
		s.opStart = tok.Offset
		return c.NewUserDefinedUnaryOp(tok.Text), false
	case BinaryBuiltins[tok.Text] != nil && canParseBinary:
		s.opStart = tok.Offset
		return BinaryBuiltinOp(tok.Text), true
	case UnaryBuiltins[tok.Text] != nil:
		s.opStart = tok.Offset
		return UnaryBuiltinOp(tok.Text), false
	case isBinaryOp(c, tok.Text):
		// We have a binary but the syntax needs a unary.
		s.Errorf("%q is not a unary operator", tok.Text)
	}
	s.Errorf("%q is not an operator", tok.Text)
	panic("not reached")
}

// parseBinaryOperator is like parseSimpleOperator but demands the operator be binary.
func (s *Statement) parseBinaryOperator(c Context, tok scan.Token) Operator {
	switch {
	case tok.Type == scan.Identifier && isVariable(c, tok.Text): // TODO: Can this happen? (Residue of old code.)
		c.Errorf("parseBinaryOperator can't do var %s yet")
	case c.UserDefined(tok.Text, true):
		s.opStart = tok.Offset
		return c.NewUserDefinedBinaryOp(tok.Text)
	case BinaryBuiltins[tok.Text] != nil:
		s.opStart = tok.Offset
		return BinaryBuiltinOp(tok.Text)
	}
	s.Errorf("%q is not a binary operator", tok.Text)
	panic("not reached")
}

// parseReduceOrScan parses a reduce (op/) or scan (op\). The axisIndicator is either empty
// (the default) or "%" if the reduce/scan operates on the first axis.
func (s *Statement) parseReduceOrScan(c Context, tok scan.Token, axisIndicator string) (op Operator, isBinary bool) {
	slash := tok
	text := slash.Text + axisIndicator
	if s.adjacent(tok) { // Reduction syntax requires operator abut the slash.
		tok = s.peek()
		switch tok.Type {
		case scan.EOF, scan.LeftParen:
			// Likely a unary inverse.
			break
		case scan.Identifier:
			if isVariable(c, tok.Text) {
				break
			}
			if c.UserDefined(tok.Text, true) {
				op := c.NewUserDefinedBinaryOp(s.prev().Text)
				s.opStart = tok.Offset
				return NewReduceOp(op, text), false
			}
			fallthrough
		case scan.Operator:
			// A reduction if this is a binary operator.
			if BinaryBuiltins[tok.Text] != nil {
				op := BinaryBuiltinOp(s.prev().Text)
				s.opStart = tok.Offset
				return NewReduceOp(op, text), false
			}
		}
	}
	// Not a reduction, just a simple operator whose existence will be checked during eval.
	if s.atOperand(c) {
		s.opStart = slash.Offset
		return BinaryBuiltinOp(text), true
	}
	s.opStart = slash.Offset
	return UnaryBuiltinOp(text), false

}

// parseProduct parses an inner or outer product.
func (s *Statement) parseProduct(c Context, marker string, tok scan.Token) (op Operator, isBinary bool) {
	right := s.parseBinaryOperator(c, tok)
	if marker == "o." {
		return NewOuterOp(right.(Binary)), true
	}
	tok = s.prev()
	left := s.parseBinaryOperator(c, tok)
	return NewInnerOp(left.(Binary), right.(Binary)), true
}

// numAts counts the number of abutting @ signs at the location.
func (s *Statement) numAts() int {
	if s.peek().Text != "@" {
		return 0
	}
	n := 1
	offset := s.prev().Offset
	for s.peek().Text == "@" && s.peek().Offset == offset-1 {
		offset = s.prev().Offset
		n++
	}
	return n
}

// parseOperator parses the general form of an operator and reports
// whether it is a binary.
func (s *Statement) parseOperator(c Context) (op Operator, isBinary bool) {
	numTrailingAts := s.numAts()
	tok := s.prev()
	axisIndicator := ""
	switch {
	case tok.Text == "%":
		peek := s.peek()
		if peek.Text != "/" && peek.Text != "\\" || !s.adjacent(tok) { // % must abut the / or \.
			break
		}
		tok = s.prev()
		axisIndicator = "%"
		fallthrough
	case tok.Text == "/", tok.Text == "\\":
		op, isBinary = s.parseReduceOrScan(c, tok, axisIndicator)
	default:
		// Is it a product?
		if s.adjacent(tok) {
			marker := s.peek().Text
			if marker == "." || marker == "o." {
				op, isBinary = s.parseProduct(c, s.prev().Text, tok)
			}
		}
	}
	if op == nil {
		op, isBinary = s.parseSimpleOperator(c, tok)
	}
	numLeadingAts := 0
	// Special check to get started because the @ must be adjacent to the start of the operator.
	// (We don't have the actual operator token any more.)
	for s.peek().Text == "@" && s.peek().Offset == s.opStart-1 {
		numLeadingAts = s.numAts()
	}
	// Wrap the op as an Each if @s are present.
	if numLeadingAts+numTrailingAts > 0 {
		if !isBinary {
			if numLeadingAts > 0 {
				c.Errorf("illegal %q on unary %s", "@", op)
			}
			if numTrailingAts != 0 {
				op = NewUnaryEachOp(op.(Unary), numTrailingAts)
			}
		} else {
			op = NewBinaryEachOp(op.(Binary), numLeadingAts, numTrailingAts)
		}
	}
	return op, isBinary
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
	if isVariable(c, id) {
		return false
	}
	return BinaryBuiltins[id] != nil || c.UserDefined(id, true)
}

func isUnaryOp(c Context, id string) bool {
	if isVariable(c, id) {
		return false
	}
	return UnaryBuiltins[id] != nil || c.UserDefined(id, false)
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
