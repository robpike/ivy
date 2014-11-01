// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"code.google.com/p/rspace/ivy/config"
	"code.google.com/p/rspace/ivy/lex"
	"code.google.com/p/rspace/ivy/parse"
	"code.google.com/p/rspace/ivy/value"
)

var (
	execute = flag.Bool("e", false, "execute arguments as a single expression")
	format  = flag.String("format", "%v", "format string for printing numbers")
	origin  = flag.Int("origin", 1, "index origin")
	prompt  = flag.String("prompt", "", "command prompt")
)

func init() {
	flag.Var(&iFlag, "I", "include directory; can be set multiple times")
}

var conf config.Config

func main() {
	log.SetFlags(0)
	log.SetPrefix("ivy: ")

	flag.Usage = usage
	flag.Parse()

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
				fd = os.Stdin
				interactive = true
			} else {
				fd, err = os.Open(name)
			}
			if err != nil {
				log.Fatalf("ivy: %s\n", err)
			}
			lexer := lex.NewLexer(name, fd, []string(iFlag))
			parser := parse.NewParser(&conf, lexer)
			if !run(parser, interactive) {
				break
			}
		}
		return
	}

	lexer := lex.NewLexer("", os.Stdin, []string(iFlag))
	parser := parse.NewParser(&conf, lexer)
	for !run(parser, true) {
	}
}

func runArgs() {
	lexer := lex.NewLexer("", strings.NewReader(strings.Join(flag.Args(), " ")), []string(iFlag))
	parser := parse.NewParser(&conf, lexer)
	run(parser, false)

}

// run runs until EOF or error. The return value says whether we completed without error.
func run(p *parse.Parser, interactive bool) (success bool) {
	defer func() {
		err := recover()
		if err == nil {
			return
		}
		if err, ok := err.(value.Error); ok {
			log.Print(err)
			success = false
			return
		}
		panic(err)
	}()
	for {
		if interactive {
			fmt.Print(conf.Prompt())
		}
		value, ok := p.Line()
		if value != nil {
			if conf.Debug("type") {
				fmt.Printf("%T:\n", value)
			}
			fmt.Println(value)
		}
		if !ok {
			return true
		}
		if interactive {
			fmt.Println()
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
