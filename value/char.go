// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"
	"strconv"
	"unicode/utf8"

	"robpike.io/ivy/config"
)

type Char rune

const (
	sQuote = '\''
	dQuote = "\""
)

func (c Char) String() string {
	return "(" + string(c) + ")"
}

func (c Char) Sprint(conf *config.Config) string {
	// We ignore the format - chars are always textual.
	// TODO: What about escapes?
	return string(c)
}

func (c Char) ProgString() string {
	return fmt.Sprintf("%q", rune(c))
}

func (c Char) Eval(Context) Value {
	return c
}

func (c Char) Inner() Value {
	return c
}

func (c Char) toType(conf *config.Config, which valueType) Value {
	switch which {
	case charType:
		return c
	case vectorType:
		return NewVector([]Value{c})
	case matrixType:
		return NewMatrix([]Value{one}, []Value{c})
	}
	Errorf("cannot convert %s to char", which)
	return nil
}

func (c Char) validate() Char {
	if !utf8.ValidRune(rune(c)) {
		Errorf("invalid char value %U\n", c)
	}
	return c
}

// ParseString parses a string. Single quotes and
// double quotes are both allowed (but must be consistent.)
// The result must contain only valid Unicode code points.
func ParseString(s string) string {
	str, ok := unquote(s)
	if !ok {
		Errorf("invalid string syntax")
	}
	if !utf8.ValidString(str) {
		Errorf("invalid code points in string")
	}
	return str
}

// unquote is a simplified strconv.Unquote that treats ' and " equally.
// Raw quotes are Go-like and bounded by ``.
// The return value is the string and a boolean rather than error, which
// was almost always the same anyway.
func unquote(s string) (t string, ok bool) {
	n := len(s)
	if n < 2 {
		return
	}
	quote := s[0]
	if quote != s[n-1] {
		return
	}
	s = s[1 : n-1]

	if quote == '`' {
		if contains(s, '`') {
			return
		}
		return s, true
	}
	if quote != '"' && quote != '\'' {
		return
	}
	if contains(s, '\n') {
		return
	}

	// Is it trivial?  Avoid allocation.
	if !contains(s, '\\') && !contains(s, quote) {
		return s, true
	}

	var runeTmp [utf8.UTFMax]byte
	buf := make([]byte, 0, 3*len(s)/2) // Try to avoid more allocations.
	for len(s) > 0 {
		c, multibyte, ss, err := strconv.UnquoteChar(s, quote)
		if err != nil {
			return
		}
		s = ss
		if c < utf8.RuneSelf || !multibyte {
			buf = append(buf, byte(c))
		} else {
			n := utf8.EncodeRune(runeTmp[:], c)
			buf = append(buf, runeTmp[:n]...)
		}
	}
	return string(buf), true
}

// contains reports whether the string contains the byte c.
func contains(s string, c byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return true
		}
	}
	return false
}
