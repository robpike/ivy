// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The mobile package provides a very narrow interface to ivy,
// suitable for wrapping in a UI for mobile applications.
// It is designed to work well with the gomobile tool by exposing
// only primitive types.
package mobile

import (
	"bytes"
	"fmt"
	"strings"

	"robpike.io/ivy/config"
	"robpike.io/ivy/exec"
	"robpike.io/ivy/parse"
	"robpike.io/ivy/run"
	"robpike.io/ivy/scan"
	"robpike.io/ivy/value"
)

var (
	conf    config.Config
	context value.Context
)

func init() {
	Reset()
}

// Eval evaluates the input string and returns its output.
// If execution caused errors, they will be returned concatenated
// together in the error value returned.
func Eval(expr string) (result string, errors error) {
	if !strings.HasSuffix(expr, "\n") {
		expr += "\n"
	}
	reader := strings.NewReader(expr)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	conf.SetOutput(stdout)
	conf.SetErrOutput(stderr)

	scanner := scan.New(&conf, context, " ", reader)
	parser := parse.NewParser(&conf, " ", scanner, context)

	for !run.Run(parser, context, false) {
	}
	var err error
	if stderr.Len() > 0 {
		err = fmt.Errorf("%s", stderr)
	}
	return stdout.String(), err
}

// Reset clears all state to the initial value.
func Reset() {
	conf.SetFormat(format)
	conf.SetMaxBits(maxBits)
	conf.SetMaxDigits(maxDigits)
	conf.SetOrigin(origin)
	conf.SetPrompt(prompt)
	value.SetConfig(&conf)
	context = exec.NewContext()
	run.Init(&conf, context)
}

// default configuration parameters.
const (
	format    = ""
	maxBits   = 1e9
	maxDigits = 1e4
	origin    = 1
	prompt    = ""
)
