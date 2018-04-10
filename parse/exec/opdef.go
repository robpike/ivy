// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec // import "robpike.io/ivy/parse/exec"

// OpDef is just a record of an op's name and arg count.
// It is held in execContext.defs to control writing the
// ops out in the right order during save.
// See comment in parse/save.go. TODO: Move save.go here.
type OpDef struct {
	Name     string
	IsBinary bool
}
