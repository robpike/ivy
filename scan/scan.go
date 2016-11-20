// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate stringer -type Type

package scan // import "robpike.io/ivy/scan"

import (
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"robpike.io/ivy/exec"
	"robpike.io/ivy/value"
)

// Token represents a token or text string returned from the scanner.
type Token struct {
	Type Type   // The type of this item.
	Line int    // The line number on which this token appears
	Text string // The text of this item.
}

// Type identifies the type of lex items.
type Type int

const (
	EOF   Type = iota // zero value so closed channel delivers EOF
	Error             // error occurred; value is text of error
	Newline
	// Interesting things
	Assign         // '='
	Char           // printable ASCII character; grab bag for comma etc.
	GreaterOrEqual // '>='
	Identifier     // alphanumeric identifier
	LeftBrack      // '['
	LeftParen      // '('
	Number         // simple number
	Operator       // known operator
	Op             // "op", operator definition keyword
	Rational       // rational number like 2/3
	RightBrack     // ']'
	RightParen     // ')'
	Semicolon      // ';'
	Space          // run of spaces separating
	String         // quoted string (includes quotes)
)

func (i Token) String() string {
	switch {
	case i.Type == EOF:
		return "EOF"
	case i.Type == Error:
		return "error: " + i.Text
	case len(i.Text) > 10:
		return fmt.Sprintf("%s: %.10q...", i.Type, i.Text)
	}
	return fmt.Sprintf("%s: %q", i.Type, i.Text)
}

const eof = -1

// stateFn represents the state of the scanner as a function that returns the next state.
type stateFn func(*Scanner) stateFn

// Scanner holds the state of the scanner.
type Scanner struct {
	tokens     chan Token // channel of scanned items
	context    value.Context
	r          io.ByteReader
	done       bool
	name       string // the name of the input; used only for error reports
	buf        []byte
	input      string  // the line of text being scanned.
	leftDelim  string  // start of action
	rightDelim string  // end of action
	state      stateFn // the next lexing function to enter
	line       int     // line number in input
	pos        int     // current position in the input
	start      int     // start position of this item
	width      int     // width of last rune read from input
}

// loadLine reads the next line of input and stores it in (appends it to) the input.
// (l.input may have data left over when we are called.)
// It strips carriage returns to make subsequent processing simpler.
func (l *Scanner) loadLine() {
	l.buf = l.buf[:0]
	for {
		c, err := l.r.ReadByte()
		if err != nil {
			l.done = true
			break
		}
		if c != '\r' {
			l.buf = append(l.buf, c)
		}
		if c == '\n' {
			break
		}
	}
	l.input = l.input[l.start:l.pos] + string(l.buf)
	l.pos -= l.start
	l.start = 0
}

// next returns the next rune in the input.
func (l *Scanner) next() rune {
	if !l.done && int(l.pos) == len(l.input) {
		l.loadLine()
	}
	if len(l.input) == int(l.pos) {
		l.width = 0
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.width = w
	l.pos += l.width
	return r
}

// peek returns but does not consume the next rune in the input.
func (l *Scanner) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one rune. Can only be called once per call of next.
func (l *Scanner) backup() {
	l.pos -= l.width
}

//  passes an item back to the client.
func (l *Scanner) emit(t Type) {
	if t == Newline {
		l.line++
	}
	s := l.input[l.start:l.pos]
	config := l.context.Config()
	if config.Debug("tokens") {
		fmt.Fprintf(config.Output(), "%s:%d: emit %s\n", l.name, l.line, Token{t, l.line, s})
	}
	l.tokens <- Token{t, l.line, s}
	l.start = l.pos
	l.width = 0
}

// ignore skips over the pending input before this point.
func (l *Scanner) ignore() {
	l.start = l.pos
}

// accept consumes the next rune if it's from the valid set.
func (l *Scanner) accept(valid string) bool {
	if strings.IndexRune(valid, l.next()) >= 0 {
		return true
	}
	l.backup()
	return false
}

// acceptRun consumes a run of runes from the valid set.
func (l *Scanner) acceptRun(valid string) {
	for strings.IndexRune(valid, l.next()) >= 0 {
	}
	l.backup()
}

// errorf returns an error token and continues to scan.
func (l *Scanner) errorf(format string, args ...interface{}) stateFn {
	l.tokens <- Token{Error, l.start, fmt.Sprintf(format, args...)}
	return lexAny
}

// New creates a new scanner for the input string.
func New(context value.Context, name string, r io.ByteReader) *Scanner {
	l := &Scanner{
		r:       r,
		name:    name,
		line:    1,
		tokens:  make(chan Token, 2), // We need a little room to save tokens.
		context: context,
		state:   lexAny,
	}
	return l
}

// Next returns the next token.
func (l *Scanner) Next() Token {
	// The lexer is concurrent but we don't want it to run in parallel
	// with the rest of the interpreter, so we only run the state machine
	// when we need a token.
	for l.state != nil {
		select {
		case tok := <-l.tokens:
			return tok
		default:
			// Run the machine
			l.state = l.state(l)
		}
	}
	if l.tokens != nil {
		close(l.tokens)
		l.tokens = nil
	}
	return Token{EOF, l.pos, "EOF"}
}

// state functions

// lexComment scans a comment. The comment marker has been consumed.
func lexComment(l *Scanner) stateFn {
	for {
		r := l.next()
		if r == eof || r == '\n' {
			break
		}
	}
	if len(l.input) > 0 {
		l.pos = len(l.input)
		l.start = l.pos - 1
		// Emitting newline also advances l.line.
		l.emit(Newline) // TODO: pass comments up?
	}
	return lexSpace
}

// lexAny scans non-space items.
func lexAny(l *Scanner) stateFn {
	switch r := l.next(); {
	case r == eof:
		return nil
	case r == '\n': // TODO: \r
		l.emit(Newline)
		return lexAny
	case r == ';':
		l.emit(Semicolon)
		return lexAny
	case r == '#':
		return lexComment
	case isSpace(r):
		return lexSpace
	case r == '"':
		return lexQuote
	case r == '`':
		return lexRawQuote
	case r == '\'':
		return lexChar
	case r == '-':
		// It's an operator if it's preceded immediately (no spaces) by an operand, which is
		// an identifier, an indexed expression, or a parenthesized expression.
		// Otherwise it could be a signed number.
		if l.start > 0 {
			rr, _ := utf8.DecodeLastRuneInString(l.input[:l.start])
			if isAlphaNumeric(rr) || rr == ')' || rr == ']' {
				l.emit(Operator)
				return lexAny
			}
		}
		fallthrough
	case r == '.' || '0' <= r && r <= '9':
		l.backup()
		return lexNumber
	case r == '=':
		if l.peek() != '=' {
			l.emit(Assign)
			return lexAny
		}
		l.next()
		fallthrough // for ==
	case l.isOperator(r):
		// Must be after after = so == is an operator,
		// and after numbers, so '-' can be a sign.
		return lexOperator
	case isAlphaNumeric(r):
		l.backup()
		return lexIdentifier
	case r == '[':
		l.emit(LeftBrack)
		return lexAny
	case r == ']':
		l.emit(RightBrack)
		return lexAny
	case r == '(':
		l.emit(LeftParen)
		return lexAny
	case r == ')':
		l.emit(RightParen)
		return lexAny
	case r <= unicode.MaxASCII && unicode.IsPrint(r):
		l.emit(Char)
		return lexAny
	default:
		return l.errorf("unrecognized character: %#U", r)
	}
}

// lexSpace scans a run of space characters.
// One space has already been seen.
func lexSpace(l *Scanner) stateFn {
	for isSpace(l.peek()) {
		l.next()
	}
	l.ignore()
	return lexAny
}

// lexIdentifier scans an alphanumeric.
// If the input base is greater than 10, some identifiers
// are actually numbers. We handle this here.
func lexIdentifier(l *Scanner) stateFn {
Loop:
	for {
		switch r := l.next(); {
		case isAlphaNumeric(r):
			// absorb.
		default:
			l.backup()
			if !l.atTerminator() {
				return l.errorf("bad character %#U", r)
			}
			// Some identifiers are operators.
			word := l.input[l.start:l.pos]
			switch {
			case word == "o" && l.peek() == '.':
				return lexOperator
			case exec.Predefined(word) || l.context.UserDefined(word, true):
				return lexOperator
			case word == "op":
				l.emit(Op)
			case isAllDigits(word, l.context.Config().InputBase()):
				l.emit(Number)
			default:
				l.emit(Identifier)
			}
			break Loop
		}
	}
	return lexAny
}

// lexOperator completes scanning an operator. We have already accepted the + or
// whatever; there may be a reduction or inner or outer product.
func lexOperator(l *Scanner) stateFn {
	// It might be an inner product or reduction, but only if it is a binary operator.
	word := l.input[l.start:l.pos]
	if word == "o" || value.BinaryOps[word] != nil || l.context.UserDefined(word, true) {
		switch l.peek() {
		case '/':
			// Reduction.
			l.next()
		case '\\':
			// Scan.
			l.next()
		case '.':
			// Inner or outer product?
			l.next()               // Accept the '.'.
			if isDigit(l.peek()) { // Is a number after all, as in 3*.7. Back up.
				l.backup()
				l.emit(Operator) // Up to but not including the period.
				return lexNumber // We know it starts ".7".
			}
			startRight := l.pos
			r := l.next()
			switch {
			case l.isOperator(r):
			case isAlphaNumeric(r):
				for isAlphaNumeric(r) {
					r = l.next()
				}
				l.backup()
				if !l.atTerminator() {
					return l.errorf("bad character %#U", r)
				}
				word := l.input[startRight:l.pos]
				if !exec.Predefined(word) && !l.context.UserDefined(word, true) {
					return l.errorf("%s not an operator", word)
				}
			}
		}
	}
	if isIdentifier(l.input[l.start:l.pos]) {
		l.emit(Identifier)
	} else {
		l.emit(Operator)
	}
	return lexSpace
}

// atTerminator reports whether the input is at valid termination character to
// appear after an identifier.
func (l *Scanner) atTerminator() bool {
	r := l.peek()
	if r == eof || isSpace(r) || isEndOfLine(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
		return true
	}
	// Does r start the delimiter? This can be ambiguous (with delim=="//", $x/2 will
	// succeed but should fail) but only in extremely rare cases caused by willfully
	// bad choice of delimiter.
	if rd, _ := utf8.DecodeRuneInString(l.rightDelim); rd == r {
		return true
	}
	return false
}

// lexChar scans a character constant. The initial quote is already
// scanned. Syntax checking is done by the parser.
func lexChar(l *Scanner) stateFn {
Loop:
	for {
		switch l.next() {
		case '\\':
			if r := l.next(); r != eof && r != '\n' {
				break
			}
			fallthrough
		case eof, '\n':
			return l.errorf("unterminated character constant")
		case '\'':
			break Loop
		}
	}
	l.emit(String)
	return lexAny
}

// lexNumber scans a number: decimal, octal, hex, float, or imaginary. This
// isn't a perfect number scanner - for instance it accepts "." and "0x0.2"
// and "089" - but when it's wrong the input is invalid and the parser (via
// strconv) will notice.
func lexNumber(l *Scanner) stateFn {
	// Optional leading sign.
	if l.accept("-") {
		// Might not be a number.
		r := l.peek()
		// Might be a scan or reduction.
		if r == '/' || r == '\\' {
			l.next()
			l.emit(Operator)
			return lexAny
		}
		if r != '.' && !isNumeral(r, l.context.Config().InputBase()) {
			l.emit(Operator)
			return lexAny
		}
	}
	if !l.scanNumber() {
		return l.errorf("bad number syntax: %s", l.input[l.start:l.pos])
	}
	if l.peek() != '/' {
		l.emit(Number)
		return lexAny
	}
	// Might be a rational.
	l.accept("/")

	if r := l.peek(); r != '.' && !isNumeral(r, l.context.Config().InputBase()) {
		// Oops, not a number. Hack!
		l.pos-- // back up before '/'
		l.emit(Number)
		l.accept("/")
		l.emit(Operator)
		return lexAny
	}
	if !l.scanNumber() {
		return l.errorf("bad number syntax: %s", l.input[l.start:l.pos])
	}
	l.emit(Rational)
	return lexAny
}

func (l *Scanner) scanNumber() bool {
	base := l.context.Config().InputBase()
	digits := digitsForBase(base)
	// If base 0, acccept octal for 0 or hex for 0x or 0X.
	if base == 0 {
		if l.accept("0") && l.accept("xX") {
			digits = digitsForBase(16)
		}
		// Otherwise leave it decimal (0); strconv.ParseInt will take care of it.
		// We can't set it to 8 in case it's a leading-0 float like 0.69 or 09e4.
	}
	l.acceptRun(digits)
	if l.accept(".") {
		l.acceptRun(digits)
	}
	if l.accept("eE") {
		l.accept("+-")
		l.acceptRun("0123456789")
	}
	// Next thing mustn't be alphanumeric except possibly an o for outer product (3o.+2).
	if l.peek() != 'o' && isAlphaNumeric(l.peek()) {
		l.next()
		return false
	}
	return true
}

var digits [36 + 1]string // base 36 is OK.

const (
	decimal = "0123456789"
	lower   = "abcdefghijklmnopqrstuvwxyz"
	upper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

// digitsForBase returns the digit set for numbers in the specified base.
func digitsForBase(base int) string {
	if base == 0 {
		base = 10
	}
	d := digits[base]
	if d == "" {
		if base <= 10 {
			// Always accept a maximal string of numerals.
			// Whatever the input base, if it's <= 10 let the parser
			// decide if it's valid. This also helps us get the always-
			// base-10 numbers for )specials.
			d = decimal[:10]
		} else {
			d = decimal + lower[:base-10] + upper[:base-10]
		}
		digits[base] = d
	}
	return d
}

// lexQuote scans a quoted string.
func lexQuote(l *Scanner) stateFn {
Loop:
	for {
		switch l.next() {
		case '\\':
			if r := l.next(); r != eof && r != '\n' {
				break
			}
			fallthrough
		case eof, '\n':
			return l.errorf("unterminated quoted string")
		case '"':
			break Loop
		}
	}
	l.emit(String)
	return lexAny
}

// lexRawQuote scans a raw quoted string.
func lexRawQuote(l *Scanner) stateFn {
Loop:
	for {
		switch l.next() {
		case eof:
			return l.errorf("unterminated raw quoted string")
		case '`':
			break Loop
		}
	}
	l.emit(String)
	return lexAny
}

// isSpace reports whether r is a space character.
func isSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

// isEndOfLine reports whether r is an end-of-line character.
func isEndOfLine(r rune) bool {
	return r == '\r' || r == '\n'
}

// isIdentifier reports whether the string is a valid identifier.
func isIdentifier(s string) bool {
	if s == "_" {
		return false // Special symbol; can't redefine.
	}
	first := true
	for _, r := range s {
		if unicode.IsDigit(r) {
			if first {
				return false
			}
		} else if r != '_' && !unicode.IsLetter(r) {
			return false
		}
		first = false
	}
	return true
}

// isAlphaNumeric reports whether r is an alphabetic, digit, or underscore.
func isAlphaNumeric(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// isDigit reports whether r is an ASCII digit.
func isDigit(r rune) bool {
	return '0' <= r && r <= '9'
}

// isNumeral reports whether r is a numeral in the specified base.
// A decimal digit is always taken as a numeral, because otherwise parsing
// would be muddled. (In base 8, 039 shouldn't be scanned as two numbers.)
// The parser will check that the scanned number is legal.
func isNumeral(r rune, base int) bool {
	if '0' <= r && r <= '9' {
		return true
	}
	if base < 10 {
		return false
	}
	top := rune(base - 10)
	if 'a' <= r && r <= 'a'+top {
		return true
	}
	if 'A' <= r && r <= 'A'+top {
		return true
	}
	return false
}

// isAllDigits reports whether s consists of digits in the specified base.
func isAllDigits(s string, base int) bool {
	top := 'a' + rune(base-10) - 1
	TOP := 'A' + rune(base-10) - 1
	for _, c := range s {
		if '0' <= c && c <= '9' {
			continue
		}
		if 'a' <= c && c <= top {
			continue
		}
		if 'A' <= c && c <= TOP {
			continue
		}
		return false
	}
	return true
}

// isOperator reports whether r is an operator. It may advance the lexer one character
// if it is a two-character operator.
func (l *Scanner) isOperator(r rune) bool {
	switch r {
	case '?', '+', '-', '/', '%', '&', '|', '^', ',':
		// No follow-on possible.
	case '!':
		if l.peek() != '=' {
			return false
		}
		l.next()
	case '>':
		switch l.peek() {
		case '>', '=':
			l.next()
		}
	case '<':
		switch l.peek() {
		case '<', '=':
			l.next()
		}
	case '*':
		switch l.peek() {
		case '*':
			l.next()
		}
	case '=':
		if l.peek() != '=' {
			return false
		}
		l.next()
	default:
		return false
	}
	return true
}
