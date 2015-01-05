// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse // import "robpike.io/ivy/parse"

import (
	"bufio"
	"fmt"
	"os"
	"strconv"

	"robpike.io/ivy/scan"
	"robpike.io/ivy/value"
)

var debugFlags = []string{
	"panic",
	"parse",
	"tokens",
	"types",
}

func (p *Parser) need(want ...scan.Type) scan.Token {
	tok := p.next()
	for _, w := range want {
		if tok.Type == w {
			return tok
		}
	}
	p.errorf("expected %s, got %s", want, tok)
	panic("not reached")
}

func (p *Parser) nextDecimalNumber() int {
	ibase, obase := p.config.Base()
	defer p.config.SetBase(ibase, obase)
	p.config.SetBase(10, obase)
	v, err := value.Parse(p.need(scan.Number).Text)
	if err != nil {
		p.errorf("%s", err)
	}
	num, ok := v.(value.Int)
	if !ok {
		p.errorf("value must be an integer: %v", v)
	}
	return int(num)
}

func truth(x bool) int {
	if x {
		return 1
	}
	return 0
}

func (p *Parser) special() {
	p.need(scan.RightParen)
Switch:
	switch text := p.need(scan.Identifier).Text; text {
	case "base", "ibase", "obase":
		ibase, obase := p.config.Base()
		if p.peek().Type == scan.Newline {
			fmt.Printf("ibase\t%d\n", ibase)
			fmt.Printf("obase\t%d\n", obase)
			break Switch
		}
		base := p.nextDecimalNumber()
		if base != 0 && (base < 2 || 36 < base) {
			p.errorf("illegal base %d", base)
		}
		switch text {
		case "base":
			ibase, obase = base, base
		case "ibase":
			ibase = base
		case "obase":
			obase = base
		}
		p.config.SetBase(ibase, obase)
	case "debug":
		if p.peek().Type == scan.Newline {
			for _, f := range debugFlags {
				fmt.Printf("%s\t%d\n", f, truth(p.config.Debug(f)))
			}
			break Switch
		}
		name := p.need(scan.Identifier).Text
		found := false
		for _, f := range debugFlags {
			if f == name {
				found = true
				break
			}
		}
		if !found {
			fmt.Println("no such debug flag:", name)
			break Switch
		}
		if p.peek().Type == scan.Newline {
			// Toggle the value
			p.config.SetDebug(name, !p.config.Debug(name))
			if p.config.Debug(name) {
				fmt.Println("1")
			} else {
				fmt.Println("0")
			}
			break
		}
		number := p.nextDecimalNumber()
		p.config.SetDebug(name, number != 0)
	case "format":
		if p.peek().Type == scan.Newline {
			fmt.Printf("%q\n", p.config.Format())
			break Switch
		}
		p.config.SetFormat(p.getString())
	case "get":
		p.runFromFile(p.getString())
	case "op":
		name := p.need(scan.Identifier).Text
		fn := p.context.unaryFn[name]
		if fn != nil {
			fmt.Println(fn)
		}
		fn = p.context.binaryFn[name]
		if fn != nil {
			fmt.Println(fn)
		}
	case "origin":
		if p.peek().Type == scan.Newline {
			fmt.Println(p.config.Origin())
			break Switch

		}
		origin := p.nextDecimalNumber()
		if origin != 0 && origin != 1 {
			p.errorf("illegal origin %d", origin)
		}
		p.config.SetOrigin(origin)
	case "prompt":
		if p.peek().Type == scan.Newline {
			fmt.Printf("%q\n", p.config.Format())
			break Switch
		}
		p.config.SetPrompt(p.getString())
	case "seed":
		if p.peek().Type == scan.Newline {
			fmt.Println(p.config.Origin())
			break Switch
		}
		p.config.RandomSeed(int64(p.nextDecimalNumber()))
	default:
		p.errorf(")%s: not recognized", text)
	}
	p.need(scan.Newline)
}

// getString returns the value of the string or raw string
// that must be next in the input.
func (p *Parser) getString() string {
	str := p.need(scan.String, scan.RawString).Text
	str, err := strconv.Unquote(str)
	if err != nil {
		p.errorf("%s", err)
	}
	return str
}

var runDepth = 0

// runFromFile executes the contents of the named file.
func (p *Parser) runFromFile(name string) {
	runDepth++
	if runDepth > 10 {
		p.errorf("get %q nested too deep", name)
	}
	defer func() {
		runDepth--
		err := recover()
		if err == nil {
			return
		}
		if err, ok := err.(value.Error); ok {
			fmt.Fprintf(os.Stderr, "%s: %s\n", p.Loc(), err)
			return
		}
		panic(err)
	}()
	fd, err := os.Open(name)
	if err != nil {
		p.errorf("%s", err)
	}
	scanner := scan.New(p.config, name, bufio.NewReader(fd))
	parser := NewParser(p.config, name, scanner, p.context)
	for {
		value, ok := parser.Line()
		if value != nil {
			fmt.Fprintln(os.Stdout, value)
		}
		if !ok {
			return
		}
	}
}
