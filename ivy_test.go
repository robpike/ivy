// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"robpike.io/ivy/config"
	"robpike.io/ivy/exec"
	"robpike.io/ivy/run"
	"robpike.io/ivy/value"
)

const verbose = false

var testConf config.Config

func init() {
	value.MaxParallelismForTesting()
}

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
		t.Log(name)
		shouldFail := strings.HasSuffix(name, "_fail.ivy")
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
			input, output, length := getText(t, path, lineNum, shouldFail, lines)
			if input == nil {
				break
			}
			if verbose {
				fmt.Printf("%s:%d: %s\n", path, lineNum, input)
			}
			if !runTest(t, path, lineNum, shouldFail, input, output) {
				errCount++
				if errCount > 100 {
					t.Fatal("too many errors")
				}
			}
			lines = lines[length:]
			lineNum += length
		}
	}
}

func runTest(t *testing.T, name string, lineNum int, shouldFail bool, input, output []string) bool {
	reset()
	in := strings.Join(input, "\n")
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	run.Ivy(exec.NewContext(&testConf), in, stdout, stderr)
	if shouldFail {
		if stderr.Len() == 0 {
			t.Fatalf("\nexpected execution failure at %s:%d:\n%s", name, lineNum, in)
		}
		expect := ""
		for _, s := range input {
			if strings.HasPrefix(s, "# Expect: ") {
				expect = s[len("# Expect: "):]
			}
		}
		if expect != "" && !strings.Contains(stderr.String(), expect) {
			t.Errorf("\nunexpected execution failure message at %s:%d:\n%s", name, lineNum, in)
			t.Errorf("got:\n\t%s", stderr)
			t.Fatalf("expected:\n\t%s\n", expect)
		}
		return true
	}
	if stderr.Len() != 0 {
		t.Fatalf("\nexecution failure (%s) at %s:%d:\n%s", stderr, name, lineNum, in)
	}
	result := strings.Split(stdout.String(), "\n")
	if !equal(result, output) {
		t.Errorf("\n%s:%d:\n\t%s\ngot:\n\t%s\nwant:\n\t%s",
			name, lineNum,
			strings.Join(input, "\n\t"),
			strings.Join(result, "\n\t"),
			strings.Join(output, "\n\t"))
		return false
	}
	return true
}

func equal(a, b []string) bool {
	// Split leaves an empty trailing line.
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

func getText(t *testing.T, fileName string, lineNum int, shouldFail bool, lines []string) (input, output []string, length int) {
	// Skip blank and initial comment lines, except keep leading comment for failure checks.
	if !shouldFail {
		for _, line := range lines {
			if len(line) > 0 && !strings.HasPrefix(line, "#") {
				break
			}
			length++
		}
	}

	// Input ends at tab-indented line.
	for _, line := range lines[length:] {
		line = strings.TrimRight(line, " \t")
		if strings.HasPrefix(line, "\t") {
			break
		}
		input = append(input, line)
		length++
	}

	// Output ends at non-blank, non-tab-indented line.
	// Indented "#" is expected blank line in output.
	for _, line := range lines[length:] {
		line = strings.TrimRight(line, " \t")
		if line != "" && !strings.HasPrefix(line, "\t") {
			break
		}
		output = append(output, strings.TrimPrefix(line, "\t"))
		length++
	}
	for len(output) > 0 && output[len(output)-1] == "" {
		output = output[:len(output)-1]
	}
	for i, line := range output {
		if line == "#" {
			output[i] = ""
		}
	}

	return // Will return nil if no more tests exist.
}

func reset() {
	testConf = config.Config{}
	testConf.SetFloatPrec(256)
	testConf.SetFormat("")
	testConf.SetMaxBits(1e9)
	testConf.SetMaxDigits(1e4)
	testConf.SetMaxStack(100000)
	testConf.SetOrigin(1)
	testConf.SetPrompt("")
	testConf.SetBase(0, 0)
	testConf.SetRandomSeed(0)
	testConf.SetLocation("UTC")
}
