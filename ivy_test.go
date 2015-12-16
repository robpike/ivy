// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"robpike.io/ivy/mobile" // The mobile package has the handy Eval function.
)

// Note: These tests share some infrastructure and cannot run in parallel.

func TestAll(t *testing.T) {

	var err error
	check := func() {
		if err != nil {
			t.Fatal(err)
		}
	}
	dir, err := os.Open("testdata")
	check()
	names, err := dir.Readdirnames(0)
	check()
	for _, name := range names {
		if !strings.HasSuffix(name, ".ivy") {
			continue
		}
		var data []byte
		path := filepath.Join("testdata", name)
		data, err = ioutil.ReadFile(path)
		check()
		text := string(data)
		lines := strings.Split(text, "\n")
		// Will have a trailing empty string.
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		lineNum := 1
		errCount := 0
		for len(lines) > 0 {
			// Assemble the input to one example.
			input, output, length := getText(t, path, lineNum, lines)
			if input == nil {
				break
			}
			if !runTest(t, path, lineNum, input, output) {
				errCount++
				if errCount > 3 {
					t.Fatal("too many errors")
				}
			}
			lines = lines[length:]
			lineNum += length
		}
	}
}

func runTest(t *testing.T, name string, lineNum int, input, output []string) bool {
	shouldFail := strings.HasSuffix(name, "_fail.ivy")
	mobile.Reset()
	in := strings.Join(input, "\n")
	result, err := mobile.Eval(in)
	if shouldFail {
		if err == nil {
			t.Fatalf("\nexpected execution failure at %s:%d:\n%s", name, lineNum, in)
		}
		return true
	}
	if err != nil {
		t.Fatalf("\nexecution failure (%s) at %s:%d:\n%s", err, name, lineNum, in)
	}
	if shouldFail {
		return true
	}
	if !equal(strings.Split(result, "\n"), output) {
		t.Errorf("\n%s:%d:\n%s\ngot:\n%swant:\n%s",
			name, lineNum,
			strings.Join(input, "\n"), result, strings.Join(output, "\n"))
		return false
	}
	return true
}

func equal(a, b []string) bool {
	// Split leaves an empty traililng line.
	if len(a) > 0 && a[len(a)-1] == "" {
		a = a[:len(a)-1]
	}
	if len(a) != len(b) {
		return false
	}
	for i, s := range a {
		if strings.TrimSpace(s) != strings.TrimSpace(b[i]) {
			return false
		}
	}
	return true
}

func getText(t *testing.T, fileName string, lineNum int, lines []string) (input, output []string, length int) {
	// Skip blank and initial comment lines.
	for _, line := range lines {
		if len(line) > 0 && !strings.HasPrefix(line, "#") {
			break
		}
		length++
	}
	// Input starts in left column.
	for _, line := range lines[length:] {
		if len(line) == 0 {
			t.Fatalf("%s:%d: unexpected empty line", fileName, lineNum+length)
		}
		if strings.HasPrefix(line, "\t") {
			break
		}
		input = append(input, line)
		length++
	}
	// Output is indented by a tab.
	for _, line := range lines[length:] {
		length++
		if len(line) == 0 {
			break
		}
		if !strings.HasPrefix(line, "\t") {
			t.Fatalf("%s:%d: output not indented", fileName, lineNum+length)
		}
		output = append(output, line[1:])
	}
	return // Will return nil if no more tests exist.
}
