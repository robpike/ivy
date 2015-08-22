// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main // import "robpike.io/ivy"

import (
	"bufio"
	"flag"
	"fmt"
	"io"
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
	execute   = flag.Bool("e", false, "execute arguments as a single expression")
	format    = flag.String("format", "", "use `fmt` as format for printing numbers; empty sets default format")
	gformat   = flag.Bool("g", false, `shorthand for -format="%.12g"`)
	maxbits   = flag.Uint("maxbits", 1e9, "maximum size of an integer, in bits; 0 means no limit")
	maxdigits = flag.Uint("maxdigits", 1e4, "above this many `digits`, integers print as floating point; 0 disables")
	origin    = flag.Int("origin", 1, "set index origin to `n` (must be 0 or 1)")
	prompt    = flag.String("prompt", "", "command `prompt`")
	debugFlag = flag.String("debug", "", "comma-separated `names` of debug settings to enable")
)

var (
	conf    config.Config
	context value.Context
)

func main() {
	flag.Usage = usage
	flag.Parse()

	if *origin != 0 && *origin != 1 {
		fmt.Fprintf(os.Stderr, "ivy: illegal origin value %d\n", *origin)
		os.Exit(2)
	}

	if *gformat {
		*format = "%.12g"
	}

	conf.SetFormat(*format)
	conf.SetMaxBits(*maxbits)
	conf.SetMaxDigits(*maxdigits)
	conf.SetOrigin(*origin)
	conf.SetPrompt(*prompt)
	if len(*debugFlag) > 0 {
		for _, debug := range strings.Split(*debugFlag, ",") {
			if !conf.SetDebug(debug, true) {
				fmt.Fprintf(os.Stderr, "ivy: unknown debug flag %q", debug)
				os.Exit(2)
			}
		}
	}

	value.SetConfig(&conf)
	context = exec.NewContext()
	value.SetContext(context)

	run.SetConfig(&conf)

	if *execute {
		runArgs(context)
		return
	}

	if flag.NArg() > 0 {
		for i := 0; i < flag.NArg(); i++ {
			name := flag.Arg(i)
			var fd io.Reader
			var err error
			interactive := false
			if name == "-" {
				interactive = true
				fd = os.Stdin
			} else {
				interactive = false
				fd, err = os.Open(name)
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "ivy: %s\n", err)
				os.Exit(1)
			}
			scanner := scan.New(&conf, context, name, bufio.NewReader(fd))
			parser := parse.NewParser(&conf, name, scanner, context)
			if !run.Run(parser, context, interactive) {
				break
			}
		}
		return
	}

	scanner := scan.New(&conf, context, "<stdin>", bufio.NewReader(os.Stdin))
	parser := parse.NewParser(&conf, "<stdin>", scanner, context)
	for !run.Run(parser, context, true) {
	}
}

// runArgs executes the text of the command-line arguments as an ivy program.
func runArgs(context value.Context) {
	scanner := scan.New(&conf, context, "<args>", strings.NewReader(strings.Join(flag.Args(), " ")))
	parser := parse.NewParser(&conf, "<args>", scanner, context)
	run.Run(parser, context, false)
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: ivy [options] [file ...]\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
	os.Exit(2)
}
