// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lex

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"code.google.com/p/rspace/ivy/scan"
)

func NewLexer(name string, r io.Reader, directories []string) TokenReader {
	input := NewInput(name, directories)
	input.Push(NewTokenizer(name, bufio.NewReader(r)))
	return input
}

// A TokenReader is like a reader, but returns lex tokens of type scan.Token. It also can tell you what
// the text of the most recently returned token is, and where it was found.
// The underlying scanner elides all spaces except newline, so the input looks like a  stream of
// Tokens; original spacing is lost but we don't need it.
type TokenReader interface {
	// Next returns the next token.
	Next() scan.Token
	// FileName reports the source file name of the current token.
	FileName() string
	// Line reports the source line number of the current token.
	Line() int
	// SetPos sets the file and line number.
	SetPos(line int, file string)
}

// The rest of this file is implementations of TokenReader.

// A Tokenizer is a simple wrapping of scan.Scanner, configured
// for our purposes and made a TokenReader. It forms the lowest level,
// turning text from readers into tokens.
type Tokenizer struct {
	tok      scan.Token
	s        *scan.Scanner
	line     int
	fileName string
}

func NewTokenizer(name string, r io.ByteReader) *Tokenizer {
	return &Tokenizer{
		s:        scan.New(name, r),
		line:     1,
		fileName: name,
	}
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

func (t *Tokenizer) Next() scan.Token {
	// TODO: Comments, count newlines
	t.tok = <-t.s.Tokens
	switch t.tok.Type {
	case scan.Newline:
		t.line++
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

func (s *Stack) Next() scan.Token {
	tok := s.tr[len(s.tr)-1].Next()
	for tok.Type == scan.EOF && len(s.tr) > 1 {
		// Pop the topmost item from the stack and resume with the next one down.
		// TODO: close file descriptor.
		s.tr = s.tr[:len(s.tr)-1]
		tok = s.Next()
	}
	return tok
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

// Input is the main input: a stack of readers and some macro definitions.
// It also handles processing (by pushing onto the input stack)
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
	// TODO	in.Error(append(args, "; got", strconv.Quote(in.Text()))...)
	in.Error(append(args, "; got SOMETHING ELSE TODO"))
}

func (in *Input) expectNewline(directive string) {
	tok := in.Stack.Next()
	if tok.Type != scan.Newline {
		in.expectText("expected newline after", directive)
	}
}

func (in *Input) Next() scan.Token {
	for {
		tok := in.Stack.Next()
		switch tok.Type {
		case scan.Number:
			switch tok.Text {
			case "get":
				if !in.beginningOfLine {
					in.Error("'#' must be first item on line")
				}
				in.beginningOfLine = in.preprocessor(tok.Text)
				continue
			}
			fallthrough
		default:
			in.beginningOfLine = tok.Type == scan.Newline
			return tok
		}
	}
}

// preprocessor processes a preprocessor directive. It returns true iff it completes.
// The only one at the moment is "get".
func (in *Input) preprocessor(text string) bool {
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
	if tok.Type != scan.String {
		in.expectText("expected string after get")
	}
	name, err := strconv.Unquote(tok.Text)
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
	in.Push(NewTokenizer(name, bufio.NewReader(fd)))
}

func (in *Input) Push(r TokenReader) {
	if len(in.tr) > 100 {
		in.Error("input recursion")
	}
	in.Stack.Push(r)
}
