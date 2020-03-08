// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"robpike.io/ivy/mobile"
)

/*
To update demo/demo.out:
	ivy -i ')seed 0' demo/demo.ivy > demo/demo.out
*/
func TestDemo(t *testing.T) {
	data, err := ioutil.ReadFile("demo/demo.ivy")
	check := func() {
		if err != nil {
			t.Fatal(err)
		}
	}
	check()
	var buf bytes.Buffer
	demo := mobile.NewDemo(string(data))
	for {
		result, err := demo.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("demo execution error: %s", err)
		}
		buf.WriteString(result)
	}
	result := buf.String()
	data, err = ioutil.ReadFile("demo/demo.out")
	check()
	if string(data) != result {
		err = ioutil.WriteFile("demo.bad", buf.Bytes(), 0666)
		t.Fatal("test output differs; run\n\tdiff demo.bad demo/demo.out\nfor details")
	}
	os.Remove("demo.bad")
}
