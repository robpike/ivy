// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"
	"strconv"
	"unicode/utf8"
)

type Char rune

func (c Char) String() string {
	return "(" + string(c) + ")"
}

func (c Char) Rank() int {
	return 0
}

func (c Char) shrink() Value {
	return c
}

func (c Char) Sprint(Context) string {
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

func (c Char) toType(op string, ctx Context, which valueType) Value {
	switch which {
	case charType:
		return c
	case vectorType:
		return oneElemVector(c)
	case matrixType:
		return NewMatrix(ctx, []int{1}, NewVector(c))
	}
	ctx.Errorf("%s: cannot convert char to %s", op, which)
	return nil
}

func (c Char) validate(ctx Context) Char {
	if !utf8.ValidRune(rune(c)) {
		ctx.Errorf("invalid char value %U\n", c)
	}
	return c
}

// ParseString parses a string. Single quotes and
// double quotes are both allowed (but must be consistent.)
// The result must contain only valid Unicode code points.
func ParseString(c Context, s string) (string, error) {
	str, ok := unquote(s)
	if !ok {
		return "", fmt.Errorf("invalid string syntax")
	}
	if !utf8.ValidString(str) {
		return "", fmt.Errorf("invalid code points in string")
	}
	return str, nil
}

// unquote is a simplified strconv.Unquote that treats single and double quotes equally.
// Raw quotes are Go-like and bounded by back quotes.
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
