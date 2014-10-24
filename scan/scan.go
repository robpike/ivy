// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scan

import (
	"fmt"
	"io"

	"strings"
	"unicode"
	"unicode/utf8"
)

type Pos int // Byte position.

// Token represents a token or text string returned from the scanner.
type Token struct {
	Type Type   // The type of this item.
	Pos  Pos    // The starting position, in bytes, of this item in the input. // TODO WRONG
	Text string // The text of this item.
}

// Type identifies the type of lex items.
type Type int

const (
	Nothing Type = iota
	Error        // error occurred; value is text of error
	Newline
	// Interesting things
	Char         // printable ASCII character; grab bag for comma etc.
	CharConstant // character constant
	Dot          // dot
	Dollar       // dollar
	EOF
	GreaterOrEqual // '>='
	Identifier     // alphanumeric identifier
	LeftParen      // '('
	Number         // simple number, including imaginary
	Operator       // known operator
	RawString      // raw quoted string (includes quotes)
	RightParen     // ')'
	Space          // run of spaces separating
	String         // quoted string (includes quotes)
	// Keywords appear after all the rest.
	Keyword // used only to delimit the keywords
)

var operatorWord = map[string]bool{
	"abs":  true,
	"div":  true,
	"idiv": true,
	"imod": true,
	"int":  true,
	"iota": true,
	"mod":  true,
}

func (t Type) String() string {
	switch t {
	case Nothing:
		return "Nothing"
	case Error:
		return "Error"
	case Newline:
		return "Newline"
	case Char:
		return "Char"
	case CharConstant:
		return "CharConstant"
	case Dot:
		return "."
	case Dollar:
		return "$"
	case EOF:
		return "EOF"
	case Identifier:
		return "Identifier"
	case LeftParen:
		return "LeftParen"
	case Number:
		return "Number"
	case RawString:
		return "RawString"
	case RightParen:
		return "RightParen"
	case Space:
		return "Space"
	case String:
		return "String"
	// Keywords
	default:
		return fmt.Sprintf("type %d", t)
	}
}

func (i Token) String() string {
	switch {
	case i.Type == EOF:
		return "EOF"
	case i.Type == Error:
		return "error: " + i.Text
	case i.Type > Keyword:
		return fmt.Sprintf("<%s>", i.Text)
	case len(i.Text) > 10:
		return fmt.Sprintf("%s: %.10q...", i.Type, i.Text)
	}
	return fmt.Sprintf("%s: %q", i.Type, i.Text)
}

var key = map[string]Type{
// No keywords (yet?).
}

const eof = -1

// stateFn represents the state of the scanner as a function that returns the next state.
type stateFn func(*Scanner) stateFn

// Scanner holds the state of the scanner.
type Scanner struct {
	Tokens     chan Token // channel of scanned items
	r          io.ByteReader
	name       string // the name of the input; used only for error reports
	buf        []byte
	input      string  // the line of text being scanned.
	leftDelim  string  // start of action
	rightDelim string  // end of action
	state      stateFn // the next lexing function to enter
	pos        Pos     // current position in the input
	start      Pos     // start position of this item
	width      Pos     // width of last rune read from input
	lastPos    Pos     // position of most recent item returned by nextToken
	parenDepth int     // nesting depth of ( ) exprs
}

// loadLine reads the next line of input and stores it in the input.
func (l *Scanner) loadLine() {
	l.buf = l.buf[:0]
	for {
		c, err := l.r.ReadByte()
		if err != nil {
			l.input = string(l.buf)
			return
		}
		l.buf = append(l.buf, c)
		if c == '\n' {
			break
		}
	}
	l.input = string(l.buf)
	l.pos = 0
	l.start = 0
}

// next returns the next rune in the input.
func (l *Scanner) next() rune {
	if int(l.pos) >= len(l.input) {
		l.loadLine()
		if len(l.input) == 0 {
			l.width = 0
			return eof
		}
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.width = Pos(w)
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

// emit passes an item back to the client.
func (l *Scanner) emit(t Type) {
	s := l.input[l.start:l.pos]
	if t == Number && len(s) > 0 && s[0] == '_' {
		// TODO Ugly. Is there a better way?
		s = "-" + s[1:]
	}
	//fmt.Printf("EMIT %q %d %d type %s\n", s, l.start, l.pos, t)
	l.Tokens <- Token{t, l.start, s}
	l.start = l.pos
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

// lineNumber reports which line we're on, based on the position of
// the previous item returned by nextToken. Doing it this way
// means we don't have to worry about peek double counting.
func (l *Scanner) lineNumber() int {
	return 1 + strings.Count(l.input[:l.lastPos], "\n")
}

// errorf returns an error token and terminates the scan by passing
// back a nil pointer that will be the next state, terminating l.nextToken.
func (l *Scanner) errorf(format string, args ...interface{}) stateFn {
	l.Tokens <- Token{Error, l.start, fmt.Sprintf(format, args...)}
	return nil
}

// nextToken returns the next item from the input.
func (l *Scanner) nextToken() Token {
	token := <-l.Tokens
	l.lastPos = token.Pos // TODO
	return token
}

// New creates a new scanner for the input string.
func New(name string, r io.ByteReader) *Scanner {
	l := &Scanner{
		r:      r,
		name:   name,
		Tokens: make(chan Token),
	}
	go l.run()
	return l
}

// run runs the state machine for the Scanner.
func (l *Scanner) run() {
	for l.state = lexSpace; l.state != nil; {
		l.state = l.state(l)
	}
}

// state functions

const (
	startComment = "//"
)

// lexComment scans a comment. The comment marker is known to be present.
func lexComment(l *Scanner) stateFn {
	l.pos += Pos(len(startComment))
	for {
		r := l.peek()
		if r == eof {
			break
		}
		l.next()
		l.pos += Pos(utf8.RuneLen(r))
		if r == '\n' {
			break
		}
	}
	if len(l.input) > 0 {
		l.pos = Pos(len(l.input))
		l.start = l.pos - 1
		l.emit(Newline) // TODO: pass comments up?
	}
	return lexSpace
}

// lexAny scans non-space items.
func lexAny(l *Scanner) stateFn {
	if l.pos >= Pos(len(l.input)) {
		return nil
	}
	if strings.HasPrefix(l.input[l.pos:], startComment) {
		return lexComment
	}
	switch r := l.next(); {
	case r == eof:
		return nil
	case r == '\n': // TODO: \r
		l.emit(Newline)
		return lexSpace
	case isSpace(r):
		return lexSpace
	case l.isOperator(r):
		l.emit(Operator)
		return lexSpace
	case r == '"':
		return lexQuote
	case r == '`':
		return lexRawQuote
	case r == '\'':
		return lexChar
	case r == '$':
		l.emit(Dollar)
		return lexAny
	case r == '.':
		if !unicode.IsDigit(l.peek()) {
			l.emit(Dot)
			return lexAny
		}
		fallthrough // '.' can start a number.
	case r == '_' || '0' <= r && r <= '9':
		l.backup()
		return lexNumber
	case '0' <= r && r <= '9':
		l.backup()
		return lexNumber
	case isAlphaNumeric(r):
		l.backup()
		return lexIdentifier
	case r == '(':
		l.emit(LeftParen)
		l.parenDepth++
		return lexAny
	case r == ')':
		l.emit(RightParen)
		l.parenDepth--
		if l.parenDepth < 0 {
			return l.errorf("unexpected right paren %#U", r)
		}
		return lexAny
	case r <= unicode.MaxASCII && unicode.IsPrint(r):
		l.emit(Char)
		return lexAny
	default:
		return l.errorf("unrecognized character in action: %#U", r)
	}
	return lexAny
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
			if operatorWord[l.input[l.start:l.pos]] {
				l.emit(Operator)
			} else {
				l.emit(Identifier)
			}
			break Loop
		}
	}
	return lexAny
}

// atTerminator reports whether the input is at valid termination character to
// appear after an identifier. Breaks .X.Y into two pieces. Also catches cases
// like "$x+2" not being acceptable without a space, in case we decide one
// day to implement arithmetic.
func (l *Scanner) atTerminator() bool {
	r := l.peek()
	if isSpace(r) || isEndOfLine(r) {
		return true
	}
	switch r {
	case eof, '.', ',', '|', ':', ')', '(', '$':
		return true
	}
	if l.isOperator(r) {
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
	l.emit(CharConstant)
	return lexAny
}

// lexNumber scans a number: decimal, octal, hex, float, or imaginary. This
// isn't a perfect number scanner - for instance it accepts "." and "0x0.2"
// and "089" - but when it's wrong the input is invalid and the parser (via
// strconv) will notice.
func lexNumber(l *Scanner) stateFn {
	// Optional leading sign.
	l.accept("_")
	if !l.scanNumber() {
		return l.errorf("bad number syntax: %q", l.input[l.start:l.pos])
	}
	l.emit(Number)
	return lexAny
}

func (l *Scanner) scanNumber() bool {
	// Is it hex?
	digits := "0123456789"
	if l.accept("0") && l.accept("xX") {
		digits = "0123456789abcdefABCDEF"
	}
	l.acceptRun(digits)
	if l.accept(".") {
		l.acceptRun(digits)
	}
	if l.accept("eE") {
		l.accept("+-")
		l.acceptRun("0123456789")
	}
	// Next thing mustn't be alphanumeric.
	if isAlphaNumeric(l.peek()) {
		l.next()
		return false
	}
	return true
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
		case eof, '\n':
			return l.errorf("unterminated raw quoted string")
		case '`':
			break Loop
		}
	}
	l.emit(RawString)
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

// isAlphaNumeric reports whether r is an alphabetic, digit, or underscore.
func isAlphaNumeric(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// isOperator reports whether r is an operator. It may advance the lexer one character
// if it is a two-character operator.
func (l *Scanner) isOperator(r rune) bool {
	switch r {
	case '+', '-', '/', '%', '&', '|', '^':
		// No follow-on possible.
	case ':':
		switch l.peek() {
		case '=':
			l.next()
		default:
			return false
		}
	case '=':
		switch l.peek() {
		case '=':
			l.next()
		}
	case '!':
		switch l.peek() {
		case '=':
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
	default:
		return false
	}
	return true
}
