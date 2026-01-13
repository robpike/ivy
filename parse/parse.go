// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse // import "robpike.io/ivy/parse"

import (
	"fmt"
	"strings"

	"robpike.io/ivy/exec"
	"robpike.io/ivy/scan"
	"robpike.io/ivy/value"
)

// tree formats an expression in an unambiguous form for debugging.
// It generates the output for )debug parse.
func tree(c value.Context, e interface{}) string {
	switch e := e.(type) {
	case value.Int:
		return fmt.Sprintf("<int %s>", e)
	case value.BigInt:
		return fmt.Sprintf("<bigint %s>", e)
	case value.BigRat:
		return fmt.Sprintf("<rat %s>", e)
	case value.BigFloat:
		return fmt.Sprintf("<float %s>", e)
	case value.Complex:
		return fmt.Sprintf("<complex %s>", e)
	case *value.Vector:
		s := "<"
		for i, x := range e.All() {
			if i > 0 {
				s += " "
			}
			s += x.ProgString()
		}
		s += ">"
		return s
	case value.VectorExpr:
		s := "<"
		for i, x := range e {
			if i > 0 {
				s += " "
			}
			s += x.ProgString()
		}
		s += ">"
		return s
	case *value.VarExpr:
		return fmt.Sprintf("<var %s>", e.Name)
	case *value.UnaryExpr:
		return fmt.Sprintf("(%s %s)", e.Op, tree(c, e.Right))
	case *value.BinaryExpr:
		return fmt.Sprintf("(%s %s %s)", tree(c, e.Left), e.Op, tree(c, e.Right))
	case *value.ColonExpr:
		return fmt.Sprintf("<%s : %s>", tree(c, e.Cond), tree(c, e.Value))
	case *value.IfExpr:
		s := fmt.Sprintf("<:if %s; %s; ", tree(c, e.Cond), tree(c, e.Body))
		if e.ElseBody != nil {
			s += fmt.Sprintf(":else %s; ", tree(c, e.ElseBody))
		}
		return s + ":end>"
	case *value.WhileExpr:
		return fmt.Sprintf("<:while %s; %s; :end>", tree(c, e.Cond), tree(c, e.Body))
	case *value.RetExpr:
		return fmt.Sprintf(":ret %s", tree(c, e.Expr))
	case value.StatementList:
		return tree(c, []value.Expr(e))
	case *value.IndexExpr:
		s := fmt.Sprintf("(%s[", tree(c, e.Left))
		for i, v := range e.Right {
			if i > 0 {
				s += "; "
			}
			s += tree(c, v)
		}
		s += "])"
		return s
	case []value.Expr:
		if len(e) == 1 {
			return tree(c, e[0])
		}
		s := "<"
		for i, expr := range e {
			if i > 0 {
				s += "; "
			}
			s += tree(c, expr)
		}
		s += ">"
		return s
	case *value.Statement:
		return tree(c, e.Parse(c))
	default:
		return fmt.Sprintf("%T", e)
	}
}

// Parser stores the state for the ivy parser.
type Parser struct {
	scanner    *scan.Scanner
	tokens     []scan.Token    // Points to tokenBuf.
	tokenBuf   [100]scan.Token // Reusable.
	fileName   string
	lineNum    int
	offset     int
	context    *exec.Context
	inOperator bool
}

// NewParser returns a new parser that will read from the scanner.
// The context must have been created by this package's NewContext function.
func NewParser(fileName string, scanner *scan.Scanner, context value.Context) *Parser {
	return &Parser{
		scanner:  scanner,
		fileName: fileName,
		context:  context.(*exec.Context),
	}
}

// Printf formats the args and writes them to the configured output writer.
func (p *Parser) Printf(format string, args ...interface{}) {
	fmt.Fprintf(p.context.Config().Output(), format, args...)
}

// Println prints the args and writes them to the configured output writer.
func (p *Parser) Println(args ...interface{}) {
	fmt.Fprintln(p.context.Config().Output(), args...)
}

// Print prints the args and writes them to the configured output writer.
func (p *Parser) Print(args ...interface{}) {
	fmt.Fprint(p.context.Config().Output(), args...)
}

func (p *Parser) next() scan.Token {
	tok := p.peek()
	if tok.Type != scan.EOF {
		p.tokens = p.tokens[1:]
		p.lineNum = tok.Line // This gives us the line number before the newline.
		p.offset = tok.Offset
	}
	if tok.Type == scan.Error {
		p.errorf("%s", tok)
	}
	return tok
}

func (p *Parser) peek() scan.Token {
	if len(p.tokens) == 0 {
		return scan.Token{Type: scan.EOF}
	}
	return p.tokens[0]
}

func (p *Parser) errorf(format string, args ...interface{}) {
	p.tokens = p.tokenBuf[:0]
	p.context.Errorf(format, args...)
}

// source returns the source code spanning the start and end lines.
func (p *Parser) source(start, end int) string {
	src := strings.Builder{}
	for _, s := range p.scanner.History()[start:end] {
		src.WriteString(s)
		src.WriteByte('\n')
	}
	return src.String()
}

// Line reads a line of input and returns the statements it holds.
// A nil returned slice means there were no statements.
// The boolean reports whether the line is valid.
//
// Line
//
//	) special command '\n'
//	op function definition
//	statementList '\n'
//
func (p *Parser) Line() (value.StatementList, bool) {
	start := len(p.scanner.History()) // Remember this location before any leading comments.
	if !p.readTokensToNewline() {
		return value.StatementList{}, false
	}
	tok := p.peek()
	switch tok.Type {
	case scan.EOF:
		return nil, true
	case scan.RightParen:
		p.special()
		p.context.SetConstants()
		return nil, true
	case scan.Op:
		p.functionDefn(start)
		return nil, true
	}
	exprs := p.statementList()
	if len(exprs) > 0 && p.context.Config().Debug("parse") > 0 {
		p.Println(tree(p.context, exprs))
	}
	return exprs, true
}

// readTokensToNewline returns the next line of input.
// The boolean is false at EOF.
// We read all tokens before parsing for easy error recovery
// if an error occurs mid-line. It also gives us lookahead
// for parsing, which we may use one day.
func (p *Parser) readTokensToNewline() bool {
	p.tokens = p.tokenBuf[:0]
	for {
		tok := p.scanner.Next()
		switch tok.Type {
		case scan.Error:
			p.errorf("%s", tok)
		case scan.Newline:
			// Need a truly blank line to terminate a multiline function body.
			if !p.inOperator || len(tok.Text) <= 1 || len(p.tokens) > 0 {
				return true
			}
			continue
		case scan.EOF:
			if p.inOperator && len(p.tokens) == 0 {
				// EOF is also fine for terminating a function.
				return true
			}
			return len(p.tokens) > 0
		}
		p.tokens = append(p.tokens, tok)
		p.lineNum = tok.Line
		p.offset = tok.Offset
	}
}

// statementList:
//
//	statement [';' statementList]...
//
func (p *Parser) statementList() value.StatementList {
	toks := []scan.Token{}
	list := value.StatementList{}
	brackLevel := 0
	ctrlLevel := 0
	i := 0
	fileName := p.fileName
	if fileName == "<stdin>" {
		fileName = ""
	}
	for {
		if i >= len(p.tokens) {
			semicolon := scan.Token{
				Type:   scan.Semicolon,
				Line:   p.lineNum,
				Offset: p.offset,
				Text:   ";",
			}
			if ctrlLevel > 0 && p.readTokensToNewline() {
				// Turn the (elided) newline into a semicolon for simpler parsing.
				toks = append(toks, semicolon)
				i = 0
				continue
			}
			break
		}
		tok := p.tokens[i]
		i++
		if tok.Type == scan.EOF || (brackLevel == 0 && ctrlLevel == 0 && tok.Type == scan.Semicolon) {
			list = append(list, value.NewStatement(toks, fileName, p.inOperator))
			toks = nil
			continue
		}
		toks = append(toks, tok)
		switch tok.Type {
		case scan.If, scan.While:
			ctrlLevel++
		case scan.End:
			ctrlLevel--
		case scan.LeftBrack:
			brackLevel++
		case scan.RightBrack:
			brackLevel--
		}
	}
	if len(toks) > 0 {
		list = append(list, value.NewStatement(toks, fileName, p.inOperator))
	}
	return list
}
