// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

// Note: All operators are not valid hexadecimal constants,
// so they work when base is 16. Higher bases are less critical.
var OperatorWord = map[string]bool{
	"abs":   true,
	"acos":  true,
	"and":   true,
	"asin":  true,
	"atan":  true,
	"ceil":  true,
	"char":  true,
	"code":  true,
	"cos":   true,
	"div":   true,
	"down":  true,
	"drop":  true,
	"fill":  true,
	"flip":  true,
	"float": true,
	"floor": true,
	"grade": true,
	"idiv":  true,
	"imod":  true,
	"in":    true,
	"iota":  true,
	"ivy":   true,
	"log":   true,
	"max":   true,
	"min":   true,
	"mod":   true,
	"nand":  true,
	"nor":   true,
	"not":   true,
	"or":    true,
	"rev":   true,
	"rho":   true,
	"rot":   true,
	"sel":   true,
	"sgn":   true,
	"sin":   true,
	"sqrt":  true,
	"take":  true,
	"tan":   true,
	"text":  true,
	"up":    true,
	"xor":   true,
}

// IsUnary identifies the binary operators; these can be used in reductions.
var IsUnary = map[string]bool{
	"**":    true,
	"+":     true,
	",":     true,
	"-":     true,
	"/":     true,
	"?":     true,
	"^":     true,
	"abs":   true,
	"acos":  true,
	"asin":  true,
	"atan":  true,
	"ceil":  true,
	"char":  true,
	"code":  true,
	"cos":   true,
	"down":  true,
	"flip":  true,
	"float": true,
	"floor": true,
	"iota":  true,
	"ivy":   true,
	"log":   true,
	"not":   true,
	"rev":   true,
	"rho":   true,
	"sgn":   true,
	"sin":   true,
	"sqrt":  true,
	"tan":   true,
	"text":  true,
	"up":    true,
}

// IsBinary identifies the binary operators; these can be used in reductions.
var IsBinary = map[string]bool{
	"!=":   true,
	"&":    true,
	"*":    true,
	"**":   true,
	"+":    true,
	",":    true, // Silly but not wrong.
	"-":    true,
	"/":    true,
	"<":    true,
	"<<":   true,
	"<=":   true,
	"==":   true,
	">":    true,
	">=":   true,
	">>":   true,
	"[]":   true,
	"^":    true,
	"and":  true,
	"div":  true,
	"drop": true,
	"fill": true,
	"idiv": true,
	"imod": true,
	"in":   true,
	"iota": true,
	"log":  true,
	"max":  true,
	"min":  true,
	"mod":  true,
	"nand": true,
	"nor":  true,
	"or":   true,
	"rho":  true,
	"rot":  true,
	"sel":  true,
	"take": true,
	"xor":  true,
	"|":    true,
}

// Defined reports whether the operator is known.
func (c *Context) Defined(op string) bool {
	if c.isVariable(op) {
		return false
	}
	return OperatorWord[op] || c.BinaryFn[op] != nil || c.UnaryFn[op] != nil
}

// DefinedBinary reports whether the operator is a known binary.
func (c *Context) DefinedBinary(op string) bool {
	if c.isVariable(op) {
		return false
	}
	return c.BinaryFn[op] != nil || OperatorWord[op] && IsBinary[op]
}

// DefinedUnary reports whether the operator is a known unary.
func (c *Context) DefinedUnary(op string) bool {
	if c.isVariable(op) {
		return false
	}
	return c.UnaryFn[op] != nil || OperatorWord[op] && IsUnary[op]
}
