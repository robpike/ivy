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
"base":      the input and output base settings as a vector of two integers
"cpu":       the processor timing for the last evaluation
             as a vector in units of seconds:
               real user(cpu) system(cpu)
"date":      the current time in Unix date format
               year month day hour minute second
"format":    the output format setting
"ibase":     the input base (ibase) setting
"maxbits":   the maxdbits setting
"maxdigits": the maxdigits setting
"maxstack":  the maxstack setting
"obase":     the output base (obase) setting
"origin":    the index origin setting
"prompt":    the prompt setting
"sec":       the time in seconds since
               Jan 1 00:00:00 1970 UTC
"time":      the time in the local time zone as a vector of numbers:
               year month day hour minute second


To convert seconds to a time vector:
  'T' encode sys 'sec'
To convert a time vector to a seconds value:
  'T' decode sys 'time'
To print seconds in Unix date format:
  'T' text sys 'sec'`

// sys implements the variegated "sys" unary operator.
func sys(c Context, v Value) Value {
	vv := v.(Vector)
	if !allChars(vv) {
		Errorf("sys %s not defined", v)
	}
	arg := fmt.Sprint(vv) // Will print as "(argument)"
	switch arg[1 : len(arg)-1] {
	case "help":
		fmt.Fprint(c.Config().Output(), sysHelp)
		return empty
	case "base":
		return NewIntVector(c.Config().Base())
	case "cpu":
		real, user, sys := c.Config().CPUTime()
		vec := make([]Value, 3)
		vec[0] = BigFloat{big.NewFloat(real.Seconds())}
		vec[1] = BigFloat{big.NewFloat(user.Seconds())}
		vec[2] = BigFloat{big.NewFloat(sys.Seconds())}
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
	case "sec":
		return BigFloat{big.NewFloat(float64(time.Now().UnixNano()) / 1e9)}
	case "time":
		return timeVec(time.Now())
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

// encodeTime returns a sys "time" vector given a seconds value.
// We know the first argument is all chars and not empty.
func encodeTime(u, v Vector) Value {
	r := rune(u[0].(Char))
	if r != 't' && r != 'T' {
		Errorf("illegal left operand %s for encode", u)
	}
	// TODO len(v) > 1
	switch t := v[0].(type) {
	case Int:
		return secEncodeTime(float64(t))
	case BigInt:
		f, _ := t.Float64()
		return secEncodeTime(f)
	case BigFloat:
		f, _ := t.Float64()
		return secEncodeTime(f)
	case BigRat:
		f, _ := t.Float64()
		return secEncodeTime(f)
	default:
		Errorf("bad time value %s in encode", v)
	}
	return zero

}

// secEncodeTime converts the "sys 'sec'" value into an unpacked time vector.
func secEncodeTime(sec float64) Value {
	nsec := 1e9 * (sec - float64(int64(sec)))
	return timeVec(time.Unix(int64(sec), int64(nsec)).In(time.Now().Location()))
}

// timeVec returns the time unpacked into year, month, day, hour, minute and second.
func timeVec(date time.Time) Vector {
	y, m, d := date.Date()
	vec := make([]Value, 6)
	vec[0] = Int(y)
	vec[1] = Int(m)
	vec[2] = Int(d)
	vec[3] = Int(date.Hour())
	vec[4] = Int(date.Minute())
	sec := float64(date.Second()) + float64(date.Nanosecond())/1e9
	vec[5] = BigFloat{big.NewFloat(sec)}
	return NewVector(vec)
}

// decodeTime returns a second value given a sys "time" vector.
func decodeTime(u, v Vector) Value {
	r := rune(u[0].(Char))
	if r != 't' && r != 'T' {
		Errorf("illegal left operand %s for decode", u)
	}
	year, month, day, hour, min, sec, nsec := 0, 1, 1, 0, 0, 0, 0
	toInt := func(v Value) int {
		i, ok := v.(Int)
		if ok {
			return int(i)
		}
		b, ok := v.(BigInt)
		if !ok || !b.IsInt64() {
			Errorf("illegal right operand %s in decode", v)
		}
		return int(b.Int64())
	}
	switch len(v) {
	default:
		Errorf("invalid time vector %s", v)
	case 6:
		switch s := v[5].(type) {
		default:
			Errorf("illegal right operand %s in decode", v)
		case Int:
			sec = int(s)
		case BigInt:
			if !s.IsInt64() {
				Errorf("illegal right operand %s in decode", v)
			}
			sec = int(s.Int64())
		case BigRat:
			var f big.Float
			f.SetRat(s.Rat)
			f64, _ := f.Float64()
			sec, nsec = secNsec(f64)
		case BigFloat:
			f64, _ := s.Float.Float64()
			sec, nsec = secNsec(f64)
		}
		fallthrough
	case 5:
		min = toInt(v[4])
		fallthrough
	case 4:
		hour = toInt(v[3])
		fallthrough
	case 3:
		day = toInt(v[2])
		fallthrough
	case 2:
		month = toInt(v[1])
		fallthrough
	case 1:
		year = toInt(v[0])
	}
	t := time.Date(year, time.Month(month), day, hour, min, sec, nsec, time.Now().Location())
	return BigFloat{big.NewFloat(float64(t.UnixNano()) / 1e9)}
}

// secNsec converts a seconds value into whole seconds and nanoseconds.
func secNsec(f float64) (sec, nsec int) {
	return int(f), int(1e9 * (f - float64(int64(f))))
}

// timeFromValue converts a seconds value into a time.Time, for
// the 'text' operator.
func timeFromValue(v Value) time.Time {
	var sec, nsec int
	switch t := v.(type) {
	case Int:
		sec, nsec = secNsec(float64(t))
	case BigInt:
		f, _ := t.Float64()
		sec, nsec = secNsec(f)
	case BigFloat:
		f, _ := t.Float64()
		sec, nsec = secNsec(f)
	case BigRat:
		f, _ := t.Float64()
		sec, nsec = secNsec(f)
	default:
		Errorf("bad time value %s in text", v)
	}
	return time.Unix(int64(sec), int64(nsec)).In(time.Now().Location())
}
