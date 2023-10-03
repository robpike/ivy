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
"maxbits":   the maxbits setting
"maxdigits": the maxdigits setting
"maxstack":  the maxstack setting
"obase":     the output base (obase) setting
"origin":    the index origin setting
"prompt":    the prompt setting
"sec":       the time in seconds since
               Jan 1 00:00:00 1970 UTC
"time":      the current time in the configured time zone as a vector; the last
             element is the time zone in which the other values apply:
               year month day hour minute second seconds-east-of-UTC

To convert seconds to a time vector:
  'T' encode sys 'sec'
To convert a time vector to a seconds value:
  'T' decode sys 'time'
To print seconds in Unix date format:
  'T' text sys 'sec'`

// sys implements the variegated "sys" unary operator.
func sys(c Context, v Value) Value {
	conf := c.Config()
	vv := v.(Vector)
	if !allChars(vv) {
		Errorf("sys %s not defined", v)
	}
	arg := fmt.Sprint(vv) // Will print as "(argument)"
	switch arg[1 : len(arg)-1] {
	case "help":
		fmt.Fprint(conf.Output(), sysHelp)
		return empty
	case "base":
		return NewIntVector(conf.Base())
	case "cpu":
		real, user, sys := conf.CPUTime()
		vec := make([]Value, 3)
		vec[0] = BigFloat{big.NewFloat(real.Seconds())}
		vec[1] = BigFloat{big.NewFloat(user.Seconds())}
		vec[2] = BigFloat{big.NewFloat(sys.Seconds())}
		return NewVector(vec)
	case "date":
		return newCharVector(time.Now().In(conf.Location()).Format(time.UnixDate))
	case "format":
		return newCharVector(fmt.Sprintf("%q", conf.Format()))
	case "ibase":
		return Int(conf.InputBase())
	case "maxbits":
		return Int(conf.MaxBits())
	case "maxdigits":
		return Int(conf.MaxDigits())
	case "maxstack":
		return Int(conf.MaxStack())
	case "obase":
		return Int(conf.OutputBase())
	case "origin":
		return Int(conf.Origin())
	case "prompt":
		return newCharVector(fmt.Sprintf("%q", conf.Prompt()))
	case "sec", "now":
		return BigFloat{big.NewFloat(float64(time.Now().UnixNano()) / 1e9)}
	case "time":
		return timeVec(time.Now().In(conf.Location()))
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
func encodeTime(c Context, u, v Vector) Value {
	r := rune(u[0].(Char))
	if r != 't' && r != 'T' {
		Errorf("illegal left operand %s for encode", u)
	}
	// TODO: more than one value
	return timeVec(timeFromValue(c, v[0]))
}

// timeVec returns the time unpacked into year, month, day, hour, minute, second
// and time zone offset in seconds east of UTC.
func timeVec(date time.Time) Vector {
	y, m, d := date.Date()
	vec := make([]Value, 7)
	vec[0] = Int(y)
	vec[1] = Int(m)
	vec[2] = Int(d)
	vec[3] = Int(date.Hour())
	vec[4] = Int(date.Minute())
	sec := float64(date.Second()) + float64(date.Nanosecond())/1e9
	vec[5] = BigFloat{big.NewFloat(sec)}
	_, offset := date.Zone()
	vec[6] = Int(offset)
	return NewVector(vec)
}

// decodeTime returns a second value given a sys "time" vector.
func decodeTime(c Context, u, v Vector) Value {
	r := rune(u[0].(Char))
	if r != 't' && r != 'T' {
		Errorf("illegal left operand %s for decode", u)
	}
	year, month, day, hour, min := 0, 1, 1, 0, 0
	sec, nsec := int64(0), int64(0)
	now := time.Now()
	loc := c.Config().Location()
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
	case 7:
		offset := toInt(v[6])
		_, nowOffset := now.Zone()
		if offset != nowOffset {
			hour := offset / 3600
			min := (offset - (3600 * hour)) / 60
			loc = time.FixedZone(fmt.Sprintf("%d:%02d", hour, min), offset)
		}
		fallthrough
	case 6:
		switch s := v[5].(type) {
		default:
			Errorf("illegal right operand %s in decode", v)
		case Int:
			sec = int64(s)
		case BigInt:
			if !s.IsInt64() {
				Errorf("illegal right operand %s in decode", v)
			}
			sec = s.Int64()
		case BigRat:
			var f big.Float
			f.SetRat(s.Rat)
			sec, nsec = secNsec(&f)
		case BigFloat:
			sec, nsec = secNsec(s.Float)
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
	// time.Time values can only extract int64s for UnixNano, which limits the range too much.
	// So we use UnixMilli, which spans a big enough range, and add the nanoseconds manually.
	t := time.Date(year, time.Month(month), day, hour, min, int(sec), 0, loc)
	t = t.In(c.Config().LocationAt(t))
	var s, tmp big.Float
	s.SetInt64(t.UnixMilli())
	s.Mul(&s, tmp.SetInt64(1e6))
	s.Add(&s, tmp.SetInt64(nsec))
	s.Quo(&s, tmp.SetInt64(1e9))
	return BigFloat{&s}
}

// secNsec converts a seconds value into whole seconds and nanoseconds.
func secNsec(fs *big.Float) (sec, nsec int64) {
	var s big.Int
	fs.Int(&s)
	fs.Sub(fs, big.NewFloat(0).SetInt(&s))
	fs.Mul(fs, big.NewFloat(1e9))
	fs.Add(fs, big.NewFloat(0.5))
	ns, _ := fs.Int64()
	return s.Int64(), ns
}

// timeFromValue converts a seconds value into a time.Time, for
// the 'text' operator.
func timeFromValue(c Context, v Value) time.Time {
	conf := c.Config()
	var fs big.Float
	fs.Set(v.toType("encode", conf, bigFloatType).(BigFloat).Float)
	t := time.Unix(secNsec(&fs))
	t = t.In(conf.LocationAt(t))
	return t
}
