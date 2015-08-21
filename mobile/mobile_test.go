// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mobile

import (
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
