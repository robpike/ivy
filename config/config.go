// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config

import "math/big"

type Config struct {
	prompt    string
	format    string
	ratFormat string
	origin    int
	bigOrigin *big.Int
	debug     map[string]bool
}

func (c *Config) Format() string {
	if c.format == "" {
		return "%v"
	}
	return c.format
}

func (c *Config) RatFormat() string {
	if c.ratFormat == "" {
		return "%v/%v"
	}
	return c.ratFormat
}

func (c *Config) SetFormat(s string) {
	c.format = s
	c.ratFormat = s + "/" + s
}

func (c *Config) Debug(s string) bool {
	return c.debug[s]
}

func (c *Config) SetDebug(s string, state bool) {
	if c.debug == nil {
		c.debug = make(map[string]bool)
	}
	c.debug[s] = state
}

func (c *Config) Origin() int {
	return c.origin
}

func (c *Config) BigOrigin() *big.Int {
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
