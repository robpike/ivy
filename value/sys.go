// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"bufio"
	"fmt"
	"math"
	"math/big"
	"os"
	"time"

	"robpike.io/ivy/config"
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
"read" file: read the named file and return a vector of lines, with line termination stripped
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

func vecText(v *Vector) string {
	s := fmt.Sprint(v) // will print as "(text)"
	return s[1 : len(s)-1]
}

// sys implements the variegated "sys" unary operator.
func sys(c Context, v Value) Value {
	vv := v.(*Vector)
	conf := c.Config()

	if allChars(vv.All()) { // single argument
		verb := vecText(vv)
		if fn, ok := sys1[verb]; ok {
			return fn(conf)
		}
		if fn, ok := sysN[verb]; ok {
			return fn(conf, []Value{})
		}
		Errorf("sys %q not defined", verb)
	}

	if v1, ok := vv.At(0).(*Vector); ok && allChars(v1.All()) { // multiple arguments, verb first
		verb := vecText(v1)
		if fn, ok := sysN[verb]; ok {
			return fn(conf, vv.Slice(1, vv.Len()))
		}
		if _, ok := sys1[verb]; ok {
			Errorf("sys %q takes no arguments", verb)
		}
		Errorf("sys %q not defined", verb)
	}

	Errorf("sys requires string argument")
	panic("unreachable")
}

var sys1 = map[string]func(conf *config.Config) Value{
	"help": func(conf *config.Config) Value {
		fmt.Fprint(conf.Output(), sysHelp)
		return empty
	},
	"base": func(conf *config.Config) Value {
		return NewIntVector(conf.Base())
	},
	"cpu": func(conf *config.Config) Value {
		real, user, sys := conf.CPUTime()
		vec := make([]Value, 3)
		vec[0] = BigFloat{big.NewFloat(real.Seconds())}
		vec[1] = BigFloat{big.NewFloat(user.Seconds())}
		vec[2] = BigFloat{big.NewFloat(sys.Seconds())}
		return NewVector(vec)
	},
	"date": func(conf *config.Config) Value {
		return newCharVector(time.Now().In(conf.Location()).Format(time.UnixDate))
	},
	"format": func(conf *config.Config) Value {
		return newCharVector(fmt.Sprintf("%q", conf.Format()))
	},
	"ibase": func(conf *config.Config) Value {
		return Int(conf.InputBase())
	},
	"maxbits": func(conf *config.Config) Value {
		return Int(conf.MaxBits())
	},
	"maxdigits": func(conf *config.Config) Value {
		return Int(conf.MaxDigits())
	},
	"maxstack": func(conf *config.Config) Value {
		return Int(conf.MaxStack())
	},
	"now": func(conf *config.Config) Value {
		return BigFloat{big.NewFloat(float64(time.Now().UnixNano()) / 1e9)}
	},
	"obase": func(conf *config.Config) Value {
		return Int(conf.OutputBase())
	},
	"origin": func(conf *config.Config) Value {
		return Int(conf.Origin())
	},
	"prompt": func(conf *config.Config) Value {
		return newCharVector(fmt.Sprintf("%q", conf.Prompt()))
	},
	"sec": func(conf *config.Config) Value {
		return BigFloat{big.NewFloat(float64(time.Now().UnixNano()) / 1e9)}
	},
	"time": func(conf *config.Config) Value {
		return timeVec(time.Now().In(conf.Location()))
	},
}

var sysN = map[string]func(*config.Config, []Value) Value{
	"read": sysRead,
}

func sysRead(conf *config.Config, args []Value) Value {
	usage := func() {
		Errorf(`usage: sys "read" "filename"`)
	}

	if len(args) != 1 {
		usage()
	}
	v, ok := args[0].(*Vector)
	if !ok || !allChars(v.All()) {
		usage()
	}
	file := vecText(v)

	f, err := os.Open(file)
	if err != nil {
		Errorf("%v", err)
	}
	defer f.Close()

	var out []Value
	s := bufio.NewScanner(f)
	s.Buffer(nil, math.MaxInt)
	for s.Scan() {
		out = append(out, newCharVector(s.Text()))
	}
	if err := s.Err(); err != nil {
		Errorf("%v", err)
	}
	return NewVector(out)
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
func encodeTime(c Context, u, v *Vector) Value {
	r := rune(u.At(0).(Char))
	if r != 't' && r != 'T' {
		Errorf("illegal left operand %s for encode", u)
	}
	// TODO: more than one value
	return timeVec(timeFromValue(c, v.At(0)))
}

// timeVec returns the time unpacked into year, month, day, hour, minute, second
// and time zone offset in seconds east of UTC.
func timeVec(date time.Time) *Vector {
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
func decodeTime(c Context, u, v *Vector) Value {
	r := rune(u.At(0).(Char))
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
	switch v.Len() {
	default:
		Errorf("invalid time vector %s", v)
	case 7:
		offset := toInt(v.At(6))
		_, nowOffset := now.Zone()
		if offset != nowOffset {
			hour := offset / 3600
			min := (offset - (3600 * hour)) / 60
			loc = time.FixedZone(fmt.Sprintf("%d:%02d", hour, min), offset)
		}
		fallthrough
	case 6:
		switch s := v.At(5).(type) {
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
		min = toInt(v.At(4))
		fallthrough
	case 4:
		hour = toInt(v.At(3))
		fallthrough
	case 3:
		day = toInt(v.At(2))
		fallthrough
	case 2:
		month = toInt(v.At(1))
		fallthrough
	case 1:
		year = toInt(v.At(0))
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
