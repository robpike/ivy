// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import (
	"strings"
)

func (p *Parser) helpOverview() {
	p.Println("Overview:")
	p.Println("\t)help intro")
	p.Println("Unary operators:")
	p.Println("\t)help unary")
	p.Println("Binary operators:")
	p.Println("\t)help binary")
	p.Println("Axis operators:")
	p.Println("\t)help axis")
	p.Println("Types and conversions:")
	p.Println("\t)help types")
	p.Println("Constants:")
	p.Println("\t)help constants")
	p.Println("Characters:")
	p.Println("\t)help char")
	p.Println("User-defined ops:")
	p.Println("\t)help ops")
	p.Println("Special commands:")
	p.Println("\t)help special")
	p.Println("Search docs:")
	p.Println("\t)help about <word>")
	p.Println("Specific op:")
	p.Println("\t)help <op>")
	p.Println()
	p.Println("More at: https://pkg.go.dev/robpike.io/ivy")
}

func (p *Parser) printHelpBlock(start, end string) {
	for i, line := range helpLines {
		if strings.HasPrefix(line, start) {
			for _, line := range helpLines[i:] {
				if strings.HasPrefix(line, end) {
					return
				}
				p.Printf("%s\n", line)
			}
		}
	}
}

func (p *Parser) helpAbout(str string) { // str is already lowercase.
	for _, line := range helpLines {
		if strings.Contains(strings.ToLower(line), str) {
			p.Printf("%s\n", line)
		}
	}
}

func (p *Parser) help(str string) {
	unaryPair, unary := helpUnary[str]
	binaryPair, binary := helpBinary[str]
	axisPair, axis := helpAxis[str]
	if !unary && !binary && !axis {
		p.Printf("no docs for %q\n", str)
		return
	}
	if unary {
		p.Println("Unary operators:")
		p.Println("	Name              APL   Ivy     Meaning")
		for i := unaryPair.start; i <= unaryPair.end; i++ {
			p.Printf("%s\n", helpLines[i])
		}
	}
	if binary {
		if unary {
			p.Println()
		}
		p.Println("Binary operators:")
		p.Println("	Name                  APL   Ivy     Meaning")
		for i := binaryPair.start; i <= binaryPair.end; i++ {
			p.Printf("%s\n", helpLines[i])
		}
	}
	if axis {
		if unary || binary {
			p.Println()
		}
		p.Println("Axis operators:")
		p.Println("	Name                APL  Ivy  APL Example  Ivy Example  Meaning (of example)")
		for i := axisPair.start; i <= axisPair.end; i++ {
			p.Printf("%s\n", helpLines[i])
		}
	}
}
