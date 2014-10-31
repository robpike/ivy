// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import (
	"fmt"
	"strconv"

	"code.google.com/p/rspace/ivy/scan"
	"code.google.com/p/rspace/ivy/value"
)

func (p *Parser) need(want scan.Type) scan.Token {
	tok := p.Next()
	if tok.Type != want {
		p.errorf("expected %s, got %s", want, tok)
	}
	return tok
}

func (p *Parser) special() {
	switch p.need(scan.Identifier).Text {
	case "format":
		str, err := strconv.Unquote(p.need(scan.String).Text)
		if err != nil {
			p.errorf("%s", err)
		}
		p.config.SetFormat(str)
	case "debug":
		name := p.need(scan.Identifier).Text
		if p.Peek().Type != scan.Number {
			// Toggle the value
			p.config.SetDebug(name, !p.config.Debug(name))
			if p.config.Debug(name) {
				fmt.Println("now 1")
			} else {
				fmt.Println("now 0")
			}
			break
		}
		number, err := value.ValueString(p.need(scan.Number).Text)
		if err != nil {
			p.errorf("%s", err)
		}
		v, ok := number.(value.Int)
		p.config.SetDebug(name, ok && v.ToBool())
	}
	p.need(scan.Newline)
}
