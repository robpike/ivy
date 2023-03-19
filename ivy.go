// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main // import "robpike.io/ivy"

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"robpike.io/ivy/config"
	"robpike.io/ivy/exec"
	"robpike.io/ivy/parse"
	"robpike.io/ivy/run"
	"robpike.io/ivy/scan"
	"robpike.io/ivy/value"
)

var (
	execute         = flag.String("e", "", "execute `argument` and quit")
	executeContinue = flag.String("i", "", "execute `argument` and continue")
	file            = flag.String("f", "", "execute `file` before input")
	format          = flag.String("format", "", "use `fmt` as format for printing numbers; empty sets default format")
	gformat         = flag.Bool("g", false, `shorthand for -format="%.12g"`)
	maxbits         = flag.Uint("maxbits", 1e9, "maximum size of an integer, in bits; 0 means no limit")
	maxdigits       = flag.Uint("maxdigits", 1e4, "above this many `digits`, integers print as floating point; 0 disables")
	maxstack        = flag.Uint("stack", 100000, "maximum call stack `depth` allowed")
	origin          = flag.Int("origin", 1, "set index origin to `n` (must be >=0)")
	prompt          = flag.String("prompt", "", "command `prompt`")
	debugFlag       = flag.String("debug", "", "comma-separated `names` of debug settings to enable")
)

// isTTY reports if fd is an interactive tty.
// It is replaced by system-specific files, like tty_unix.go.
var isTTY = func(fd uintptr) bool { return true }

var (
	conf    config.Config
	context value.Context
)

func main() {
	flag.Usage = usage
	flag.Parse()

	if *origin < 0 {
		fmt.Fprintf(os.Stderr, "ivy: illegal origin value %d\n", *origin)
		os.Exit(2)
	}

	if *gformat {
		*format = "%.12g"
	}

	conf.SetFormat(*format)
	conf.SetMaxBits(*maxbits)
	conf.SetMaxDigits(*maxdigits)
	conf.SetMaxStack(*maxstack)
	conf.SetOrigin(*origin)
	conf.SetPrompt(*prompt)

	if len(*debugFlag) > 0 {
		for _, debug := range strings.Split(*debugFlag, ",") {
			if !conf.SetDebug(debug, true) {
				fmt.Fprintf(os.Stderr, "ivy: unknown debug flag %q\n", debug)
				os.Exit(2)
			}
		}
	}

	context = exec.NewContext(&conf)

	if *file != "" {
		if !runFile(context, *file) {
			os.Exit(1)
		}
	}

	if *executeContinue != "" {
		if !runString(context, *executeContinue) {
			os.Exit(1)
		}
	}

	if *execute != "" {
		if !runString(context, *execute) {
			os.Exit(1)
		}
		return
	}

	if flag.NArg() > 0 {
		for i := 0; i < flag.NArg(); i++ {
			if !runFile(context, flag.Arg(i)) {
				os.Exit(1)
			}
		}
		return
	}

	scanner := scan.New(context, "<stdin>", bufio.NewReader(os.Stdin))
	parser := parse.NewParser("<stdin>", scanner, context)
	for !run.Run(parser, context, isTTY(os.Stdin.Fd())) {
	}
}

// runFile executes the contents of the file as an ivy program.
func runFile(context value.Context, file string) bool {
	var err error
	f := os.Stdin
	if file != "-" {
		f, err = os.Open(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ivy: %s\n", err)
			os.Exit(1)
		}
	}
	scanner := scan.New(context, file, bufio.NewReader(f))
	parser := parse.NewParser(file, scanner, context)
	return run.Run(parser, context, isTTY(f.Fd()))
}

// runString executes the string, typically a command-line argument, as an ivy program.
func runString(context value.Context, str string) bool {
	scanner := scan.New(context, "<args>", strings.NewReader(str))
	parser := parse.NewParser("<args>", scanner, context)
	return run.Run(parser, context, false)
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: ivy [options] [file ...]\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
	os.Exit(2)
}
