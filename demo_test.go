// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"io/ioutil"
	"testing"

	"robpike.io/ivy/exec"
	"robpike.io/ivy/parse"
	"robpike.io/ivy/scan"
	"robpike.io/ivy/value"
)

/*
To update demo/demo.out:
	(echo ')seed 0'; cat demo/demo.ivy) | ivy | sed 1d > demo/demo.out
*/
func TestDemo(t *testing.T) {
	initConf()
	value.SetConfig(&conf)

	data, err := ioutil.ReadFile("demo/demo.ivy")
	check := func() {
		if err != nil {
			t.Fatal(err)
		}
	}
	check()
	context := exec.NewContext()
	scanner := scan.New(&conf, context, "", bytes.NewBuffer(data))
	value.SetContext(context)
	parser := parse.NewParser(&conf, "demo.ivy", scanner, context)
	if !run(parser, context, true) {
		t.Fatal("demo execution error")
	}
	result := testBuf.String()
	data, err = ioutil.ReadFile("demo/demo.out")
	check()
	if string(data) != result {
		err = ioutil.WriteFile("demo.bad", testBuf.Bytes(), 0666)
		t.Fatal("test output differs; run\n\tdiff demo/demo.out demo.bad\nfor details")
	}
}
