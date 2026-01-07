// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package state provides the sole implementation of config.State.
package state

import (
	"io"

	"robpike.io/ivy/value"
)

type State struct {
	value.Context
}

func New(c value.Context) *State {
	return &State{c}
}

func (s *State) IsBinaryOp(op string) bool {
	return value.BinaryOps[op] != nil
}

func (s *State) InputBase() int {
	return s.Config().InputBase()
}

func (s *State) Debug(word string) int {
	return s.Config().Debug(word)
}

func (s *State) Output() io.Writer {
	return s.Config().Output()
}

func (s *State) Predefined(op string, isUnary, isBinary bool) bool {
	if isUnary && value.UnaryOps[op] != nil {
		return true
	}
	if isBinary && value.BinaryOps[op] != nil {
		return true
	}
	return false
}
