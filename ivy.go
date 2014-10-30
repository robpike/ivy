// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"code.google.com/p/rspace/ivy/config"
	"code.google.com/p/rspace/ivy/lex"
	"code.google.com/p/rspace/ivy/parse"
	"code.google.com/p/rspace/ivy/value"
)

var (
	format     = flag.String("format", "%v", "format string")
	printParse = flag.Bool("printparse", false, "print parse tree")
)

func init() {
	flag.Var(&iFlag, "I", "include directory; can be set multiple times")
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("ivy: ")

	flag.Usage = usage
	flag.Parse()

	conf := new(config.Config)
	conf.SetFormat(*format)
	conf.SetDebug("printParse", *printParse)

	value.SetConfig(conf)

	name := ""
	fd := os.Stdin
	var err error
	switch flag.NArg() {
	case 0:
	case 1:
		name = flag.Arg(0)
		fd, err = os.Open(name)
		if err != nil {
			log.Fatalf("ivy: %s\n", err)
		}
	default:
		flag.Usage()
	}

	lexer := lex.NewLexer(name, fd, []string(iFlag))
	parser := parse.NewParser(conf, lexer)
	for {
		run(parser)
	}
}

func run(p *parse.Parser) {
	defer func() {
		err := recover()
		if err == nil {
			return
		}
		if err, ok := err.(value.Error); ok {
			log.Print(err)
			return
		}
		panic(err)
	}()
	for {
		fmt.Print("_\t")
		value, ok := p.Line()
		if value != nil {
			fmt.Println(value)
		}
		if !ok {
			os.Exit(0)
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
	fmt.Fprintf(os.Stderr, "usage: asm [options] file.s\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
	os.Exit(2)
}
