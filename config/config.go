// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config // import "robpike.io/ivy/config"

import (
	"fmt"
	"io"
	"math/big"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

// Order here determines order in the Config.debug array.
var DebugFlags = [...]string{
	"cpu",
	"panic",
	"parse",
	"tokens",
	"types",
}

// A Config holds information about the configuration of the system.
// The zero value of a Config represents the default values for all settings.
type Config struct {
	prompt      string
	output      io.Writer
	errOutput   io.Writer
	format      string
	ratFormat   string
	formatVerb  byte // The verb if format is floating-point.
	formatPrec  int  // The precision if format is floating-point.
	formatFloat bool // Whether format is floating-point.
	origin      int
	bigOrigin   *big.Int
	seed        int64
	debug       [len(DebugFlags)]bool
	source      rand.Source
	random      *rand.Rand
	maxBits     uint          // Maximum length of an integer; 0 means no limit.
	maxDigits   uint          // Above this size, ints print in floating format.
	maxStack    uint          // Maximum call stack depth.
	floatPrec   uint          // Length of mantissa of a BigFloat.
	realTime    time.Duration // Elapsed time of last interactive command.
	userTime    time.Duration // User time of last interactive command.
	sysTime     time.Duration // System time of last interactive command.
	// Bases: 0 means C-like, base 10 with 07 for octal and 0xa for hex.
	inputBase  int
	outputBase int
	mobile     bool // Running on a mobile platform.
}

func (c *Config) init() {
	if c.output == nil {
		c.output = os.Stdout
		c.errOutput = os.Stderr
		c.origin = 1
		c.seed = time.Now().UnixNano()
		c.bigOrigin = big.NewInt(1)
		c.source = rand.NewSource(c.seed)
		c.random = rand.New(c.source)
		c.maxBits = 1e6
		c.maxDigits = 1e4
		c.maxStack = 1e5
		c.floatPrec = 256
		c.mobile = false
	}
}

// Output returns the writer to be used for program output.
func (c *Config) Output() io.Writer {
	c.init()
	return c.output
}

// SetOutput sets the writer to which program output is printed; default is os.Stdout.
func (c *Config) SetOutput(output io.Writer) {
	c.init()
	c.output = output
}

// ErrOutput returns the writer to be used for error output.
func (c *Config) ErrOutput() io.Writer {
	c.init()
	return c.errOutput
}

// SetErrOutput sets the writer to which error output is printed; default is os.Stderr.
func (c *Config) SetErrOutput(output io.Writer) {
	c.init()
	c.errOutput = output
}

// Format returns the formatting string. If empty, the default
// formatting is used, as defined by the bases.
func (c *Config) Format() string {
	return c.format
}

// Format returns the formatting string for rationals.
func (c *Config) RatFormat() string {
	return c.ratFormat
}

// SetFormat sets the formatting string. Rational formatting
// is just this format applied twice with a / in between.
func (c *Config) SetFormat(s string) {
	c.init()
	c.formatVerb = 0
	c.formatPrec = 0
	c.formatFloat = false
	c.format = s
	if s == "" {
		c.ratFormat = "%v/%v"
		return
	}
	c.ratFormat = s + "/" + s
	// Is it a floating-point format?
	switch s[len(s)-1] {
	case 'f', 'F', 'g', 'G', 'e', 'E':
		// Yes
	default:
		return
	}
	c.formatFloat = true
	c.formatVerb = s[len(s)-1]
	c.formatPrec = 6 // The default
	point := strings.LastIndex(s, ".")
	if point > 0 {
		prec, err := strconv.ParseInt(s[point+1:len(s)-1], 10, 32)
		if err == nil && prec >= 0 {
			c.formatPrec = int(prec)
		}
	}
}

// FloatFormat returns the parsed information about the format,
// if it's a floating-point format.
func (c *Config) FloatFormat() (verb byte, prec int, ok bool) {
	return c.formatVerb, c.formatPrec, c.formatFloat
}

// Debug returns the value of the specified boolean debugging flag.
func (c *Config) Debug(flag string) bool {
	for i, f := range DebugFlags {
		if f == flag {
			return c.debug[i]
		}
	}
	return false
}

// SetDebug sets the value of the specified boolean debugging flag.
// It returns false if the flag is unknown.
func (c *Config) SetDebug(flag string, state bool) bool {
	c.init()
	for i, f := range DebugFlags {
		if f == flag {
			c.debug[i] = state
			return true
		}
	}
	return false
}

// Origin returns the index origin, default 1.
func (c *Config) Origin() int {
	return c.origin
}

// BigOrigin returns the index origin as a *big.Int.
func (c *Config) BigOrigin() *big.Int {
	return c.bigOrigin
}

// SetOrigin sets the index origin.
func (c *Config) SetOrigin(origin int) {
	c.init()
	c.origin = origin
	c.bigOrigin = big.NewInt(int64(origin))
}

// Prompt returns the interactive prompt.
func (c *Config) Prompt() string {
	return c.prompt
}

// SetPrompt sets the interactive prompt.
func (c *Config) SetPrompt(prompt string) {
	c.init()
	c.prompt = prompt
}

// Random returns the generator for random numbers.
func (c *Config) Random() *rand.Rand {
	c.init()
	return c.random
}

// RandomSeed returns the seed used to initialize the random number generator.
func (c *Config) RandomSeed() int64 {
	return c.seed
}

// SetRandomSeed sets the seed for the random number generator.
func (c *Config) SetRandomSeed(seed int64) {
	c.init()
	c.seed = seed
	c.source.Seed(seed)
}

// MaxBits returns the maximum integer size to store, in bits.
func (c *Config) MaxBits() uint {
	c.init()
	return c.maxBits
}

// MaxBits sets the maximum integer size to store, in bits.
func (c *Config) SetMaxBits(digits uint) {
	c.init()
	c.maxBits = digits
}

// MaxDigits returns the maximum integer size to print as integer, in digits.
func (c *Config) MaxDigits() uint {
	c.init()
	return c.maxDigits
}

// SetMaxDigits sets the maximum integer size to print as integer, in digits.
func (c *Config) SetMaxDigits(digits uint) {
	c.init()
	c.maxDigits = digits
}

// MaxStack returns the maximum call stack depth.
func (c *Config) MaxStack() uint {
	c.init()
	return c.maxStack
}

// SetMaxStack sets the maximum call stack depth.
func (c *Config) SetMaxStack(depth uint) {
	c.init()
	c.maxStack = depth
}

// FloatPrec returns the floating-point precision in bits.
// The exponent size is fixed by math/big.
func (c *Config) FloatPrec() uint {
	c.init()
	return c.floatPrec
}

// SetFloatPrec sets the floating-point precision in bits.
func (c *Config) SetFloatPrec(prec uint) {
	c.init()
	if prec == 0 {
		panic("zero float precision")
	}
	c.floatPrec = prec
}

// CPUTime returns the duration of the last interactive operation.
func (c *Config) CPUTime() (real, user, sys time.Duration) {
	c.init()
	return c.realTime, c.userTime, c.sysTime
}

// SetCPUTime sets the duration of the last interactive operation.
func (c *Config) SetCPUTime(real, user, sys time.Duration) {
	c.init()
	c.realTime = real
	c.userTime = user
	c.sysTime = sys
}

// PrintCPUTime returns a nicely formatted version of the CPU time.
func (c *Config) PrintCPUTime() string {
	if c.userTime == 0 && c.sysTime == 0 {
		return printDuration(c.realTime)
	}
	return fmt.Sprintf("%s (%s user, %s sys)", printDuration(c.realTime), printDuration(c.userTime), printDuration(c.sysTime))
}

// printDuration returns a nice formatting of the duration d,
// with 3 decimal places in whatever unit best fits, but
// if all the decimals are zero, drop them.
// The Duration.String method never rounds and is too noisy.
func printDuration(d time.Duration) string {
	switch {
	case d > time.Minute:
		m := int(d.Minutes())
		s := int(d.Seconds()) - 60*m
		return fmt.Sprintf("%dm%02ds", m, s)
	case d > time.Second:
		return formatDuration(d.Seconds(), "s")
	case d > time.Millisecond:
		return formatDuration(float64(d.Nanoseconds())/1e6, "ms")
	default:
		return formatDuration(float64(d.Nanoseconds())/1e3, "µs")
	}
}

// formatDuration returns a neatly formatted duration, omitting
// an all-zero decimal sequence, which is common for small values.
func formatDuration(d float64, units string) string {
	s := fmt.Sprintf("%.3f", d)
	if strings.HasSuffix(s, ".000") {
		s = s[:len(s)-4]
	}
	return s + units
	
}

// Base returns the input and output bases.
func (c *Config) Base() (inputBase, outputBase int) {
	return c.inputBase, c.outputBase
}

// InputBase returns the input base.
func (c *Config) InputBase() int {
	return c.inputBase
}

// OutputBase returns the output base.
func (c *Config) OutputBase() int {
	return c.outputBase
}

// SetBase sets the input and output bases.
func (c *Config) SetBase(inputBase, outputBase int) {
	c.init()
	c.inputBase = inputBase
	c.outputBase = outputBase
}

// Mobile reports whether we are running on a mobile platform.
func (c *Config) Mobile() bool {
	return c.mobile
}

// SetMobile sets the Mobile bit as specified.
func (c *Config) SetMobile(mobile bool) {
	c.init()
	c.mobile = mobile
}
