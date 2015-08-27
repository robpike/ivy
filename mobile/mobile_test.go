// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mobile

import (
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// We know ivy works. These just test that the wrapper works.

func TestEval(t *testing.T) {
	var tests = []struct {
		input  string
		output string
	}{
		{"", ""},
		{"23", "23\n"},
		{"sqrt 2", "1.41421356237\n"},
		{")format '%.2f'\nsqrt 2", "1.41\n"},
	}
	for _, test := range tests {
		Reset()
		out, err := Eval(test.input)
		if err != nil {
			t.Errorf("evaluating %q: %v", test.input, err)
			continue
		}
		if out != test.output {
			t.Errorf("%q: expected %q; got %q", test.input, test.output, out)
		}
	}
}

func TestEvalError(t *testing.T) {
	var tests = []struct {
		input string
		error string
	}{
		{"'x", "unterminated character constant"},
		{"1/0", "zero denominator in rational"},
		{"1 / 0", "division by zero"},
	}
	for _, test := range tests {
		Reset()
		_, err := Eval(test.input)
		if err == nil {
			t.Errorf("evaluating %q: expected %q; got nothing", test.input, test.error)
			continue
		}
		if !strings.Contains(err.Error(), test.error) {
			t.Errorf("%q: expected %q; got %q", test.input, test.error, err)
		}
	}
}

const demoText = `# This is a demo.
23
iota 10
1/0 # Cause an error.
iota 10 # Keep going
`

const demoOut = `23
1 2 3 4 5 6 7 8 9 10
1 2 3 4 5 6 7 8 9 10
`

const demoErr = " :1: zero denominator in rational\n"

func TestDemo(t *testing.T) {
	demo := NewDemo(demoText)
	results := make([]byte, 0, 100)
	errors := make([]byte, 0, 100)
	for {
		result, err := demo.Next()
		if err == io.EOF {
			break
		}
		results = append(results, result...)
		if err != nil {
			errors = append(errors, err.Error()...)
		}
	}
	if demoOut != string(results) {
		t.Fatalf("expected %q; got %q", demoOut, results)
	}
	if demoErr != string(errors) {
		t.Fatalf("expected errors %q; got %q", demoErr, errors)
	}
}

func TestHelp(t *testing.T) {
	// Test to make sure the document is up to date.
	buf, err := exec.Command("go", "run", "help_gen.go").Output()
	if err != nil {
		t.Fatalf("failed to run 'go run help_gen.go': %v", err)
	}
	f, err := ioutil.TempFile("", "mobilehelp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	_, err = f.Write(buf)
	errc := f.Close()
	if err != nil || errc != nil {
		t.Fatalf("failed to write the new help.go: %v", err)
	}

	data, err := exec.Command("diff", "-u", f.Name(), "help.go").CombinedOutput()
	if len(data) > 0 || err != nil {
		t.Errorf("Help message is outdated. Run go generate: %s (diff ended with %v)", data, err)
	}
}
