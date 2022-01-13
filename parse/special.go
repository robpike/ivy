// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run helpgen.go

package parse // import "robpike.io/ivy/parse"

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"robpike.io/ivy/config"
	"robpike.io/ivy/demo"
	"robpike.io/ivy/exec"
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
	// Save the base and do everything here base 0, which is decimal but
	// allows hex and octal in C syntax: 0xFF, 072.
	// The base command will set the values of the variables ibase and obase.
	ibase, obase := conf.Base()
	defer func() {
		conf.SetBase(ibase, obase)
	}()
	conf.SetBase(0, 0)
Switch:
	// Permit scan.Number in case we are in a high base (say 52) in which
	// case text looks numeric.
	switch text := p.need(scan.Identifier, scan.Number, scan.Op).Text; text {
	case "help":
		p.Println("")
		tok := p.peek()
		if tok.Type == scan.EOF {
			p.helpOverview()
			break
		}
		str := strings.ToLower(strings.TrimSpace(tok.Text))
		switch str {
		case "help":
			p.helpOverview()
		case "intro":
			p.printHelpBlock("", "Unary operators")
		case "unary":
			p.printHelpBlock("Unary operators", "Binary operators")
		case "binary":
			p.printHelpBlock("Binary operators", "Operators and axis indicator")
		case "axis":
			p.printHelpBlock("Operators and axis indicator", "Type-converting operations")
		case "type", "types", "conversion", "conversions":
			p.printHelpBlock("Type-converting operations", "Pre-defined constants")
		case "constant", "constants":
			p.printHelpBlock("Pre-defined constants", "Character data")
		case "char", "character":
			p.printHelpBlock("Character data", "User-defined operators")
		case "op", "ops", "operator", "operators":
			p.printHelpBlock("User-defined operators", "Special commands")
		case "special":
			p.printHelpBlock("Special commands", "$$EOF$$")
		case "about":
			p.next()
			tok = p.next()
			if tok.Type == scan.EOF {
				p.helpOverview()
				break
			}
			p.helpAbout(tok.Text)
		default:
			p.help(str)
		}
		p.next()
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
	case "demo":
		p.need(scan.EOF)
		if conf.Mobile() {
			p.Printf("For a demo on mobile platforms, use the Demo button in the UI.\n")
			break
		}
		// Use a default configuration.
		var conf config.Config
		err := demo.Run(os.Stdin, DemoRunner(os.Stdin, conf.Output()), conf.Output())
		if err != nil {
			p.errorf("%v", err)
		}
		p.Println("Demo finished")
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
	case "maxstack":
		if p.peek().Type == scan.EOF {
			p.Printf("%d\n", conf.MaxStack())
			break Switch
		}
		max := p.nextDecimalNumber()
		conf.SetMaxStack(uint(max))
	case "op", "ops": // We keep forgetting whether it's a plural or not.
		if p.peek().Type == scan.EOF {
			var unary, binary []string
			for _, def := range p.context.Defs {
				if def.IsBinary {
					binary = append(binary, def.Name)
				} else {
					unary = append(unary, def.Name)
				}
			}
			sort.Strings(unary)
			sort.Strings(binary)
			if unary != nil {
				p.Println("\nUnary: \t")
				for _, s := range unary {
					p.Println("\t" + s)
				}
			}
			if binary != nil {
				p.Println("\nBinary: \t")
				for _, s := range binary {
					p.Println("\t" + s)
				}
			}
			break Switch
		}
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
		// Must restore ibase, obase for save.
		conf.SetBase(ibase, obase)
		if p.peek().Type == scan.EOF {
			save(p.context, defaultFile)
		} else {
			save(p.context, p.getString())
		}
	case "seed":
		if p.peek().Type == scan.EOF {
			p.Println(conf.RandomSeed())
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
	fd, err := os.Open(name)
	if err != nil {
		p.errorf("%s", err)
	}
	p.runFromReader(context, name, fd, true)
}

// runFromReader executes the contents of the io.Reader, identified by name.
func (p *Parser) runFromReader(context value.Context, name string, reader io.Reader, stopOnError bool) {
	runDepth++
	if runDepth > 10 {
		p.errorf("invocations of %q nested too deep", name)
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
	scanner := scan.New(context, name, bufio.NewReader(reader))
	parser := NewParser(name, scanner, p.context)
	for parser.runUntilError(name) != io.EOF {
		if stopOnError {
			break
		}
	}
}

func (p *Parser) runUntilError(name string) error {
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
	for {
		exprs, ok := p.Line()
		for _, expr := range exprs {
			val := expr.Eval(p.context)
			if val == nil {
				continue
			}
			if _, ok := val.(Assignment); ok {
				continue
			}
			p.context.AssignGlobal("_", val)
			fmt.Fprintf(p.context.Config().Output(), "%v\n", val.Sprint(p.context.Config()))
		}
		if !ok {
			return io.EOF
		}
	}
}

// A simple way to connect the user's input to the interpreter.
// Sending one byte at a time is slow but very easy, and
// it's just for a demo.
type demoIO chan byte

func (dio demoIO) Write(b []byte) (int, error) {
	for _, c := range b {
		dio <- c
	}
	return len(b), nil
}

func (dio demoIO) Read(b []byte) (int, error) {
	for i := range b {
		b[i] = <-dio
		if b[i] == '\n' {
			return i + 1, nil
		}
	}
	return len(b), nil
}

func DemoRunner(userInput io.Reader, userOutput io.Writer) io.Writer {
	conf := &config.Config{}
	conf.SetOutput(userOutput)
	conf.SetRandomSeed(0)
	context := exec.NewContext(conf)
	dio := demoIO(make(chan byte, 1000))
	parser := NewParser("demo", nil, context) // Only needed for error prints in runFromReader.
	go parser.runFromReader(context, "demo", dio, false)
	return dio
}
