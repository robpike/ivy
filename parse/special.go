// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse

import "code.google.com/p/rspace/ivy/scan"

func (p *Parser) need(want scan.Type) scan.Token {
	tok := p.Next()
	if tok.Type != want {
		p.errorf("expected %s, got %s", want, tok)
	}
	return tok
}

func (p *Parser) special() {
	switch p.need(scan.Identifier) {
	case "format":
		p.config.SetFormat(p.need(scan.String).Text)
	}
	p.need(scan.Newline)
}
