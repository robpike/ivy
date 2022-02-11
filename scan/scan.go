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
	Assign     // '='
	Char       // printable ASCII character; grab bag for comma etc.
	Identifier // alphanumeric identifier
	LeftBrack  // '['
	LeftParen  // '('
	Number     // simple number
	Operator   // known operator
	Op         // "op", operator definition keyword
	Rational   // rational number like 2/3
	Complex    // complex number like 3j2
	RightBrack // ']'
	RightParen // ')'
	Semicolon  // ';'
	String     // quoted string (includes quotes)
	Colon      // ':'
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
	context   value.Context
	r         io.ByteReader
	done      bool
	name      string  // the name of the input; used only for error reports
	buf       []byte  // I/O buffer, re-used.
	input     string  // the line of text being scanned.
	lastRune  rune    // most recent return from next()
	lastWidth int     // size of that rune
	readOK    bool    // allow reading of a new line of input
	state     stateFn // the next lexing function to enter
	line      int     // line number in input
	pos       int     // current position in the input
	start     int     // start position of this item
	token     Token
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
		if c != '\r' { // There will never be a \r in l.input.
			l.buf = append(l.buf, c)
		}
		if c == '\n' {
			break
		}
	}
	// Reset to beginning of input buffer if there is nothing pending.
	if l.start == l.pos {
		l.input = string(l.buf)
		l.start = 0
		l.pos = 0
	} else {
		l.input += string(l.buf)
	}
}

// readRune reads the next rune from the input.
func (l *Scanner) readRune() (rune, int) {
	if !l.done && l.pos == len(l.input) {
		if !l.readOK { // Token did not end before newline.
			l.errorf("incomplete token")
			return '\n', 1
		}
		l.loadLine()
	}
	if len(l.input) == l.pos {
		return eof, 0
	}
	return utf8.DecodeRuneInString(l.input[l.pos:])
}

// next returns the next rune in the input.
func (l *Scanner) next() rune {
	l.lastRune, l.lastWidth = l.readRune()
	l.pos += l.lastWidth
	return l.lastRune
}

// peek returns but does not consume the next rune in the input.
func (l *Scanner) peek() rune {
	r, _ := l.readRune()
	return r
}

// peek2 returns the next two runes ahead, but does not consume anything.
func (l *Scanner) peek2() (rune, rune) {
	pos := l.pos
	r1 := l.next()
	r2 := l.next()
	l.pos = pos
	return r1, r2
}

// backup steps back one rune. Should only be called once per call of next.
func (l *Scanner) backup() {
	if l.lastRune == eof {
		return
	}
	if l.pos == l.start {
		l.errorf("internal error: backup at start of input")
	}
	if l.pos > l.start { // TODO can't happen?
		l.pos -= l.lastWidth
	}
}

// emit passes an item back to the client.
func (l *Scanner) emit(t Type) stateFn {
	if t == Newline {
		l.line++
	}
	text := l.input[l.start:l.pos]
	config := l.context.Config()
	if config.Debug("tokens") {
		fmt.Fprintf(config.Output(), "%s:%d: emit %s\n", l.name, l.line, Token{t, l.line, text})
	}
	l.token = Token{t, l.line, text}
	l.start = l.pos
	return nil
}

// accept consumes the next rune if it's from the valid set.
func (l *Scanner) accept(valid string) bool {
	if strings.ContainsRune(valid, l.next()) {
		return true
	}
	l.backup()
	return false
}

// acceptRun consumes a run of runes from the valid set.
func (l *Scanner) acceptRun(valid string) {
	for strings.ContainsRune(valid, l.next()) {
	}
	l.backup()
}

// errorf returns an error token and empties the input.
func (l *Scanner) errorf(format string, args ...interface{}) stateFn {
	l.token = Token{Error, l.start, fmt.Sprintf(format, args...)}
	l.start = 0
	l.pos = 0
	l.input = l.input[:0]
	return nil
}

// New creates and returns a new scanner.
func New(context value.Context, name string, r io.ByteReader) *Scanner {
	l := &Scanner{
		r:       r,
		name:    name,
		line:    1,
		context: context,
	}
	return l
}

// Next returns the next token.
func (l *Scanner) Next() Token {
	l.readOK = true
	l.lastRune = eof
	l.lastWidth = 0
	l.token = Token{EOF, l.pos, "EOF"}
	state := lexAny
	for {
		state = state(l)
		if state == nil {
			return l.token
		}
	}
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
		return l.emit(Newline)
	}
	return lexAny
}

// lexAny scans non-space items.
func lexAny(l *Scanner) stateFn {
	switch r := l.next(); {
	case r == eof:
		return nil
	case r == '\n':
		return l.emit(Newline)
	case r == ';':
		return l.emit(Semicolon)
	case r == '#':
		return lexComment
	case isSpace(r):
		return lexSpace
	case r == '\'' || r == '"':
		l.backup() // So lexQuote can read the quote character.
		return lexQuote
	case r == '`':
		return lexRawQuote
	case r == '-' || r == '+':
		// It's an operator if it's preceded immediately (no spaces) by an operand, which is
		// an identifier, an indexed expression, or a parenthesized expression.
		// Otherwise it could be a signed number.
		if l.start > 0 {
			rr, _ := utf8.DecodeLastRuneInString(l.input[:l.start])
			if isAlphaNumeric(rr) || rr == ')' || rr == ']' {
				return lexOperator
			}
			// Ugly corner case: inner product starting with '-' or '+'.
			if r1, r2 := l.peek2(); r1 == '.' && !l.isNumeral(r2) {
				return lexOperator
			}
		}
		fallthrough
	case r == '.' || '0' <= r && r <= '9':
		l.backup()
		return lexComplex
	case r == '=':
		if l.peek() != '=' {
			return l.emit(Assign)
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
		return l.emit(LeftBrack)
	case r == ':':
		return l.emit(Colon)
	case r == ']':
		return l.emit(RightBrack)
	case r == '(':
		return l.emit(LeftParen)
	case r == ')':
		return l.emit(RightParen)
	case r <= unicode.MaxASCII && unicode.IsPrint(r):
		return l.emit(Char)
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
	// Skips over the pending input.
	l.start = l.pos
	return lexAny
}

// lexIdentifier scans an alphanumeric.
// If the input base is greater than 10, some identifiers
// are actually numbers. We handle this here.
func lexIdentifier(l *Scanner) stateFn {
	for isAlphaNumeric(l.peek()) {
		l.next()
	}
	if !l.atTerminator() {
		return l.errorf("bad character %#U", l.next())
	}
	// Some identifiers are operators.
	word := l.input[l.start:l.pos]
	switch {
	case word == "op":
		return l.emit(Op)
	case word == "o" && l.peek() == '.':
		return lexOperator
	case l.defined(word):
		return lexOperator
	case isAllDigits(word, l.context.Config().InputBase()):
		// Mistake: back up and scan it as a number.
		l.pos = l.start
		return lexComplex
	}
	return l.emit(Identifier)
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
				return l.emit(Operator) // Up to but not including the period.
			}
			prevPos := l.pos
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
				word := l.input[prevPos:l.pos]
				if !l.defined(word) {
					return l.errorf("%s not an operator", word)
				}
			}
		}
	}
	if isIdentifier(l.input[l.start:l.pos]) {
		return l.emit(Identifier)
	}
	return l.emit(Operator)
}

// atTerminator reports whether the input is at valid termination character to
// appear after an identifier or number element.
func (l *Scanner) atTerminator() bool {
	r := l.peek()
	if r == eof || isSpace(r) || isEndOfLine(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
		return true
	}
	// It could be a compound operator like o.*. Ugly!
	if l.pos < len(l.input) {
		r1, r2 := l.peek2()
		if r1 == 'o' && r2 == '.' {
			return true
		}
	}
	return false
}

func lexComplex(l *Scanner) stateFn {
	ok, fn := acceptNumber(l, true)
	if !ok {
		return fn
	}
	if !l.accept("j") {
		return l.emit(Number)
	}
	ok, _ = acceptNumber(l, true)
	if !ok {
		return l.errorf("bad complex number syntax: %s", l.input[l.start:l.pos])
	}
	return l.emit(Number)
}

// acceptNumber scans a number: decimal, octal, hex, float. This
// isn't a perfect number scanner - for instance it accepts "." and "0x0.2"
// and "089" - but when it's wrong the input is invalid and the parser (via
// strconv) will notice. The realPart boolean says whether this might be
// the first half of a complex number, permitting a 'j' afterwards. If it's
// false, we've just seen a 'j' and we need another number.
// It returns the next lex function to run.
func acceptNumber(l *Scanner, realPart bool) (bool, stateFn) {
	// Optional leading sign.
	if l.accept("+-") && realPart {
		// Might not be a number.
		r := l.peek()
		// Might be a scan or reduction.
		if r == '/' || r == '\\' {
			l.next()
			return false, l.emit(Operator)
		}
		if r != '.' && !l.isNumeral(r) {
			return false, lexOperator
		}
	}
	if !l.scanNumber(true, realPart) {
		l.errorf("bad number syntax: %s", l.input[l.start:l.pos])
		return false, lexAny
	}
	r := l.peek()
	if r != '/' {
		return true, lexAny
	}
	// Might be a rational.
	l.accept("/")

	if realPart {
		if r := l.peek(); r != '.' && !l.isNumeral(r) {
			// Oops, not a rational. Back up!
			l.pos--
			return true, lexOperator
		}
	}
	// Note: No signs here. 1/-2 is (1 / -2) not (-1/2). This differs from 'j' but feels right;
	// you don't write 1/-2 for -1/2. The sign should be first.
	if !l.scanNumber(false, realPart) {
		l.errorf("bad number syntax: %s", l.input[l.start:l.pos])
		return false, lexAny
	}
	if l.peek() == '.' {
		l.errorf("bad number syntax: %s", l.input[l.start:l.pos+1])
		return false, lexAny
	}
	return true, lexAny
}

func (l *Scanner) scanNumber(followingSlashOK, followingJOK bool) bool {
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
	r := l.peek()
	if followingSlashOK && r == '/' {
		return true
	}
	if followingJOK && r == 'j' {
		return true
	}
	// Next thing mustn't be alphanumeric except possibly an o for outer product (3o.+2) or a complex.
	if r != 'o' && isAlphaNumeric(r) {
		l.next()
		return false
	}
	if r == '.' || !l.atTerminator() {
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
// The next character is the quote.
func lexQuote(l *Scanner) stateFn {
	quote := l.next()
	for {
		switch l.next() {
		case '\\':
			if r := l.next(); r != eof && r != '\n' {
				break
			}
			fallthrough
		case eof, '\n':
			return l.errorf("unterminated quoted string")
		case quote:
			return l.emit(String)
		}
	}

}

// lexRawQuote scans a raw quoted string.
func lexRawQuote(l *Scanner) stateFn {
	for {
		l.readOK = true // Here we do accept a newline mid-token.
		switch l.next() {
		case eof:
			return l.errorf("unterminated raw quoted string")
		case '`':
			return l.emit(String)
		}
	}
}

// isSpace reports whether r is a space character.
func isSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

// isEndOfLine reports whether r is an end-of-line (really end-of-statement) character.
func isEndOfLine(r rune) bool {
	return r == '\n' || r == ';'
}

// isIdentifier reports whether the slice is a valid identifier.
func isIdentifier(s string) bool {
	if len(s) == 1 && s[0] == '_' {
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

// isNumeral reports whether r is a numeral in the input base.
// A decimal digit is always taken as a numeral, because otherwise parsing
// would be muddled. (In base 8, 039 shouldn't be scanned as two numbers.)
// The parser will check that the scanned number is legal.
func (l *Scanner) isNumeral(r rune) bool {
	if '0' <= r && r <= '9' {
		return true
	}
	base := l.context.Config().InputBase()
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

// isAllDigits reports whether s consists of digits in the specified base,
// includig possibly one 'j'.
func isAllDigits(s string, base int) bool {
	top := 'a' + rune(base-10) - 1
	TOP := 'A' + rune(base-10) - 1
	sawJ := false
	for _, c := range s {
		if c == 'j' && !sawJ {
			sawJ = true
			continue
		}
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
		if l.peek() == '=' {
			l.next()
		}
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

// defined reports whether the argument has been defined as a variable or operator.
func (l *Scanner) defined(word string) bool {
	return exec.Predefined(word) || l.context.UserDefined(word, true)
}
