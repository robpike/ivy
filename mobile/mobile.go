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

//go:generate sh -c "go run help_gen.go >help.go"

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"robpike.io/ivy/config"
	"robpike.io/ivy/exec"
	"robpike.io/ivy/run"
	"robpike.io/ivy/value"
)

var (
	conf    config.Config
	context value.Context
)

func init() {
	Reset()
}

// On mobile platforms, the output gets turned into HTML.
// Some characters go wrong there (< and > are handled in
// Objective C or Java, but not all characters), tabs don't appear
// at all, and runs of spaces are collapsed. Also for some reason
// backslashes are trouble. Here is the hacky fix.
var escaper = strings.NewReplacer(" ", "\u00A0", "\t", "    ", "\\", "&#92;")

// Eval evaluates the input string and returns its output.
// The output is HTML-safe, suitable for mobile platforms.
func Eval(expr string) (string, error) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	conf.SetErrOutput(stderr)
	run.Ivy(context, expr, stdout, stderr)
	result := escaper.Replace(stdout.String())
	if stderr.Len() > 0 {
		return result, fmt.Errorf(stderr.String())
	}
	return result, nil
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
// input. It returns ("", io.EOF) at EOF. The output is escaped.
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
	conf.SetMobile(true)
	context = exec.NewContext(&conf)
}

// Help returns the help page formatted in HTML.
func Help() string {
	return help
}
