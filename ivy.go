// Copyright 2014 Rob Pike. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

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
	execute = flag.Bool("e", false, "execute arguments as a single expression")
	format  = flag.String("format", "", "format string for printing numbers; empty sets default format")
	origin  = flag.Int("origin", 1, "index origin (must be 0 or 1)")
	prompt  = flag.String("prompt", "", "command prompt")
)

func init() {
	flag.Var(&iFlag, "I", "include directory; can be set multiple times")
}

var conf config.Config

func main() {
	flag.Usage = usage
	flag.Parse()

	if *origin != 0 && *origin != 1 {
		fmt.Fprintf(os.Stderr, "ivy: illegal origin value %d", *origin)
		os.Exit(2)
	}

	conf.SetFormat(*format)
	conf.SetOrigin(*origin)
	conf.SetPrompt(*prompt)

	value.SetConfig(&conf)

	if *execute {
		runArgs()
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
				fmt.Fprintf(os.Stderr, "ivy: %s", err)
				os.Exit(1)
			}
			scanner := scan.New(&conf, name, bufio.NewReader(fd))
			parser := parse.NewParser(&conf, name, scanner)
			if !run(parser, os.Stdout, interactive) {
				break
			}
		}
		return
	}

	scanner := scan.New(&conf, "<stdin>", bufio.NewReader(os.Stdin))
	parser := parse.NewParser(&conf, "<stdin>", scanner)
	for !run(parser, os.Stdout, true) {
	}
}

func runArgs() {
	scanner := scan.New(&conf, "<args>", strings.NewReader(strings.Join(flag.Args(), " ")))
	parser := parse.NewParser(&conf, "<args>", scanner)
	run(parser, os.Stdout, false)
}

// run runs until EOF or error. The return value says whether we completed without error.
func run(p *parse.Parser, writer io.Writer, interactive bool) (success bool) {
	defer func() {
		if conf.Debug("panic") {
			return
		}
		err := recover()
		if err == nil {
			return
		}
		if err, ok := err.(value.Error); ok {
			fmt.Fprintf(os.Stderr, "%s: %s\n", p.Loc(), err)
			success = false
			return
		}
		panic(err)
	}()
	for {
		if interactive {
			fmt.Fprint(writer, conf.Prompt())
		}
		values, ok := p.Line()
		if values != nil {
			if conf.Debug("types") {
				for i, v := range values {
					if i > 0 {
						fmt.Fprint(writer, ",")
					}
					fmt.Fprintf(writer, "%T", v)
				}
			}
			for i, v := range values {
				s := v.String()
				if i > 0 && len(s) > 0 && s[len(s)-1] != '\n' {
					fmt.Fprint(writer, " ")
				}
				fmt.Fprint(writer, v)
			}
			fmt.Fprintln(writer)
		}
		if !ok {
			return true
		}
		if interactive {
			fmt.Fprintln(writer)
		}
	}
}

var (
	iFlag multiFlag
)

// multiFlag allows setting a value multiple times to collect a list, as in -I=dir1 -I=dir2.
type multiFlag []string

func (m *multiFlag) String() string {
	return fmt.Sprint(*m)
}

func (m *multiFlag) Set(val string) error {
	(*m) = append(*m, val)
	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: ivy [options] [file ...]\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
	os.Exit(2)
}
