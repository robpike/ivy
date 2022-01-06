// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"bytes"
	"fmt"
	"math"
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

func (v Vector) Rank() int {
	return 1
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

// AllInts reports whether the vector contains only Ints.
func (v Vector) AllInts() bool {
	for _, c := range v {
		if _, ok := c.Inner().(Int); !ok {
			return false
		}
	}
	return true
}

func NewVector(elems []Value) Vector {
	return Vector(elems)
}

func NewIntVector(elems []int) Vector {
	vec := make([]Value, len(elems))
	for i, elem := range elems {
		vec[i] = Int(elem)
	}
	return Vector(vec)
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

func (v Vector) toType(op string, conf *config.Config, which valueType) Value {
	switch which {
	case vectorType:
		return v
	case matrixType:
		return NewMatrix([]int{len(v)}, v)
	}
	Errorf("%s: cannot convert vector to %s", op, which)
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
	n := copy(dst, src[j:])
	copy(dst[n:n+j], src[:j])
}

// grade returns as a Vector the indexes that sort the vector into increasing order
func (v Vector) grade(c Context) Vector {
	x := make([]int, len(v))
	for i := range x {
		x[i] = i
	}
	sort.Slice(x, func(i, j int) bool {
		return toBool(c.EvalBinary(v[x[i]], "<", v[x[j]]))
	})
	origin := c.Config().Origin()
	for i := range x {
		x[i] += origin
	}
	return NewIntVector(x)
}

// reverse returns the reversal of a vector.
func (v Vector) reverse() Vector {
	r := v.Copy()
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return r
}

// membership creates a vector of size len(u) reporting
// whether each element is an element of v.
// Algorithm is O(nV log nV + nU log nV) where nU==len(u) and nV==len(V).
func membership(c Context, u, v Vector) []Value {
	values := make([]Value, len(u))
	sortedV := v.sortedCopy(c)
	work := 2 * (1 + int(math.Log2(float64(len(v)))))
	pfor(true, work, len(values), func(lo, hi int) {
		for i := lo; i < hi; i++ {
			values[i] = toInt(sortedV.contains(c, u[i]))
		}
	})
	return values
}

// sortedCopy returns a copy of v, in ascending sorted order.
func (v Vector) sortedCopy(c Context) Vector {
	sortedV := make([]Value, len(v))
	copy(sortedV, v)
	sort.Slice(sortedV, func(i, j int) bool {
		return c.EvalBinary(sortedV[i], "<", sortedV[j]) == Int(1)
	})
	return sortedV
}

// contains reports whether x is in v, which must be already in ascending
// sorted order.
func (v Vector) contains(c Context, x Value) bool {
	pos := sort.Search(len(v), func(j int) bool {
		return c.EvalBinary(v[j], ">=", x) == Int(1)
	})
	return pos < len(v) && c.EvalBinary(v[pos], "==", x) == Int(1)
}

func (v Vector) shrink() Value {
	if len(v) == 1 {
		return v[0]
	}
	return v
}
