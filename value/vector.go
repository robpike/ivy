// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"bytes"
	"fmt"
	"sort"

	"robpike.io/ivy/config"
)

type Vector []Value

func (v Vector) String() string {
	return "(" + v.Sprint(debugConf) + ")"
}

func (v Vector) Sprint(conf *config.Config) string {
	return v.makeString(conf, !v.AllChars())
}

func (v Vector) ProgString() string {
	// There is no such thing as a vector in program listings; they
	// are represented as a sliceExpr.
	panic("vector.ProgString - cannot happen")
}

// makeString is like String but takes a flag specifying
// whether to put spaces between the elements. By
// default (that is, by calling String) spaces are suppressed
// if all the elements of the Vector are Chars.
func (v Vector) makeString(conf *config.Config, spaces bool) string {
	var b bytes.Buffer
	for i, elem := range v {
		if spaces && i > 0 {
			fmt.Fprint(&b, " ")
		}
		fmt.Fprintf(&b, "%s", elem.Sprint(conf))
	}
	return b.String()
}

// AllChars reports whether the vector contains only Chars.
func (v Vector) AllChars() bool {
	for _, c := range v {
		if _, ok := c.Inner().(Char); !ok {
			return false
		}
	}
	return true
}

func NewVector(elem []Value) Vector {
	return Vector(elem)
}

func (v Vector) Eval(Context) Value {
	return v
}

func (v Vector) Inner() Value {
	return v
}

func (v Vector) Copy() Vector {
	elem := make([]Value, len(v))
	copy(elem, v)
	return NewVector(elem)
}

func (v Vector) toType(conf *config.Config, which valueType) Value {
	switch which {
	case vectorType:
		return v
	case matrixType:
		return NewMatrix([]Value{Int(len(v))}, v)
	}
	Errorf("cannot convert vector to %s", which)
	return nil
}

func (v Vector) sameLength(x Vector) {
	if len(v) != len(x) {
		Errorf("length mismatch: %d %d", len(v), len(x))
	}
}

// rotate returns a copy of v with elements rotated left by n.
func (v Vector) rotate(n int) Value {
	if len(v) == 0 {
		return v
	}
	if len(v) == 1 {
		return v[0]
	}
	n %= len(v)
	if n < 0 {
		n += len(v)
	}
	elems := make([]Value, len(v))
	doRotate(elems, v, n%len(elems))
	return NewVector(elems)
}

func doRotate(dst, src []Value, j int) {
	for i := range dst {
		dst[i] = src[j]
		j++
		if j >= len(src) {
			j = 0
		}
	}
}

// grade returns as a Vector the indexes that sort the vector into increasing order
func (v Vector) grade(c Context) Vector {
	x := make([]int, len(v))
	for i := range x {
		x[i] = i
	}
	sort.Sort(&gradeIndex{c: c, v: v, x: x})
	origin := c.Config().Origin()
	result := make([]Value, len(v))
	for i, index := range x {
		n := origin + index
		if n > maxInt { // Unlikely but be careful.
			result[i] = bigInt64(int64(n))
		} else {
			result[i] = Int(n)
		}
	}
	return NewVector(result)
}

// membership creates a vector of size len(u) reporting
// whether each element is an element of v.
// TODO: N*M algorithm - can we do better?
func membership(c Context, u, v Vector) Value {
	values := make([]Value, len(u))
	for i, x := range u {
		values[i] = Int(0)
		for _, y := range v {
			if c.EvalBinary(x, "==", y) == Int(1) {
				values[i] = Int(1)
				break
			}
		}
	}
	return NewVector(values).shrink()
}

func (v Vector) shrink() Value {
	if len(v) == 1 {
		return v[0]
	}
	return v
}

type gradeIndex struct {
	c Context
	v Vector
	x []int
}

func (g *gradeIndex) Len() int {
	return len(g.v)
}

func (g *gradeIndex) Less(i, j int) bool {
	return toBool(g.c.EvalBinary(g.v[g.x[i]], "<", g.v[g.x[j]]))
}

func (g *gradeIndex) Swap(i, j int) {
	g.x[i], g.x[j] = g.x[j], g.x[i]
}
