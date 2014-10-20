// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lex

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/scanner"
	"unicode"
)

// A Token represents an input item. It is a simple wrapping of rune, as
// returned by text/scanner.Scanner, plus a couple of extra values.
type Token rune

const (
	// Asm defines some two-character lexemes. We make up
	// a rune/Token value for them - ugly but simple.
	LSH Token = -1000 - iota // << Left shift.
	RSH                      // >> Logical right shift.
)

func (t Token) String() string {
	switch t {
	case scanner.EOF:
		return "EOF"
	case scanner.Ident:
		return "identifier"
	case scanner.Int:
		return "integer constant"
	case scanner.Float:
		return "float constant"
	case scanner.Char:
		return "rune constant"
	case scanner.String:
		return "string constant"
	case scanner.RawString:
		return "raw string constant"
	case scanner.Comment:
		return "comment"
	default:
		return fmt.Sprintf("%q", rune(t))
	}
}

func NewLexer(name string, directories []string) TokenReader {
	input := NewInput(name, directories)
	fd, err := os.Open(name)
	if err != nil {
		log.Printf("ivy: %s\n", err)
		return nil
	}
	input.Push(NewTokenizer(name, fd))
	return input
}

// A TokenReader is like a reader, but returns lex tokens of type Token. It also can tell you what
// the text of the most recently returned token is, and where it was found.
// The underlying scanner elides all spaces except newline, so the input looks like a  stream of
// Tokens; original spacing is lost but we don't need it.
type TokenReader interface {
	// Next returns the next token.
	Next() Token
	// The following methods all refer to the most recent token returned by Next.
	// Text returns the original string representation of the token.
	Text() string
	// FileName reports the source file name of the token.
	FileName() string
	// Line reports the source line number of the token.
	Line() int
	// SetPos sets the file and line number.
	SetPos(line int, file string)
}

// A LexToken is a token and its string value.
// A macro is stored as a sequence of LexTokens with spaces stripped.
type LexToken struct {
	Token
	text string
}

func (l LexToken) String() string {
	return l.text
}

// A Macro represents the definition of a #defined macro.
type Macro struct {
	name   string
	args   []string
	tokens []LexToken
}

// tokenize turns a string into a list of LexTokens; used to parse the -D flag.
func tokenize(str string) []LexToken {
	t := NewTokenizer("command line", strings.NewReader(str))
	var tokens []LexToken
	for {
		tok := t.Next()
		if tok == scanner.EOF {
			break
		}
		tokens = append(tokens, LexToken{Token: tok, text: t.Text()})
	}
	return tokens
}

// The rest of this file is implementations of TokenReader.

// A Tokenizer is a simple wrapping of text/scanner.Scanner, configured
// for our purposes and made a TokenReader. It forms the lowest level,
// turning text from readers into tokens.
type Tokenizer struct {
	tok      Token
	s        *scanner.Scanner
	line     int
	fileName string
}

func NewTokenizer(name string, r io.Reader) *Tokenizer {
	var s scanner.Scanner
	s.Init(r)
	// Newline is like a semicolon; other space characters are fine.
	s.Whitespace = 1<<'\t' | 1<<'\r' | 1<<' '
	// Don't skip comments: we need to count newlines.
	s.Mode = scanner.ScanChars |
		scanner.ScanFloats |
		scanner.ScanIdents |
		scanner.ScanInts |
		scanner.ScanStrings |
		scanner.ScanComments
	s.Position.Filename = name
	s.IsIdentRune = isIdentRune
	return &Tokenizer{
		s:        &s,
		line:     1,
		fileName: name,
	}
}

// We want center dot (·) and division slash (∕) to work as identifier characters.
func isIdentRune(ch rune, i int) bool {
	if unicode.IsLetter(ch) {
		return true
	}
	switch ch {
	case '_': // Underscore; traditional.
		return true
	case '\u00B7': // Represents the period in runtime.exit.
		return true
	case '\u2215': // Represents the slash in runtime/debug.setGCPercent
		return true
	}
	// Digits are OK only after the first character.
	return i > 0 && unicode.IsDigit(ch)
}

func (t *Tokenizer) Text() string {
	switch t.tok {
	case LSH:
		return "<<"
	case RSH:
		return ">>"
	}
	return t.s.TokenText()
}

func (t *Tokenizer) FileName() string {
	return t.fileName
}

func (t *Tokenizer) Line() int {
	return t.line
}

func (t *Tokenizer) SetPos(line int, file string) {
	t.line = line
	t.fileName = file
}

func (t *Tokenizer) Next() Token {
	s := t.s
	for {
		t.tok = Token(s.Scan())
		if t.tok != scanner.Comment {
			break
		}
		t.line += strings.Count(s.TokenText(), "\n")
		// TODO: If we ever have //go: comments in assembly, will need to keep them here.
		// For now, just discard all comments.
	}
	switch t.tok {
	case '\n':
		t.line++
	case '<':
		if s.Peek() == '<' {
			s.Next()
			t.tok = LSH
			return LSH
		}
	case '>':
		if s.Peek() == '>' {
			s.Next()
			t.tok = RSH
			return RSH
		}
	}
	return t.tok
}

// A Stack is a stack of TokenReaders. As the top TokenReader hits EOF,
// it resumes reading the next one down.
type Stack struct {
	tr []TokenReader
}

// Push adds tr to the top of the input stack. (Popping happens automatically.)
func (s *Stack) Push(tr TokenReader) {
	s.tr = append(s.tr, tr)
}

func (s *Stack) Next() Token {
	tok := s.tr[len(s.tr)-1].Next()
	for tok == scanner.EOF && len(s.tr) > 1 {
		// Pop the topmost item from the stack and resume with the next one down.
		// TODO: close file descriptor.
		s.tr = s.tr[:len(s.tr)-1]
		tok = s.Next()
	}
	return tok
}

func (s *Stack) Text() string {
	return s.tr[len(s.tr)-1].Text()
}

func (s *Stack) FileName() string {
	return s.tr[len(s.tr)-1].FileName()
}

func (s *Stack) Line() int {
	return s.tr[len(s.tr)-1].Line()
}

func (s *Stack) SetPos(line int, file string) {
	s.tr[len(s.tr)-1].SetPos(line, file)
}

// A Slice reads from a slice of LexTokens.
type Slice struct {
	tokens   []LexToken
	fileName string
	line     int
	pos      int
}

func NewSlice(fileName string, line int, tokens []LexToken) *Slice {
	return &Slice{
		tokens:   tokens,
		fileName: fileName,
		line:     line,
		pos:      -1, // Next will advance to zero.
	}
}

func (s *Slice) Next() Token {
	s.pos++
	if s.pos >= len(s.tokens) {
		return scanner.EOF
	}
	return s.tokens[s.pos].Token
}

func (s *Slice) Text() string {
	return s.tokens[s.pos].text
}

func (s *Slice) FileName() string {
	return s.fileName
}

func (s *Slice) Line() int {
	return s.line
}

func (s *Slice) SetPos(line int, file string) {
	// Cannot happen because we only have slices of already-scanned
	// text, but be prepared.
	s.line = line
	s.fileName = file
}

// Input is the main input: a stack of readers and some macro definitions.
// It also handles #include processing (by pushing onto the input stack)
// and parses and instantiates macro definitions.
type Input struct {
	Stack
	directories     []string
	beginningOfLine bool
	ifdefStack      []bool
}

func NewInput(name string, directories []string) *Input {
	return &Input{
		// include directories: look in source dir, then -I directories.
		directories:     append([]string{filepath.Dir(name)}, directories...),
		beginningOfLine: true,
	}
}

func (in *Input) Error(args ...interface{}) {
	fmt.Fprintf(os.Stderr, "asm: %s:%d: %s", in.FileName(), in.Line(), fmt.Sprintln(args...))
	os.Exit(1)
}

// expect is like Error but adds "got XXX" where XXX is a quoted representation of the most recent token.
func (in *Input) expectText(args ...interface{}) {
	in.Error(append(args, "; got", strconv.Quote(in.Text()))...)
}

func (in *Input) expectNewline(directive string) {
	tok := in.Stack.Next()
	if tok != '\n' {
		in.expectText("expected newline after", directive)
	}
}

func (in *Input) Next() Token {
	for {
		tok := in.Stack.Next()
		switch tok {
		case scanner.Ident:
			switch in.Text() {
			case "get":
				if !in.beginningOfLine {
					in.Error("'#' must be first item on line")
				}
				in.beginningOfLine = in.preprocessor()
				continue
			}
			fallthrough
		default:
			in.beginningOfLine = tok == '\n'
			return tok
		}
	}
	in.Error("recursive macro invocation")
	return 0
}

// preprocessor processes a preprocessor directive. It returns true iff it completes.
// The only one at the moment is "get".
func (in *Input) preprocessor() bool {
	text := in.Text()
	switch text {
	case "get":
		in.get()
	default:
		in.Error("unrecognized preprocessor directive: ", text)
	}
	return true
}

// get processing.
func (in *Input) get() {
	// Find and parse string.
	tok := in.Stack.Next()
	if tok != scanner.String {
		in.expectText("expected string after get")
	}
	name, err := strconv.Unquote(in.Text())
	if err != nil {
		in.Error("unquoting include file name: ", err)
	}
	in.expectNewline("get")
	// Push tokenizer for file onto stack.
	fd, err := os.Open(name)
	if err != nil {
		for _, dir := range in.directories {
			fd, err = os.Open(filepath.Join(dir, name))
			if err == nil {
				break
			}
		}
		if err != nil {
			in.Error("#get:", err)
		}
	}
	in.Push(NewTokenizer(name, fd))
}

func (in *Input) Push(r TokenReader) {
	if len(in.tr) > 100 {
		in.Error("input recursion")
	}
	in.Stack.Push(r)
}
