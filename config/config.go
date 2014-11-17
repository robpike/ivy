// Copyright 2014 Rob Pike. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config // import "robpike.io/ivy/config"

import (
	"math/big"
	"math/rand"
	"time"
)

// A Config holds information about the configuration of the system.
// The zero value of a Config holds the default values for all settings.
type Config struct {
	prompt    string
	format    string
	ratFormat string
	origin    int
	bigOrigin *big.Int
	debug     map[string]bool
	source    rand.Source
	random    *rand.Rand
	// Bases: 0 means C-like, base 10 with 07 for octal and 0xa for hex.
	inputBase  int
	outputBase int
}

func (c *Config) init() {
	if c.random == nil {
		c.source = rand.NewSource(time.Now().Unix())
		c.random = rand.New(c.source)
	}
}

func (c *Config) Format() string {
	if c == nil {
		return ""
	}
	return c.format
}

func (c *Config) RatFormat() string {
	if c == nil {
		return "%v/%v"
	}
	return c.ratFormat
}

func (c *Config) SetFormat(s string) {
	c.format = s
	if s == "" {
		c.ratFormat = "%v/%v"
	} else {
		c.ratFormat = s + "/" + s
	}
}

func (c *Config) Debug(s string) bool {
	if c == nil {
		return false
	}
	return c.debug[s]
}

func (c *Config) SetDebug(s string, state bool) {
	if c.debug == nil {
		c.debug = make(map[string]bool)
	}
	c.debug[s] = state
}

func (c *Config) Origin() int {
	if c == nil {
		return 0
	}
	return c.origin
}

func (c *Config) BigOrigin() *big.Int {
	if c == nil {
		return big.NewInt(0)
	}
	return c.bigOrigin
}

func (c *Config) SetOrigin(origin int) {
	c.origin = origin
	c.bigOrigin = big.NewInt(int64(origin))
}

func (c *Config) Prompt() string {
	return c.prompt
}

func (c *Config) SetPrompt(prompt string) {
	c.prompt = prompt
}

func (c *Config) Random() *rand.Rand {
	c.init()
	return c.random
}

func (c *Config) RandomSeed(seed int64) {
	c.init()
	c.source.Seed(seed)
}

func (c *Config) Base() (int, int) {
	if c == nil {
		return 0, 0
	}
	return c.inputBase, c.outputBase
}

func (c *Config) InputBase() int {
	if c == nil {
		return 0
	}
	return c.inputBase
}

func (c *Config) OutputBase() int {
	if c == nil {
		return 0
	}
	return c.outputBase
}

func (c *Config) SetBase(inputBase, outputBase int) {
	c.inputBase = inputBase
	c.outputBase = outputBase
}
