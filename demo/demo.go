// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

// Demo is a demo driver for ivy. It takes no arguments.
// Install ivy into your $PATH, then build demo and run
// it in the demo directory.
//
// It reads each line of the demo.ivy file, waiting for a
// newline on standard input to proceed. After receiving
// a newline, it prints the next line of demo.ivy and also
// feeds it to a single running ivy instance, which prints
// the output of the command to standard output. Thus
// demo serves as a way to control the input to ivy and
// thus demonstrate its abilities.
package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) != 1 {
		log.Fatal("Usage: cd ivy/demo; go build demo.go; ./demo\n")
	}
	log.SetPrefix("demo: ")
	text, err := ioutil.ReadFile(pathTo("demo.ivy"))
	ck(err)
	cmd := exec.Command("ivy")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	input, err := cmd.StdinPipe()
	ck(err)
	err = cmd.Start()
	ck(err)
	scan := bufio.NewScanner(os.Stdin)
	fmt.Println("Type a newline to get started.")
	for scan.Scan() {
		// User typed something; step back across the newline.
		if len(scan.Bytes()) > 0 {
			// User typed a non-empty line of text; send that.
			line := []byte(fmt.Sprintf("%s\n", scan.Bytes()))
			_, err = input.Write(line)
		} else {
			// User typed newline; send next line of file's text.
			if len(text) == 0 {
				break
			}
			for i := 0; i < len(text); i++ {
				if text[i] == '\n' {
					os.Stdout.Write(text[:i+1])
					_, err = input.Write(text[:i+1])
					text = text[i+1:]
					break
				}
			}
		}
		ck(err)
	}
	ck(scan.Err())
}

func ck(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func exists(file string) bool {
	info, err := os.Stat(file)
	return err == nil && !info.IsDir()
}

func pathTo(file string) string {
	if exists(file) {
		return file
	}
	for _, dir := range filepath.SplitList(os.Getenv("GOPATH")) {
		if dir == "" {
			continue
		}
		name := filepath.Join(dir, "src", "robpike.io", "ivy", "demo", file)
		if exists(name) {
			return name
		}
	}
	return file // We'll get an error when we try to open it.
}
