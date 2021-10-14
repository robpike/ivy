// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package demo implements the I/O for running the )demo
// special command. The script for the demo is in demo.ivy
// in this directory. Its content is embedded in this source file.
package demo

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	_ "embed"
)

//go:embed demo.ivy
var demoText []byte

// Text returns the input text for the standard demo.
func Text() string {
	return string(demoText)
}

// Run runs the demo. The arguments are the user's input, a Writer used to deliver
// text to an ivy interpreter, and a Writer for the output. It assumes that ivy is
// writing to the same output. Ivy expressions are read from a file (maintained in
// demo.ivy but embedded in the package). When the user hits a blank line, the next
// line from the file is delivered to ivy. If the user's input line has text, that
// is delivered instead and the file does not advance.
// A nil userInput ignores the user and just runs the script.
func Run(userInput io.Reader, toIvy io.Writer, output io.Writer) error {
	text := demoText // Don't overwrite the global!
	var scan *bufio.Scanner
	if userInput != nil {
		scan = bufio.NewScanner(userInput)
	}
	nextLine := func() (line []byte) {
		nl := bytes.IndexByte(text, '\n')
		if nl < 0 { // EOF or incomplete line.
			return nil
		}
		line, text = text[:nl+1], text[nl+1:]
		return line
	}
	// Show first line, with instructions, before accepting user input.
	output.Write(nextLine())
	for userInput == nil || scan.Scan() {
		if userInput != nil && len(scan.Bytes()) > 0 {
			// User typed a non-empty line of text; send that.
			line := []byte(fmt.Sprintf("%s\n", scan.Bytes()))
			// "quit" terminates.
			if string(bytes.TrimSpace(line)) == "quit" {
				break
			}
			if _, err := toIvy.Write(line); err != nil {
				return err
			}
		} else {
			// User typed newline; send next line of file's text.
			line := nextLine()
			if line == nil {
				break
			}
			output.Write(line)
			if _, err := toIvy.Write(line); err != nil {
				return err
			}
		}
	}
	if scan == nil {
		return nil
	}
	return scan.Err()
}
