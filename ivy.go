// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"text/scanner"

	"code.google.com/p/rspace/ivy/lex"
	"code.google.com/p/rspace/ivy/value"
)

var (
	outputFile = flag.String("o", "", "output file; default foo.6 for /a/b/c/foo.s on arm64 (unused TODO)")
	printOut   = flag.Bool("S", true, "print assembly and machine code") // TODO: set to false
	trimPath   = flag.String("trimpath", "", "remove prefix from recorded source file paths (unused TODO)")
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
	for {
		run(lexer)
	}
}

func run(r lex.TokenReader) {
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
		tok := r.Next()
		text := r.Text()
		fmt.Printf("%v %q\n", tok, r.Text())
		switch tok {
		case scanner.EOF:
			return
		case scanner.Int:
			v, ok := value.Set(text)
			for j := 0; ok && j < 8; j++ {
				fmt.Println(v)
				v = v.Div(v)
				fmt.Printf("%T: %s\n", v, v)
			}
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
