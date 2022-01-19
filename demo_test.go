// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"robpike.io/ivy/config"
	"robpike.io/ivy/demo"
	"robpike.io/ivy/exec"
	"robpike.io/ivy/run"
)

/*
To update demo/demo.out:
	ivy -i ')seed 0' demo/demo.ivy > demo/demo.out
*/

const (
	demoBad = "demo.bad"
	demoOut = "demo/demo.out"
)

func TestDemo(t *testing.T) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	var conf config.Config
	conf.SetRandomSeed(0)
	context := exec.NewContext(&conf)
	scan := bufio.NewScanner(strings.NewReader(demo.Text()))
	for scan.Scan() {
		run.Ivy(context, scan.Text(), stdout, stderr)
		if stderr.Len() > 0 {
			t.Fatal(stderr.String())
		}
	}
	result := stdout.String()
	data, err := ioutil.ReadFile(demoOut)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != result {
		err = ioutil.WriteFile(demoBad, stdout.Bytes(), 0666)
		if err != nil {
			t.Fatalf("test output differs; error writing bad output to %q: %v", demoBad, err)
		}
		t.Fatalf("test output differs; run\n\tdiff %s %s\nfor details", demoBad, demoOut)
	}
	err = os.Remove(demoBad)
	if err != nil {
		t.Logf("error removing test output file %q: %v", demoBad, err)
	}
}
