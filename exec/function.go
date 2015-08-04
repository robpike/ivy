// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

import (
	"fmt"

	"robpike.io/ivy/value"
)

// Function represents a unary or binary user-defined operator.
type Function struct {
	IsBinary bool
	Name     string
	Left     string
	Right    string
	Body     []value.Expr
}

func (fn *Function) String() string {
	left := ""
	if fn.IsBinary {
		left = fn.Left + " "
	}
	s := fmt.Sprintf("op %s%s %s =", left, fn.Name, fn.Right)
	if len(fn.Body) == 1 {
		return s + " " + fn.Body[0].ProgString()
	}
	for _, stmt := range fn.Body {
		s += "\n\t" + stmt.ProgString()
	}
	return s
}
