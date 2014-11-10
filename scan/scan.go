// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate stringer -type Type

package scan

import (
	"fmt"
	"io"

	"code.google.com/p/rspace/ivy/config"

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
	EOF   Type = iota // zero value so closed channel delivers EOF
	Error             // error occurred; value is text of error
	Newline
	// Interesting things
	Char           // printable ASCII character; grab bag for comma etc.
	CharConstant   // character constant
	Assign         // '='
	GreaterOrEqual // '>='
	Identifier     // alphanumeric identifier
	LeftBrack      // '['
	LeftParen      // '('
	Number         // simple number
	Operator       // known operator
	Rational       // rational number like 2/3
	RawString      // raw quoted string (includes quotes)
	RightBrack     // ']'
	RightParen     // ')'
	Space          // run of spaces separating
	String         // quoted string (includes quotes)
)

var operatorWord = map[string]bool{
	"abs":   true,
	"and":   true,
	"ceil":  true,
	"div":   true,
	"down":  true,
	"drop":  true,
	"floor": true,
	"grade": true,
	"idiv":  true,
	"imod":  true,
	"iota":  true,
	"min":   true,
	"max":   true,
	"mod":   true,
	"nand":  true,
	"nor":   true,
	"not":   true,
	"or":    true,
	"rho":   true,
	"sgn":   true,
	"take":  true,
	"up":    true,
	"xor":   true,
}

// isBinary identifies the binary operators; these can be used in reductions.
var isBinary = map[string]bool{
	"+":    true,
	"-":    true,
	"*":    true,
	"/":    true,
	"idiv": true,
	"imod": true,
	"div":  true,
	"mod":  true,
	"**":   true,
	"&":    true,
	"|":    true,
	"^":    true,
	"<<":   true,
	">>":   true,
	"==":   true,
	"!=":   true,
	"<":    true,
	"<=":   true,
	">":    true,
	">=":   true,
	"[]":   true,
	"and":  true,
	"or":   true,
	"xor":  true,
	"iota": true,
	"min":  true,
	"max":  true,
	"rho":  true,
	",":    true, // Silly but not wrong.
}

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
	Tokens     chan Token // channel of scanned items
	config     *config.Config
	r          io.ByteReader
	done       bool
	name       string // the name of the input; used only for error reports
	buf        []byte
	input      string  // the line of text being scanned.
	leftDelim  string  // start of action
	rightDelim string  // end of action
	state      stateFn // the next lexing function to enter
	pos        Pos     // current position in the input
	start      Pos     // start position of this item
	width      Pos     // width of last rune read from input
}

// loadLine reads the next line of input and stores it in (appends it to) the input.
// (l.input may have data left over when we are called.)
func (l *Scanner) loadLine() {
	l.buf = l.buf[:0]
	for {
		c, err := l.r.ReadByte()
		if err != nil {
			l.done = true
			break
		}
		l.buf = append(l.buf, c)
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
	if Pos(len(l.input)) == l.start {
		return eof
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
	if l.config.Debug("tokens") {
		fmt.Printf("emit %s\n", Token{t, l.start, s})
	}
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

// errorf returns an error token and continues to scan.
func (l *Scanner) errorf(format string, args ...interface{}) stateFn {
	l.Tokens <- Token{Error, l.start, fmt.Sprintf(format, args...)}
	return lexAny
}

// New creates a new scanner for the input string.
func New(conf *config.Config, name string, r io.ByteReader) *Scanner {
	l := &Scanner{
		r:      r,
		config: conf,
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
	close(l.Tokens)
}

// state functions

const (
	startComment = "#"
)

// lexComment scans a comment. The comment marker is known to be present.
func lexComment(l *Scanner) stateFn {
	l.pos += Pos(len(startComment))
	for {
		r := l.next()
		if r == eof || r == '\n' {
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
	if strings.HasPrefix(l.input[l.pos:], startComment) {
		return lexComment
	}
	switch r := l.next(); {
	case r == eof:
		return nil
	case r == '\n': // TODO: \r
		l.emit(Newline)
		return lexAny
	case isSpace(r):
		return lexSpace
	case r == '"':
		return lexQuote
	case r == '`':
		return lexRawQuote
	case r == '\'':
		return lexChar
	case r == '-':
		// It's the start of a number iff there is nothing, a space or a paren before it.
		// Otherwise it's an operator.
		if l.start > 0 && (!isSpace(rune(l.input[l.start-1])) && l.input[l.start-1] != '(') { // FIX
			l.emit(Operator)
			return lexAny
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
	case l.isOperator(r): // Must be after numbers, so '-' can be a sign.
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
				return lexOperator
			} else {
				l.emit(Identifier)
			}
			break Loop
		}
	}
	return lexAny
}

// lexOperator completes scanning an operator. We have already accepted the + or
// whatever; there may be a reduction or inner product.
func lexOperator(l *Scanner) stateFn {
	// It might be an inner product or reduction, but only if it is a binary operator.
	if isBinary[l.input[l.start:l.pos]] {
		switch l.peek() {
		case '/':
			// Reduction.
			l.next()
		case '.':
			// Inner product
			l.next() // Accept the '.'.
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
				if !operatorWord[l.input[startRight:l.pos]] {
					return l.errorf("%s not an operator", l.input[startRight:l.pos])
				}
			}
		}
	}
	l.emit(Operator)
	return lexSpace
}

// atTerminator reports whether the input is at valid termination character to
// appear after an identifier.
func (l *Scanner) atTerminator() bool {
	r := l.peek()
	if isSpace(r) || isEndOfLine(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
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
	if l.accept("-") {
		// Might not be a number
		r := l.peek()
		if r != '.' && !unicode.IsDigit(r) {
			l.emit(Operator)
			return lexAny
		}
	}
	if !l.scanNumber() {
		return l.errorf("bad number syntax: %q", l.input[l.start:l.pos])
	}
	if l.peek() != '/' {
		l.emit(Number)
		return lexAny
	}
	// Might be a rational.
	l.accept("/")

	if r := l.peek(); r != '.' && !unicode.IsDigit(r) {
		// Oops, not a number. Hack!
		l.pos-- // back up before '/'
		l.emit(Number)
		l.accept("/")
		l.emit(Operator)
		return lexAny
	}
	if !l.scanNumber() {
		return l.errorf("bad number syntax: %q", l.input[l.start:l.pos])
	}
	l.emit(Rational)
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
	case '?', '+', '-', '/', '%', '&', '|', '^', '~', ',':
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
	default:
		return false
	}
	return true
}
