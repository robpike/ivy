// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config // import "robpike.io/ivy/config"

import (
	"io"
	"math/big"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Order here determines order in the Config.debug array.
var DebugFlags = [...]string{
	"panic",
	"parse",
	"tokens",
	"types",
}

// A Config holds information about the configuration of the system.
// The zero value of a Config represents the default values for all settings.
type Config struct {
	mu          sync.Mutex
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
	debug       [len(DebugFlags)]bool
	source      rand.Source
	random      *rand.Rand
	maxBits     uint // Maximum length of an integer; 0 means no limit.
	maxDigits   uint // Above this size, ints print in floating format.
	floatPrec   uint // Length of mantissa of a BigFloat.
	// Bases: 0 means C-like, base 10 with 07 for octal and 0xa for hex.
	inputBase  int
	outputBase int
}

func (c *Config) init() {
	if c.output == nil {
		c.output = os.Stdout
		c.errOutput = os.Stderr
		c.origin = 1
		c.source = rand.NewSource(time.Now().Unix())
		c.random = rand.New(c.source)
		c.maxBits = 1e6
		c.maxDigits = 1e4
		c.floatPrec = 256
	}
}

func (c *Config) sync() func() {
	c.mu.Lock()
	return c.mu.Unlock
}

// Output returns the writer to be used for program output.
func (c *Config) Output() io.Writer {
	defer c.sync()()
	c.init()
	return c.output
}

// SetOutput sets the writer to which program output is printed; default is os.Stdout.
func (c *Config) SetOutput(output io.Writer) {
	defer c.sync()()
	c.init()
	c.output = output
}

// ErrOutput returns the writer to be used for error output.
func (c *Config) ErrOutput() io.Writer {
	defer c.sync()()
	c.init()
	return c.errOutput
}

// SetErrOutput sets the writer to which error output is printed; default is os.Stderr.
func (c *Config) SetErrOutput(output io.Writer) {
	defer c.sync()()
	c.init()
	c.errOutput = output
}

// Format returns the formatting string. If empty, the default
// formatting is used, as defined by the bases.
func (c *Config) Format() string {
	defer c.sync()()
	return c.format
}

// Format returns the formatting string for rationals.
func (c *Config) RatFormat() string {
	defer c.sync()()
	return c.ratFormat
}

// SetFormat sets the formatting string. Rational formatting
// is just this format applied twice with a / in between.
func (c *Config) SetFormat(s string) {
	defer c.sync()()
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
	defer c.sync()()
	return c.formatVerb, c.formatPrec, c.formatFloat
}

// Debug returns the value of the specified boolean debugging flag.
func (c *Config) Debug(flag string) bool {
	defer c.sync()()
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
	defer c.sync()()
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
	defer c.sync()()
	return c.origin
}

// BigOrigin returns the index origin as a *big.Int.
func (c *Config) BigOrigin() *big.Int {
	defer c.sync()()
	return c.bigOrigin
}

// SetOrigin sets the index origin.
func (c *Config) SetOrigin(origin int) {
	defer c.sync()()
	c.init()
	c.origin = origin
	c.bigOrigin = big.NewInt(int64(origin))
}

// Prompt returns the interactive prompt.
func (c *Config) Prompt() string {
	defer c.sync()()
	return c.prompt
}

// SetPrompt sets the interactive prompt.
func (c *Config) SetPrompt(prompt string) {
	defer c.sync()()
	c.init()
	c.prompt = prompt
}

// Random returns the generator for random numbers.
func (c *Config) Random() *rand.Rand {
	defer c.sync()()
	c.init()
	return c.random
}

// SetRandomSeed sets the seed for the random number generator.
func (c *Config) SetRandomSeed(seed int64) {
	defer c.sync()()
	c.init()
	c.source.Seed(seed)
}

// MaxBits returns the maximum integer size to store, in bits.
func (c *Config) MaxBits() uint {
	defer c.sync()()
	c.init()
	return c.maxBits
}

// MaxBits sets the maximum integer size to store, in bits.
func (c *Config) SetMaxBits(digits uint) {
	defer c.sync()()
	c.init()
	c.maxBits = digits
}

// MaxDigits returns the maximum integer size to print as integer, in digits.
func (c *Config) MaxDigits() uint {
	defer c.sync()()
	c.init()
	return c.maxDigits
}

// SetMaxDigits sets the maximum integer size to print as integer, in digits.
func (c *Config) SetMaxDigits(digits uint) {
	defer c.sync()()
	c.init()
	c.maxDigits = digits
}

// FloatPrec returns the floating-point precision in bits.
// The exponent size is fixed by math/big.
func (c *Config) FloatPrec() uint {
	defer c.sync()()
	c.init()
	return c.floatPrec
}

// SetFloatPrec sets the floating-point precision in bits.
func (c *Config) SetFloatPrec(prec uint) {
	defer c.sync()()
	c.init()
	if prec == 0 {
		panic("zero float precision")
	}
	c.floatPrec = prec
}

// Base returns the input and output bases.
func (c *Config) Base() (inputBase, outputBase int) {
	defer c.sync()()
	return c.inputBase, c.outputBase
}

// InputBase returns the input base.
func (c *Config) InputBase() int {
	defer c.sync()()
	return c.inputBase
}

// OutputBase returns the output base.
func (c *Config) OutputBase() int {
	defer c.sync()()
	return c.outputBase
}

// SetBase sets the input and output bases.
func (c *Config) SetBase(inputBase, outputBase int) {
	defer c.sync()()
	c.init()
	c.inputBase = inputBase
	c.outputBase = outputBase
}
