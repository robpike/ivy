// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"code.google.com/p/rspace/ivy/lex"
	"code.google.com/p/rspace/ivy/parse"
	"code.google.com/p/rspace/ivy/value"
)

func init() {
	flag.Var(&iFlag, "I", "include directory; can be set multiple times")
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("ivy: ")

	flag.Usage = usage
	flag.Parse()
	//	if flag.NArg() != 1 {
	//		flag.Usage()
	//	}

	lexer := lex.NewLexer( /*flag.Arg(0)*/ "/dev/stdin", []string(iFlag))
	parser := parse.NewParser(lexer)
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
		expr, ok := p.Line()
		if expr != nil {
			fmt.Println(parse.Tree(expr))
			fmt.Println(expr.Eval())
		}
		if !ok {
			break
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
