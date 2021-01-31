// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"strings"
	"unicode/utf8"

	"robpike.io/ivy/config"
)

// fmtText returns a vector of Chars holding the string representation
// of the value v. The lhs u defines the format:
// 1 item: number of decimals, or if textual, the complete format.
// 2 items: width of field, number of decimals.
// 3 items: width of field, number of decimals, format char.
// For the 1-item variant, the format is as in Go, except that
// unlike in Go conversions can occur to coerce the value,
// for instance to print a floating point number as a decimal
// integer with '%d'.
func fmtText(c Context, u, v Value) Value {
	config := c.Config()
	format, verb := formatString(config, u)
	if format == "" {
		Errorf("illegal format %q", u.Sprint(config))
	}
	var b bytes.Buffer
	switch val := v.(type) {
	case Int, BigInt, BigRat, BigFloat, Char:
		formatOne(c, &b, format, verb, val)
	case Vector:
		if val.AllChars() && strings.ContainsRune("boOqsvxX", rune(verb)) {
			// Print the string as a unit.
			fmt.Fprintf(&b, format, val.Sprint(debugConf))
		} else {
			for i, v := range val {
				if i > 0 {
					b.WriteByte(' ')
				}
				formatOne(c, &b, format, verb, v)
			}
		}
	case *Matrix:
		val.fprintf(c, &b, format)
	default:
		Errorf("cannot format '%s'", val.Sprint(config))
	}
	str := b.String()
	elem := make([]Value, utf8.RuneCountInString(str))
	for i, r := range str {
		elem[i] = Char(r)
	}
	return NewVector(elem)
}

// formatString returns the format string given u, the lhs of a binary text invocation.
func formatString(c *config.Config, u Value) (string, byte) {
	switch val := u.(type) {
	case Int:
		return fmt.Sprintf("%%.%df", val), 'f'
	case Char:
		s := fmt.Sprintf("%%%c", val)
		return s, verbOf(s) // Error check is in there.
	case Vector:
		if val.AllChars() {
			s := val.Sprint(c)
			if !strings.ContainsRune(s, '%') {
				s = "%" + s
			}
			verb := verbOf(s)
			return s, verb
		}
		char := Char('f')
		switch len(val) {
		case 1:
			// Decimal count only.
			dec, ok := val[0].(Int)
			if ok {
				return fmt.Sprintf("%%.%df", dec), 'f'
			}
		case 3:
			// Width count, and char.
			var ok bool
			char, ok = val[2].(Char)
			if !ok {
				break
			}
			char |= ' '
			if char != 'e' && char != 'f' && char != 'g' {
				break
			}
			fallthrough
		case 2:
			// Width and decimal count.
			wid, ok1 := val[0].(Int)
			dec, ok2 := val[1].(Int)
			if ok1 && ok2 {
				return fmt.Sprintf("%%%d.%d%c", wid, dec, char), byte(char)
			}
		}
	}
	return "", 'f'
}

// verbOf returns the first formatting verb, after an obligatory percent, in the string,
// skipping %% of course. It returns 0 if no verb is found. It does some rudimentary
// validation.
func verbOf(format string) byte {
	percent := strings.IndexByte(format, '%')
	if percent < 0 {
		Errorf("invalid format %q", format)
	}
	s := format[percent+1:]
Loop:
	for i, c := range s {
		if c == '%' {
		}
		switch c {
		// Flags etc.
		case '+', '-', '#', ' ', '0':
			continue
		// Digits etc.
		case '.', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			continue
		// Special case for %%: go on to next verb.
		case '%':
			return verbOf(s[i+1:])
		case 'b', 'c', 'd', 'e', 'E', 'f', 'F', 'g', 'G', 'o', 'O', 'q', 's', 't', 'U', 'v', 'x', 'X':
			return byte(c)
		default:
			break Loop
		}
	}
	Errorf("invalid format %q", format)
	panic("not reached")
}

// formatOne prints a scalar value into b with the specified format.
// How it does this depends on the format, permitting us to use %d on
// floats and rationals, for example.
func formatOne(c Context, w io.Writer, format string, verb byte, v Value) {
	switch verb {
	case 't': // Boolean. TODO: Should be 0 or 1, but that's messy. Odd case anyway.
		fmt.Fprintf(w, format, toBool(v))
	case 'v':
		fmt.Fprintf(w, format, v.Sprint(debugConf)) // Cleanest output.
	case 'c', 'U':
		// Dig inside the values to find or form a char.
		switch val := v.(type) {
		case Int:
			fmt.Fprintf(w, format, int32(val))
		case Char:
			fmt.Fprintf(w, format, uint32(val))
		case BigInt:
			Errorf("value too large for %%%c: %v", verb, v)
		case BigRat:
			i, _ := val.Float64()
			fmt.Fprintf(w, format, int64(i))
		case BigFloat:
			i, _ := val.Int64()
			fmt.Fprintf(w, format, i)
		}
		return
	case 's', 'q':
		// Chars become strings.
		switch val := v.(type) {
		case Int:
			fmt.Fprintf(w, format, string(int32(val)))
		case Char:
			fmt.Fprintf(w, format, string(int32(val)))
		case BigInt:
			Errorf("value too large for %%%c: %v", verb, v)
		case BigRat:
			i, _ := val.Float64()
			fmt.Fprintf(w, format, string(int32(i)))
		case BigFloat:
			i, _ := val.Int64()
			fmt.Fprintf(w, format, string(int32(i)))
		}
		return
	case 'b', 'd', 'o', 'O', 'x', 'X':
		// Dig inside the values to find or form an int. Avoid default String method.
		switch val := v.(type) {
		case Int:
			fmt.Fprintf(w, format, int64(val))
		case Char:
			fmt.Fprintf(w, format, uint32(val))
		case BigInt:
			fmt.Fprintf(w, format, val.Int)
		case BigRat:
			// This formats numerator and denomator separately,
			// but that's like applying the format to a vector.
			fmt.Fprintf(w, format, val.Num())
			fmt.Fprint(w, "/")
			fmt.Fprintf(w, format, val.Denom())
		case BigFloat:
			// Hex float format is special, but big.Float does not implement 'X'.
			switch verb {
			case 'x':
				fmt.Fprintf(w, format, val.Float)
				return
			case 'X':
				Errorf("%%X not implemented for float: %v", val)
			}
			i, _ := val.Int(big.NewInt(0)) // TODO: Truncates towards zero. Do rounding?
			fmt.Fprintf(w, format, i)
		}
		return
	case 'e', 'E', 'f', 'F', 'g', 'G':
		f := newFloat(c)
		switch val := v.(type) {
		case Int:
			f.SetInt64(int64(val))
			fmt.Fprintf(w, format, f)
		case Char:
			f.SetInt64(int64(val))
			fmt.Fprintf(w, format, f)
		case BigInt:
			f.SetInt(val.Int)
			fmt.Fprintf(w, format, f)
		case BigRat:
			f.SetRat(val.Rat)
			fmt.Fprintf(w, format, f)
		case BigFloat:
			fmt.Fprintf(w, format, val.Float)
		}
	default:
		fmt.Fprintf(w, format, v)
	}
}
