// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The mobile package provides a very narrow interface to ivy,
// suitable for wrapping in a UI for mobile applications.
// It is designed to work well with the gomobile tool by exposing
// only primitive types. It's also handy for testing.
//
// TODO: This package (and ivy itself) has global state, so only
// one execution stream (Eval or Demo) can be active at a time.
package mobile

//go:generate sh -c "go run help_gen.go | gofmt >help.go"

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
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
// TODO: Should it stop at first error?
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

// Demo represents a running line-by-line demonstration.
type Demo struct {
	scanner *bufio.Scanner
}

// NewDemo returns a new Demo that will scan the input text line by line.
func NewDemo(input string) *Demo {
	// TODO: The state being reset should be local to the demo.
	// but that's not worth doing until ivy itself has no global state.
	Reset()
	return &Demo{
		scanner: bufio.NewScanner(strings.NewReader(input)),
	}
}

// Next returns the result (and error) produced by the next line of
// input. It returns ("", io.EOF) at EOF.
func (d *Demo) Next() (result string, err error) {
	if !d.scanner.Scan() {
		if err := d.scanner.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return Eval(d.scanner.Text())
}

// Reset clears all state to the initial value.
func Reset() {
	conf.SetFormat("")
	conf.SetMaxBits(1e9)
	conf.SetMaxDigits(1e4)
	conf.SetOrigin(1)
	conf.SetPrompt("")
	conf.SetBase(0, 0)
	conf.SetRandomSeed(0)
	value.SetConfig(&conf)
	context = exec.NewContext()
	value.SetContext(context)
	run.SetConfig(&conf)
}

// Help returns the help page formatted in HTML.
func Help() string {
	return help
}
