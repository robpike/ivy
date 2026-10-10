// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"
	"slices"

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
	history    []string     // Saved scanner history.
	tokens     []scan.Token // Tokens that built this Statement. Locked at creation time.
	c          Context      // Set only during parse.
	pos        int          // Token index in pTokens.
	last       scan.Token   // Last scanned token, even from peek.
	opStart    int          // Offset of start of operator. It's a seven-letter word.
	inOperator bool         // Part of a function body, for :ret.
	fileName   string       // Name of input stream.
	parsed     Expr         // Saved parse, flushed when global state changes.
}

var _ Expr = (*Statement)(nil)

// NewStatement creates a statement (expression) defined by the tokens.
// inOperator records whether the statement is part of a user-defined op,
// which when true admits :ret expressions.
func NewStatement(tokens []scan.Token, history []string, fileName string, inOperator bool) *Statement {
	return &Statement{
		history:    history,
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
		switch t.Type {
		case scan.Ret:
			hasRet = true
		case scan.Identifier:
			varNames[t.Text] = true
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
//	operator
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
		case scan.Operator, scan.Identifier, scan.RightBrace:
			unary, binary := s.parseOperator(c, s.prev())
			if unary == nil && binary == nil {
				s.Errorf("can't happen: no op")
			}
			if s.atOperand(c) && binary != nil {
				expr = &BinaryExpr{
					file:   s.fileName,
					line:   tok.Line,
					offset: tok.Offset,
					Left:   s.operand(c),
					Op:     binary,
					Right:  expr,
				}
			} else if unary != nil {
				expr = &UnaryExpr{
					file:   s.fileName,
					line:   tok.Line,
					offset: tok.Offset,
					Op:     unary,
					Right:  expr,
				}
			} else {
				s.Errorf("missing left operand for %q", binary)
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
	slices.Reverse(slice)
	return VectorExpr(slice)
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
			if isUnaryOp(c, tok) || isBinaryOp(c, tok) {
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
	slices.Reverse(slice)
	return VectorExpr(slice)
}

// element
//
//	number
//	identifier
//	string
//	operatorExpression
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
		if isOpToken(c, tok) {
			return NewOpExpr(s.parseOperator(c, tok))
		}
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
	case scan.Operator:
		return NewOpExpr(s.parseOperator(c, tok))
	case scan.RightBrace:
		return NewOpExpr(s.parseNonceOp(c, tok))
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

// parseUndecoratedOperator parses an operator with no decorations: no @s, not a reduce or scan, not a product.
func (s *Statement) parseUndecoratedOperator(c Context, tok scan.Token, inOpExpr bool) (unary Unary, binary Binary) {
	if tok.Type == scan.RightBrace {
		return s.parseNonceOp(c, tok)
	}
	v := variable(c, tok)
	if v != nil {
		if opExpr, ok := v.Value().(*OpExpr); ok {
			return opExpr.Unpack()
		}
		s.Errorf("%s is not an operator", tok.Text)
	}
	if c.UserDefined(tok.Text, true) {
		binary = c.NewUserDefinedBinaryOp(tok.Text)
	}
	if c.UserDefined(tok.Text, false) {
		unary = c.NewUserDefinedUnaryOp(tok.Text)
	}
	if binary == nil && BinaryBuiltins[tok.Text] != nil {
		binary = BinaryBuiltinOp(tok.Text)
	}
	if unary == nil && UnaryBuiltins[tok.Text] != nil {
		unary = UnaryBuiltinOp(tok.Text)
	}
	if unary == nil && binary == nil {
		s.Errorf("%q is not an operator", tok.Text)
	}
	s.opStart = tok.Offset
	return unary, binary
}

// parseReduceOrScan parses a reduce (op/) or scan (op\). The axisIndicator is either empty
// (the default) or "%" if the reduce/scan operates on the first axis.
func (s *Statement) parseReduceOrScan(c Context, tok scan.Token, axisIndicator string, inOpExpr bool) (unary Unary, binary Binary) {
	slash := tok
	text := slash.Text + axisIndicator
	if s.adjacent(tok) { // Reduction syntax requires operator abut the slash.
		tok = s.peek()
		switch tok.Type {
		case scan.EOF, scan.LeftParen:
			// Likely a unary inverse.
			break
		case scan.Identifier:
			if variable(c, tok) != nil {
				break
			}
			if c.UserDefined(tok.Text, true) {
				op := c.NewUserDefinedBinaryOp(s.prev().Text)
				s.opStart = tok.Offset
				return NewReduceOp(op, text), nil
			}
			fallthrough
		case scan.Operator:
			// A reduction if this is a binary operator.
			if BinaryBuiltins[tok.Text] != nil {
				op := BinaryBuiltinOp(s.prev().Text)
				s.opStart = tok.Offset
				return NewReduceOp(op, text), nil
			}
		}
	}
	// Not a reduction, just a simple operator whose existence will be checked during eval.
	s.opStart = slash.Offset
	return UnaryBuiltinOp(text), BinaryBuiltinOp(text)
}

// parseProduct parses an inner or outer product.
func (s *Statement) parseProduct(c Context, marker string, tok scan.Token) (binary Binary) {
	_, right := s.parseOperator(c, tok)
	if right == nil {
		s.Errorf("%s not a binary operator", right)
	}
	if marker == "o." {
		return NewOuterOp(right.(Binary))
	}
	tok = s.prev()
	_, left := s.parseOperator(c, tok)
	if left == nil {
		s.Errorf("%s not a binary operator", left)
	}
	return NewInnerOp(left, right)
}

// parseNonceOp parses an in-line operator definition. The argument token is the right brace.
// For simplicity, the entire definition must be on a single line.
//
// '{' op args = statementList '}'
// '{' args op args = statementList '}'
func (s *Statement) parseNonceOp(c Context, tok scan.Token) (unary Unary, binary Binary) {
	// Scan leftward to find opening brace.
	foundOp := false
	startPos := -1
	endPos := s.pos + 1 // Right of closing brace.
	endOffset := tok.Offset
	for nesting, i := 1, s.pos-1; i >= 0; i-- {
		switch s.tokens[i].Type {
		case scan.EOF, scan.Newline:
			s.Errorf("missing left brace")
		case scan.Op:
			if nesting == 1 {
				foundOp = true
			}
		case scan.RightBrace:
			nesting++
		case scan.LeftBrace:
			nesting--
			if nesting == 0 {
				startPos = i
				break
			}
		}
	}
	if startPos < 0 {
		s.Errorf("missing left brace in nonce operator declaration")
	}
	if !foundOp {
		s.Errorf("no %q keyword in nonce operator declaration", "op")
	}
	tokens := s.tokens[startPos:endPos]
	// Slice should be { [arg] op arg = body }
	if len(tokens) < 6 { // So we can avoid index checks later.
		s.Errorf("incomplete nonce operator declaration")
	}
	// Scan forward for "op" and "="
	opPos := 0
	for i, t := range tokens {
		if t.Type == scan.Op {
			opPos = i
			break
		}
	}
	eqPos := 0
	for i, t := range tokens {
		if t.Type == scan.Assign {
			eqPos = i
			break
		}
	}
	if opPos == 0 {
		s.Errorf("syntax error in nonce operator declaration; missing %q", "op")
	}
	if eqPos == 0 {
		s.Errorf("syntax error in nonce operator declaration; missing %q", "=")
	}
	var left Expr
	isBinary := false
	if opPos != 1 {
		isBinary = true
		left = s.parseNonceArgs(c, tokens[1:opPos])
	}
	right := s.parseNonceArgs(c, tokens[opPos+1:eqPos])
	s.opStart = tokens[0].Offset
	s.pos = startPos
	source := s.history[len(s.history)-1][s.opStart-1 : endOffset]
	body := s.nonceStatementList(c, tokens[eqPos+1:len(tokens)-1])
	if isBinary {
		return nil, NewBinaryNonceOp(c, left, right, body, source)
	}
	return NewUnaryNonceOp(c, right, body, source), nil
}

// parseNonceArgs returns an expression for the possibly parenthesized
// argument list.
//
//	name
//	'(' args ')'
func (s *Statement) parseNonceArgs(c Context, tokens []scan.Token) Expr {
	if len(tokens) == 0 {
		s.Errorf("empty arg list in nonce operator declaration")
	}
	var v VectorExpr
	for len(tokens) > 0 {
		if tokens[0].Type == scan.Identifier {
			v = append(v, NewVarExpr(tokens[0].Text))
			tokens = tokens[1:]
			continue
		}
		if tokens[0].Type != scan.LeftParen {
			s.Errorf("unexpected %q in nonce operator argument", tokens[0].Text)
		}
		tokens = tokens[1:]
		// Find right paren.
		nesting := 1
		for i, t := range tokens {
			if t.Type == scan.LeftParen {
				nesting++
			}
			if t.Type == scan.RightParen {
				nesting--
			}
			if nesting == 0 {
				v = append(v, s.parseNonceArgs(c, tokens[:i]))
				tokens = tokens[i+1:]
				break
			}
		}
		if nesting != 0 {
			s.Errorf("missing right paren in nonce operator argument at %q", tokens[0].Text)
		}
	}
	if len(v) == 1 {
		return v[0]
	}
	return v
}

// nonceStatementList returns the statement list inside a nonce op definition.
// The token stream is known to be on a single line.
func (s *Statement) nonceStatementList(c Context, tokens []scan.Token) StatementList {
	list := StatementList{}
	end := len(tokens)
	ctrlLevel := 0
	for i := end; i > 0; i-- {
		tok := tokens[i-1]
		switch tok.Type {
		case scan.If, scan.While:
			ctrlLevel++
		case scan.End:
			ctrlLevel--
		case scan.Semicolon:
			if ctrlLevel == 0 && end > i {
				list = append(list, NewStatement(tokens[i:end], s.history, s.fileName, true))
				i-- // Omit the semicolon.
				end = i
			}
		}
	}
	if end > 0 {
		list = append(list, NewStatement(tokens[:end], s.history, s.fileName, true))
	}
	slices.Reverse(list)
	return list
}

// parseOperator parses the general form of an operator and returns
// the unary and binary form, depending on which exist.
func (s *Statement) parseOperator(c Context, tok scan.Token) (unary Unary, binary Binary) {
	numTrailingAts := 0
	for tok.Text == "@" {
		numTrailingAts++
		tok = s.prev()
	}
	axisIndicator := ""
	if tok.Text == "%" { // Might be a scan.
		peek := s.peek()
		if s.adjacent(tok) && (peek.Text == "/" || peek.Text == "\\") { // % must abut to form a reduce.
			tok = s.prev()
			axisIndicator = "%"
		}
	}
	if s.adjacent(tok) { // Could be a compound operator.
		marker := s.peek().Text
		if axisIndicator == "" && (marker == "." || marker == "o.") {
			binary = s.parseProduct(c, s.prev().Text, tok)
		} else if tok.Text == "/" || tok.Text == "\\" {
			unary, binary = s.parseReduceOrScan(c, tok, axisIndicator, false)
		}
	}
	if unary == nil && binary == nil {
		unary, binary = s.parseUndecoratedOperator(c, tok, false)
	}
	if unary == nil && binary == nil {
		s.Errorf("can't happen: no operator")
	}
	numLeadingAts := 0
	// Special check to get started because the @ must be adjacent to the start of the operator.
	// (We don't have the actual operator token any more.)
	if s.peek().Offset == s.opStart-1 {
		for s.peek().Text == "@" {
			numLeadingAts++
			s.prev()
		}
	}
	if numLeadingAts > 0 && binary == nil {
		s.Errorf("leading @ on unary operator %q", unary)
	}
	// Wrap the op as an Each if @s are present.
	if numLeadingAts+numTrailingAts > 0 {
		if binary != nil {
			binary = NewBinaryEachOp(binary, numLeadingAts, numTrailingAts)
			if numLeadingAts > 0 {
				unary = nil // Can't put @ on left of unary; return binary only.
			}
		}
		if unary != nil {
			if numLeadingAts > 0 {
				s.Errorf("illegal %q on unary %s", "@", unary)
			}
			if numTrailingAts > 0 {
				unary = NewUnaryEachOp(unary, numTrailingAts)
			}
		}
	}
	return unary, binary
}

// parseOperator parses the general form of an operator and reports
// whether it is a binary.
func (s *Statement) XXparseOperator(c Context, canParseBinary bool) (op Operator, isBinary bool) {
	panic("NO")
	/*
		numTrailingAts := s.numAts()
		tok := s.prev()
		axisIndicator := ""
		if tok.Text == "%" { // Might be a scan.
			peek := s.peek()
			if s.adjacent(tok) && (peek.Text == "/" || peek.Text == "\\") { // % must abut to form a reduce.
				tok = s.prev()
				axisIndicator = "%"
			}
		}
		if s.adjacent(tok) { // Could be a compound operator.
			marker := s.peek().Text
			if axisIndicator == "" && (marker == "." || marker == "o.") {
				op, isBinary = s.parseProduct(c, s.prev().Text, tok)
			} else if tok.Text == "/" || tok.Text == "\\" {
				op, isBinary = s.parseReduceOrScan(c, tok, axisIndicator, canParseBinary)
			}
		}
		if op == nil {
			op, isBinary = s.parseUndecoratedOperator(c, tok, canParseBinary)
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
	*/
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

func variable(c Context, tok scan.Token) *Var {
	if tok.Type != scan.Identifier {
		return nil
	}
	if c.IsLocal(tok.Text) {
		return c.Local(tok.Text)
	}
	return c.Global(tok.Text)
}

func isBinaryOp(c Context, tok scan.Token) bool {
	if v := variable(c, tok); v != nil {
		if op, ok := v.Value().(*OpExpr); ok {
			return op.HasBinary()
		}
		return false
	}
	return BinaryBuiltins[tok.Text] != nil || c.UserDefined(tok.Text, true)
}

func isUnaryOp(c Context, tok scan.Token) bool {
	if v := variable(c, tok); v != nil {
		if op, ok := v.Value().(*OpExpr); ok {
			return op.HasUnary()
		}
		return false
	}
	return UnaryBuiltins[tok.Text] != nil || c.UserDefined(tok.Text, false)
}

// Is the token the name of an operator?
func isOpToken(c Context, tok scan.Token) bool {
	if v := variable(c, tok); v != nil {
		_, ok := v.Value().(*OpExpr)
		return ok
	}
	if BinaryBuiltins[tok.Text] != nil || c.UserDefined(tok.Text, true) {
		return true
	}
	if UnaryBuiltins[tok.Text] != nil || c.UserDefined(tok.Text, false) {
		return true
	}
	return false
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
		if isBinaryOp(c, tok) || isUnaryOp(c, tok) {
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
