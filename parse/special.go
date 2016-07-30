// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate sh -c "(echo '// echo GENERATED; DO NOT EDIT'; echo; echo 'package parse; const specialHelpMessage=`'; sed -n '/^.) [a-z]/,/^$DOLLAR/s/^	//p' ../doc.go; echo '`') | gofmt >help.go"

package parse // import "robpike.io/ivy/parse"

import (
	"bufio"
	"fmt"
	"os"

	"robpike.io/ivy/config"
	"robpike.io/ivy/scan"
	"robpike.io/ivy/value"
)

const defaultFile = "save.ivy"

func (p *Parser) need(want ...scan.Type) scan.Token {
	tok := p.next()
	for _, w := range want {
		if tok.Type == w {
			return tok
		}
	}
	// Make the output look nice; usually there is only one item.
	if len(want) == 1 {
		p.errorf("expected %s, got %s", want[0], tok)
	}
	p.errorf("expected %s, got %s", want, tok)
	panic("not reached")
}

// nextDecimalNumber returns the next number, which
// must be a non-negative int32 (even though the result is of type int).
func (p *Parser) nextDecimalNumber() int {
	n64 := p.nextDecimalNumber64()
	if n64 != int64(int32(n64)) {
		p.errorf("value too large: %v", n64)
	}
	return int(n64)
}

// nextDecimalNumber64 returns the next number, which
// must fit in a non-negative int64.
func (p *Parser) nextDecimalNumber64() int64 {
	conf := p.context.Config()
	ibase, obase := conf.Base()
	defer conf.SetBase(ibase, obase)
	conf.SetBase(10, obase)
	v, err := value.Parse(conf, p.need(scan.Number).Text)
	if err != nil {
		p.errorf("%s", err)
	}
	var n int64 = -1
	switch num := v.(type) {
	case value.Int:
		n = int64(num)
	case value.BigInt:
		// The Int64 method produces undefined results if
		// the value is too large, so we must check.
		if num.Int.Sign() < 0 || num.Int.Cmp(value.MaxBigInt63) > 0 {
			p.errorf("value out of range: %v", v)
		}
		n = num.Int64()
	}
	if n < 0 {
		p.errorf("value must be a positive integer: %v", v)
	}
	return n
}

func truth(x bool) int {
	if x {
		return 1
	}
	return 0
}

func (p *Parser) special() {
	p.need(scan.RightParen)
	conf := p.context.Config()
	// Save the base and do everything here base 10.
	// The base command will set the values of the variables ibase and obase.
	ibase, obase := conf.Base()
	defer func() {
		conf.SetBase(ibase, obase)
	}()
	conf.SetBase(10, 10)
Switch:
	switch text := p.need(scan.Identifier, scan.Op).Text; text {
	case "help":
		p.Println(specialHelpMessage)
		p.Println("More at: https://godoc.org/robpike.io/ivy")
	case "base", "ibase", "obase":
		if p.peek().Type == scan.EOF {
			p.Printf("ibase\t%d\n", ibase)
			p.Printf("obase\t%d\n", obase)
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
	case "cpu":
		p.Printf("%s\n", conf.PrintCPUTime())
	case "debug":
		if p.peek().Type == scan.EOF {
			for _, f := range config.DebugFlags {
				p.Printf("%s\t%d\n", f, truth(conf.Debug(f)))
			}
			break Switch
		}
		name := p.need(scan.Identifier).Text
		if p.peek().Type == scan.EOF {
			// Toggle the value
			if !conf.SetDebug(name, !conf.Debug(name)) {
				p.Println("no such debug flag:", name)
			}
			if conf.Debug(name) {
				p.Println("1")
			} else {
				p.Println("0")
			}
			break
		}
		number := p.nextDecimalNumber()
		if !conf.SetDebug(name, number != 0) {
			p.Println("no such debug flag:", name)
		}
	case "format":
		if p.peek().Type == scan.EOF {
			p.Printf("%q\n", conf.Format())
			break Switch
		}
		conf.SetFormat(p.getString())
	case "get":
		if p.peek().Type == scan.EOF {
			p.runFromFile(p.context, defaultFile)
		} else {
			p.runFromFile(p.context, p.getString())
		}
	case "maxbits":
		if p.peek().Type == scan.EOF {
			p.Printf("%d\n", conf.MaxBits())
			break Switch
		}
		max := p.nextDecimalNumber()
		conf.SetMaxBits(uint(max))
	case "maxdigits":
		if p.peek().Type == scan.EOF {
			p.Printf("%d\n", conf.MaxDigits())
			break Switch
		}
		max := p.nextDecimalNumber()
		conf.SetMaxDigits(uint(max))
	case "op":
		name := p.need(scan.Operator, scan.Identifier).Text
		fn := p.context.UnaryFn[name]
		found := false
		if fn != nil {
			p.Println(fn)
			found = true
		}
		fn = p.context.BinaryFn[name]
		if fn != nil {
			p.Println(fn)
			found = true
		}
		if !found {
			p.errorf("%q not defined", name)
		}
	case "origin":
		if p.peek().Type == scan.EOF {
			p.Println(conf.Origin())
			break Switch

		}
		origin := p.nextDecimalNumber()
		if origin != 0 && origin != 1 {
			p.errorf("illegal origin %d", origin)
		}
		conf.SetOrigin(origin)
	case "prec":
		if p.peek().Type == scan.EOF {
			p.Printf("%d\n", conf.FloatPrec())
			break Switch
		}
		prec := p.nextDecimalNumber()
		if prec == 0 || prec > 1e6 {
			p.errorf("illegal prec %d", prec) // TODO: make 0 be disable?
		}
		conf.SetFloatPrec(uint(prec))
	case "prompt":
		if p.peek().Type == scan.EOF {
			p.Printf("%q\n", conf.Format())
			break Switch
		}
		conf.SetPrompt(p.getString())
	case "save":
		// Must restore ibase, obase for safe.
		conf.SetBase(ibase, obase)
		if p.peek().Type == scan.EOF {
			save(p.context, defaultFile)
		} else {
			save(p.context, p.getString())
		}
	case "seed":
		if p.peek().Type == scan.EOF {
			p.Println(conf.Origin())
			break Switch
		}
		conf.SetRandomSeed(p.nextDecimalNumber64())
	default:
		p.errorf(")%s: not recognized", text)
	}
	// We set the configuration in the scanner here, before it retrieves
	// the following newline. That means that any number it scans
	// at the beginning of the next line will happen after the config
	// has been updated.
	conf.SetBase(ibase, obase)
	p.need(scan.EOF)
}

// getString returns the value of the string that must be next in the input.
func (p *Parser) getString() string {
	return value.ParseString(p.need(scan.String).Text)
}

var runDepth = 0

// runFromFile executes the contents of the named file.
func (p *Parser) runFromFile(context value.Context, name string) {
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
			fmt.Fprintf(p.context.Config().ErrOutput(), "%s%s\n", p.Loc(), err)
			return
		}
		panic(err)
	}()
	fd, err := os.Open(name)
	if err != nil {
		p.errorf("%s", err)
	}
	scanner := scan.New(context, name, bufio.NewReader(fd))
	parser := NewParser(name, scanner, p.context)
	out := p.context.Config().Output()
	for {
		exprs, ok := parser.Line()
		for _, expr := range exprs {
			val := expr.Eval(p.context)
			if val == nil {
				continue
			}
			if _, ok := val.(Assignment); ok {
				continue
			}
			fmt.Fprintf(out, "%v\n", val.Sprint(context.Config()))
		}
		if !ok {
			return
		}
	}
}
