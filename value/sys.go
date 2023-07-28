// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"
	"math/big"
	"time"
)

const sysHelp = `
"help":      print this text and return iota 0
"base":      the input and output base settings as
             a vector of two integers
"cpu":       the processor timing for the last evaluation
             as a vector in units of seconds:
               real user(cpu) system(cpu)
"date":      the current time as a vector of numbers:
               year month day hour minute second
"format":    the output format setting
"ibase":     the input base (ibase) setting
"maxbits":   the maxdbits setting
"maxdigits": the maxdigits setting
"maxstack":  the maxstack setting
"obase":     the output base (obase) setting
"origin":    the index origin setting
"prompt":    the prompt setting
"time":      the current time in Unix format`

// sys implements the variegated "sys" unary operator.
func sys(c Context, v Value) Value {
	vv := v.(Vector)
	if !allChars(vv) {
		Errorf("sys %s not defined", v)
	}
	arg := fmt.Sprint(vv) // Will print as "(argument)"
	arg = arg[1 : len(arg)-1]
	switch arg {
	case "help":
		fmt.Fprint(c.Config().Output(), sysHelp)
		return empty
	case "base":
		return NewIntVector(c.Config().Base())
	case "cpu":
		real, user, sys := c.Config().CPUTime()
		var r, u, s big.Float
		r.SetFloat64(real.Seconds())
		u.SetFloat64(user.Seconds())
		s.SetFloat64(sys.Seconds())
		vec := make([]Value, 3)
		vec[0] = BigFloat{&r}
		vec[1] = BigFloat{&u}
		vec[2] = BigFloat{&s}
		return NewVector(vec)
	case "date":
		return newCharVector(time.Now().Format(time.UnixDate))
	case "format":
		return newCharVector(fmt.Sprintf("%q", c.Config().Format()))
	case "ibase":
		return Int(c.Config().InputBase())
	case "maxbits":
		return Int(c.Config().MaxBits())
	case "maxdigits":
		return Int(c.Config().MaxDigits())
	case "maxstack":
		return Int(c.Config().MaxStack())
	case "obase":
		return Int(c.Config().OutputBase())
	case "origin":
		return Int(c.Config().Origin())
	case "prompt":
		return newCharVector(fmt.Sprintf("%q", c.Config().Prompt()))
	case "time":
		date := time.Now()
		y, m, d := date.Date()
		vec := make([]Value, 6)
		vec[0] = Int(y)
		vec[1] = Int(m)
		vec[2] = Int(d)
		vec[3] = Int(date.Hour())
		vec[4] = Int(date.Minute())
		sec := float64(date.Second())
		sec += float64(date.Nanosecond()) / 1e9
		var f big.Float
		f.SetFloat64(sec)
		vec[5] = BigFloat{&f}
		return NewVector(vec)
	default:
		Errorf("sys %q not defined", arg)
	}
	return zero
}

// newCharVector takes a string and returns its representation as a Vector of Chars.
func newCharVector(s string) Value {
	chars := []Char(s)
	vec := make([]Value, len(chars))
	for i, r := range chars {
		vec[i] = r
	}
	return NewVector(vec)
}
