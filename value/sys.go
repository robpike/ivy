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
"help":       print this text and return iota 0
"base":       the input and output base settings as a vector of two integers
"cpu":        the processor timing for the last evaluation
              as a vector in units of seconds:
                real user(cpu) system(cpu)
"date":       the current time in Unix date format
                year month day hour minute second
"format":     the output format setting
"ibase":      the input base (ibase) setting
"maxbits":    the maxbits setting
"maxdigits":  the maxdigits setting
"maxstack":   the maxstack setting
"obase":      the output base (obase) setting
"origin":     the index origin setting
"prec":       the bit length of the mantissa of float values
"prompt":     the prompt setting
"read" file:  read the named file and return a vector of lines, with line termination stripped
"sec":        the time in seconds since
                Jan 1 00:00:00 1970 UTC
"time":       the current time in the configured time zone as a vector; the last
              element is the time zone in which the other values apply:
                year month day hour minute second seconds-east-of-UTC
"trace" args: print a stack trace followed by the arguments

The following commands also have a binary form that sets the value
to the left argument and returns the previous value:

	"base"     "format" "ibase"  "maxbits" "maxdigits"
	"maxstack" "obase"  "origin" "prec" "prompt"


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

func sysInt(c Context, arg Value, op string) int {
	vv := arg.(*Vector) // We know it's a vector from unary.go.
	if vv.Len() != 1 {
		c.Errorf("left argument (%s) for sys %q must be integer", arg, op)
	}
	v, ok := vv.At(0).(Int)
	if !ok {
		c.Errorf("left argument (%s) for sys %q must be integer", v, op)
	}
	return int(v)
}

// for setting base.
func sys2Ints(c Context, arg Value, op string) (int, int, bool) {
	vv := arg.(*Vector) // We know it's a 2-vector.
	if vv.Len() != 2 {
		return 0, 0, false
	}
	v1, ok1 := vv.At(0).(Int)
	v2, ok2 := vv.At(1).(Int)
	if !ok1 || !ok2 {
		c.Errorf("left argument (%s) for sys %q must be integers", vv, op)
	}
	return int(v1), int(v2), true
}

func sysUint(c Context, arg Value, op string) uint {
	u := sysInt(c, arg, op)
	if u < 0 {
		c.Errorf("left argument (%s) for sys %q must be non-negative integer", u, op)
	}
	return uint(u)
}

// sys implements the variegated "sys" unary operator.
func sys(c Context, v Value) Value {
	vv := v.(*Vector)
	conf := c.Config()

	if vv.AllChars() { // single argument
		verb := vecText(vv)
		if fn, ok := sys1[verb]; ok {
			return fn(conf)
		}
		if fn, ok := sysN[verb]; ok {
			return fn(c, []Value{})
		}
		c.Errorf("sys %q not defined", verb)
	}

	if v1, ok := vv.At(0).(*Vector); ok && v1.AllChars() { // multiple arguments, verb first
		verb := vecText(v1)
		if fn, ok := sysN[verb]; ok {
			var args []Value
			for _, v := range vv.Slice(1, vv.Len()) {
				args = append(args, v)
			}
			return fn(c, args)
		}
		if _, ok := sys1[verb]; ok {
			c.Errorf("sys %q takes no arguments", verb)
		}
		c.Errorf("sys %q not defined", verb)
	}

	c.Errorf("sys requires string argument")
	panic("unreachable")
}

// binarySys implements the "sys" binary operator, which sets the value and returns
// the previous value
func binarySys(c Context, u, v Value) Value {
	// Remember the old value.
	ret := sys(c, v)

	vv := v.(*Vector)
	if !vv.AllChars() { // single argument
		c.Errorf("sys requires string argument")
	}
	verb := vecText(vv)
	if fn, ok := sys2[verb]; ok {
		fn(c, u)
		return ret
	}
	c.Errorf("binary sys %q not defined", verb)
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
		edit := newVectorEditor(0, nil)
		edit.Append(
			BigFloat{big.NewFloat(real.Seconds())},
			BigFloat{big.NewFloat(user.Seconds())},
			BigFloat{big.NewFloat(sys.Seconds())},
		)
		return edit.Publish()
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
	"prec": func(conf *config.Config) Value {
		return Int(conf.FloatPrec())
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

// These are the binary ones that set the value. It's a smaller group.
var sys2 = map[string]func(c Context, v Value){
	"base": func(c Context, v Value) {
		// Since it comes back as a pair, allow a pair here.
		ib, ob, ok := sys2Ints(c, v, "base")
		if !ok {
			ib = sysInt(c, v, "base")
			ob = ib
		}
		c.Config().SetBase(ib, ob)
	},
	"format": func(c Context, v Value) {
		vv := v.(*Vector)
		if !vv.AllChars() {
			c.Errorf("left argument of binary sys 'format' must be string")
		}
		c.Config().SetFormat(vecText(vv))
	},
	"ibase": func(c Context, v Value) {
		c.Config().SetBase(sysInt(c, v, "ibase"), c.Config().OutputBase())
	},
	"maxbits": func(c Context, v Value) {
		c.Config().SetMaxBits(sysUint(c, v, "maxbits"))
	},
	"maxdigits": func(c Context, v Value) {
		c.Config().SetMaxDigits(sysUint(c, v, "maxdigits"))
	},
	"maxstack": func(c Context, v Value) {
		c.Config().SetMaxStack(sysUint(c, v, "maxstack"))
	},
	"obase": func(c Context, v Value) {
		c.Config().SetBase(c.Config().InputBase(), sysInt(c, v, "obase"))
	},
	"origin": func(c Context, v Value) {
		c.Config().SetOrigin(sysInt(c, v, "origin"))
	},
	"prec": func(c Context, v Value) {
		c.Config().SetFloatPrec(sysUint(c, v, "prec"))
	},
	"prompt": func(c Context, v Value) {
		vv := v.(*Vector)
		if !vv.AllChars() {
			c.Errorf("left argument of binary sys 'prompt' must be string")
		}
		c.Config().SetPrompt(vecText(vv))
	},
}

var sysN = map[string]func(Context, []Value) Value{
	"read":  sysRead,
	"trace": sysTrace,
}

func sysRead(c Context, args []Value) Value {
	usage := func() {
		c.Errorf(`usage: sys "read" "filename"`)
	}

	if len(args) != 1 {
		usage()
	}
	v, ok := args[0].(*Vector)
	if !ok || !v.AllChars() {
		usage()
	}
	file := vecText(v)

	f, err := os.Open(file)
	if err != nil {
		c.Errorf("%v", err)
	}
	defer f.Close()

	edit := newVectorEditor(0, nil)
	s := bufio.NewScanner(f)
	s.Buffer(nil, math.MaxInt)
	for s.Scan() {
		edit.Append(newCharVector(s.Text()))
	}
	if err := s.Err(); err != nil {
		c.Errorf("%v", err)
	}
	return edit.Publish()
}

func sysTrace(c Context, args []Value) Value {
	c.StackTrace()
	if len(args) == 1 {
		return printValue(c, args[0])
	}
	return printValue(c, NewVector(args...))
}

// encodeTime returns a sys "time" vector given a seconds value.
// We know the first argument is all chars and not empty.
func encodeTime(c Context, u, v *Vector) Value {
	if v.Len() != 1 {
		c.Errorf("'T' encode takes a single right hand argument; got %s", v)
	}
	r := rune(u.At(0).(Char))
	if r != 't' && r != 'T' {
		c.Errorf("illegal left operand %s for encode", u)
	}
	return timeVec(timeFromValue(c, v.At(0)))
}

// timeVec returns the time unpacked into year, month, day, hour, minute, second
// and time zone offset in seconds east of UTC.
func timeVec(date time.Time) *Vector {
	y, m, d := date.Date()
	sec := float64(date.Second()) + float64(date.Nanosecond())/1e9
	_, offset := date.Zone()
	edit := newVectorEditor(0, nil)
	edit.Append(
		Int(y), Int(m), Int(d),
		Int(date.Hour()), Int(date.Minute()), BigFloat{big.NewFloat(sec)},
		Int(offset))
	return edit.Publish()
}

// decodeTime returns a second value given a sys "time" vector.
func decodeTime(c Context, u, v *Vector) Value {
	r := rune(u.At(0).(Char))
	if r != 't' && r != 'T' {
		c.Errorf("illegal left operand %s for decode", u)
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
			c.Errorf("illegal right operand %s in decode", v)
		}
		return int(b.Int64())
	}
	switch v.Len() {
	default:
		c.Errorf("invalid time vector %s", v)
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
			c.Errorf("illegal right operand %s in decode", v)
		case Int:
			sec = int64(s)
		case BigInt:
			if !s.IsInt64() {
				c.Errorf("illegal right operand %s in decode", v)
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
	fs.Set(v.toType("encode", c, bigFloatType).(BigFloat).Float)
	t := time.Unix(secNsec(&fs))
	t = t.In(conf.LocationAt(t))
	return t
}
