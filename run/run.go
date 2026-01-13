// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package run provides the execution control for ivy.
// It is factored out of main so it can be used for tests.
// This layout also helps out ivy/mobile.
package run // import "robpike.io/ivy/run"

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"robpike.io/ivy/parse"
	"robpike.io/ivy/scan"
	"robpike.io/ivy/state"
	"robpike.io/ivy/value"
)

func init() {
	value.IvyEval = IvyEval
}

// IvyEval is the function called by value/unaryIvy to implement the ivy (eval) operation.
// It is exported but is not intended to be used outside of ivy.
func IvyEval(context value.Context, str string) value.Value {
	scanner := scan.New(state.New(context), "<ivy>", strings.NewReader(str))
	parser := parse.NewParser("<ivy>", scanner, context)
	v := eval(parser, context)
	if v == nil {
		v = value.NewIntVector() // Must return something, so make it an empty vector.
	}
	return v
}

// cpuTime reports user and system time.
// It is replaced by system-specific files, like time_unix.go.
var cpuTime = func() (user, sys time.Duration) { return 0, 0 }

// Run runs the parser/evaluator until EOF or error.
// The return value says whether we completed without error. If the return
// value is true, it means we ran out of data (EOF) and the run was successful.
// Typical execution is therefore to loop calling Run until it succeeds.
// Error details are reported to the configured error output stream.
func Run(p *parse.Parser, context value.Context, interactive bool) (success bool) {
	conf := context.Config()
	writer := conf.Output()
	defer func() {
		if conf.Debug("panic") > 0 {
			return
		}
		err := recover()
		if err == nil {
			return
		}
		msg := ""
		e, ok := err.(value.Error)
		if ok {
			msg = fmt.Sprintf("%s: %s\n", e.Pos, e.Err)
		} else {
			msg = fmt.Sprint(err)
			_, ok = err.(big.ErrNaN) // Floating point error from math/big.
		}
		if ok {
			fmt.Fprint(conf.ErrOutput(), msg)
			if interactive {
				fmt.Fprintln(writer)
			}
			success = false
			return
		}
		panic(err) // Unexpected type.
	}()
	for {
		if interactive {
			fmt.Fprint(writer, conf.Prompt())
		}
		exprs, ok := p.Line()
		var values []value.Value
		if exprs != nil {
			start := time.Now()
			user, sys := cpuTime()
			values = context.Eval(exprs)
			user2, sys2 := cpuTime()
			conf.SetCPUTime(time.Since(start), user2-user, sys2-sys)
		}
		if printValues(context, writer, values) {
			context.AssignGlobal("_", values[len(values)-1])
		}
		if !ok {
			return true
		}
		if interactive {
			if exprs != nil && conf.Debug("cpu") > 0 {
				if real, _, _ := conf.CPUTime(); real != 0 {
					fmt.Printf("(%s)\n", conf.PrintCPUTime())
				}
			}
			fmt.Fprintln(writer)
		}
	}
}

// eval runs until EOF or error. It prints every value but the last, and returns the last.
// By last we mean the last expression of the last evaluation.
// (Expressions are separated by ; in the input.)
// It is always called from (somewhere below) run, so if it errors out the recover in
// run will catch it.
func eval(p *parse.Parser, context value.Context) value.Value {
	conf := context.Config()
	writer := conf.Output()
	var prevValues []value.Value
	for {
		exprs, ok := p.Line()
		var values []value.Value
		if exprs != nil {
			values = context.Eval(exprs)
		}
		if !ok {
			if len(prevValues) == 0 {
				return nil
			}
			printValues(context, writer, prevValues[:len(prevValues)-1])
			return prevValues[len(prevValues)-1]
		}
		printValues(context, writer, prevValues)
		prevValues = values
	}
}

// printValues neatly prints the values returned from execution, followed by a newline.
// It also handles the ')debug types' output.
// The return value reports whether it printed anything.
func printValues(c value.Context, writer io.Writer, values []value.Value) bool {
	if len(values) == 0 {
		return false
	}
	if c.Config().Debug("types") > 0 {
		for i, v := range values {
			if i > 0 {
				fmt.Fprint(writer, ",")
			}
			fmt.Fprintf(writer, "%T", v)
		}
		fmt.Fprintln(writer)
	}
	printed := false
	for _, v := range values {
		switch v.(type) {
		case value.QuietValue:
			continue
		}
		s := v.Sprint(c)
		if printed && len(s) > 0 && s[len(s)-1] != '\n' {
			fmt.Fprint(writer, " ")
		}
		fmt.Fprint(writer, s)
		printed = true
	}
	if printed {
		fmt.Fprintln(writer)
	}
	return printed
}

// Ivy evaluates the input string, appending standard output
// and error output to the provided buffers, which it does by
// calling context.Config.SetOutput and SetError.
// If execution caused errors, they will be returned concatenated
// together in the error value returned.
func Ivy(context value.Context, expr string, stdout, stderr *bytes.Buffer) {
	if !strings.HasSuffix(expr, "\n") {
		expr += "\n"
	}
	reader := strings.NewReader(expr)

	scanner := scan.New(state.New(context), " ", reader)
	parser := parse.NewParser(" ", scanner, context)

	conf := context.Config()
	conf.SetOutput(stdout)
	conf.SetErrOutput(stderr)
	for !Run(parser, context, false) {
	}
}
