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
	"robpike.io/ivy/parse"
	"robpike.io/ivy/scan"
	"robpike.io/ivy/value"
)

var (
	execute   = flag.Bool("e", false, "execute arguments as a single expression")
	format    = flag.String("format", "", "use `fmt` as format for printing numbers; empty sets default format")
	gformat   = flag.Bool("g", false, `shorthand for -format="%.12g"`)
	origin    = flag.Int("origin", 1, "set index origin to `n` (must be 0 or 1)")
	prompt    = flag.String("prompt", "", "command `prompt`")
	debugFlag = flag.String("debug", "", "comma-separated `names` of debug settings to enable")
)

var conf config.Config

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

	context := parse.NewContext()

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
			scanner := scan.New(&conf, name, bufio.NewReader(fd))
			parser := parse.NewParser(&conf, name, scanner, context)
			if !run(parser, context, interactive) {
				break
			}
		}
		return
	}

	scanner := scan.New(&conf, "<stdin>", bufio.NewReader(os.Stdin))
	parser := parse.NewParser(&conf, "<stdin>", scanner, context)
	for !run(parser, context, true) {
	}
}

func runArgs(context value.Context) {
	scanner := scan.New(&conf, "<args>", strings.NewReader(strings.Join(flag.Args(), " ")))
	parser := parse.NewParser(&conf, "<args>", scanner, context)
	run(parser, context, false)
}

// run runs until EOF or error. The return value says whether we completed without error.
func run(p *parse.Parser, context value.Context, interactive bool) (success bool) {
	writer := conf.Output()
	defer func() {
		if conf.Debug("panic") {
			return
		}
		err := recover()
		if err == nil {
			return
		}
		p.FlushToNewline()
		if err, ok := err.(value.Error); ok {
			fmt.Fprintf(os.Stderr, "%s: %s\n", p.Loc(), err)
			if interactive {
				fmt.Fprintln(writer)
			}
			success = false
			return
		}
		panic(err)
	}()
	for {
		if interactive {
			fmt.Fprint(writer, conf.Prompt())
		}
		exprs, ok := p.Line()
		var values []value.Value
		if exprs != nil {
			values = context.Eval(exprs)
		}
		if values != nil {
			if conf.Debug("types") {
				for i, v := range values {
					if i > 0 {
						fmt.Fprint(writer, ",")
					}
					fmt.Fprintf(writer, "%T", v)
				}
				fmt.Fprintln(writer)
			}
			for i, v := range values {
				s := v.String()
				if i > 0 && len(s) > 0 && s[len(s)-1] != '\n' {
					fmt.Fprint(writer, " ")
				}
				fmt.Fprint(writer, v)
			}
			fmt.Fprintln(writer)
			context.Assign("_", values[len(values)-1])
		}
		if !ok {
			return true
		}
		if interactive {
			fmt.Fprintln(writer)
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: ivy [options] [file ...]\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
	os.Exit(2)
}
