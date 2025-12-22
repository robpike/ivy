// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run libgen.go

package lib

// Library holds the relevant information for a (loaded or unloaded) Library entry.
type Library struct {
	Name   string
	Doc    string
	Ops    string
	Vars   string
	Source string
}

var testing bool

var Directory []*Library = directory // directory is built by generating with libgen.go.

func Lookup(name string) *Library {
	for _, e := range Directory {
		if e.Name == name {
			return e
		}
	}
	if testing && name == "_test" {
		return testLibrary
	}
	return nil
}

func Testing(t bool) {
	testing = t
}
