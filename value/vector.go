// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"robpike.io/ivy/config"
)

type Vector []Value

func (v Vector) String() string {
	return "(" + v.Sprint(debugConf) + ")"
}

func (v Vector) Sprint(conf *config.Config) string {
	allChars := v.AllChars()
	allScalars := v.allScalars()
	if allScalars {
		// Easy case, might as well be efficient.
		return v.oneLineString(conf, false, !allChars)
	}
	lines := v.mutiLineString(conf, true, allChars, !allScalars, !allChars)
	switch len(lines) {
	case 0:
		return ""
	case 1:
		return lines[0]
	default:
		var b strings.Builder
		for i, line := range lines {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(line)
		}
		return b.String()
	}
}

func (v Vector) Rank() int {
	return 1
}

func (v Vector) ProgString() string {
	// There is no such thing as a vector in program listings; they
	// are represented as a sliceExpr.
	panic("vector.ProgString - cannot happen")
}

// oneLineString prints a vector as a single line (assuming
// there are no hidden newlines within) and returns the result.
// Flags report whether parentheses will be needed and
// whether to put spaces between the elements.
func (v Vector) oneLineString(conf *config.Config, parens, spaces bool) string {
	var b bytes.Buffer
	if parens {
		spaces = true
	}
	for i, elem := range v {
		if spaces && i > 0 {
			fmt.Fprint(&b, " ")
		}
		if parens && !isScalarType(elem) {
			fmt.Fprintf(&b, "(%s)", elem.Sprint(conf))
		} else {
			fmt.Fprintf(&b, "%s", elem.Sprint(conf))
		}
	}
	return b.String()
}

// mutiLineString formats a vector that may span multiple lines,
// returning the results as a slice of strings, one per line.
// Lots of flags:
//	allChars: the vector is all chars and can be printed simply.
//	parens: may need parens around an element.
//	spaces: put spaces between elements.
//	trim: remove trailing spaces from each line.
// If trim is not set, the lines are all of equal length, bytewise.
func (v Vector) mutiLineString(conf *config.Config, trim, allChars, parens, spaces bool) []string {
	if allChars {
		// Special handling as the array may contain newlines.
		// Ignore all the other flags.
		// TODO: We can still get newlines for individual elements
		// the general case handled below.
		b := strings.Builder{}
		for _, c := range v {
			b.WriteRune(rune(c.Inner().(Char)))
		}
		return strings.Split(b.String(), "\n")
	}
	lines := []*strings.Builder{}
	lastColumn := []int{} // For each line, last column with a non-padding character.
	if parens {
		spaces = true
	}
	for i, elem := range v {
		strs := strings.Split(elem.Sprint(conf), "\n")
		if len(strs) > len(lines) {
			wid := 0
			for _, line := range lines {
				if line.Len() > wid {
					wid = line.Len()
				}
			}
			leading := blanks(wid)
			for j := range strs {
				if j >= len(lines) {
					lastColumn = append(lastColumn, 0)
					lines = append(lines, &strings.Builder{})
					if j > 0 {
						lines[j].WriteString(leading)
					}
				}
			}
		}
		if spaces && i > 0 {
			for _, line := range lines {
				line.WriteString(" ")
			}
		}
		doParens := parens && !isScalarType(elem)
		if doParens {
			lines[0].WriteString("(")
			lastColumn[0] = lines[0].Len()
		}
		maxWid := 0
		for i, s := range strs {
			if s == "" {
				if _, ok := elem.(*Matrix); ok {
					// Blank line in matrix output; ignore
					continue
				}
			}
			line := lines[i]
			w := 0
			if doParens && i > 0 {
				line.WriteString("|")
				lastColumn[i] = line.Len()
				w = 1
			}
			line.WriteString(s)
			lastColumn[i] = line.Len()
			w += len(s)
			if doParens && i < len(strs)-1 {
				line.WriteString("|")
				lastColumn[i] = line.Len()
				w++
			}
			if w > maxWid {
				maxWid = w
			}
		}
		if len(strs) < len(lines) {
			// Right-fill the lines below this element.
			padding := blanks(maxWid)
			for j := len(strs); j < len(lines); j++ {
				lines[j].WriteString(padding)
			}
		}
		if doParens {
			last := len(strs) - 1
			lines[last].WriteString(")")
			lastColumn[last] = lines[last].Len()
		}
	}
	s := make([]string, len(lines))
	for i := range s {
		s[i] = lines[i].String()
		if trim {
			s[i] = s[i][:lastColumn[i]]
		}
	}
	return s
}

var (
	blanksLock   sync.RWMutex
	staticBlanks string
)

// blanks returns a string of n blanks.
func blanks(n int) string {
	for {
		blanksLock.RLock()
		if len(staticBlanks) >= n {
			result := staticBlanks[:n]
			blanksLock.RUnlock()
			return result
		}
		blanksLock.RUnlock()
		blanksLock.Lock()
		if len(staticBlanks) < n {
			staticBlanks = strings.Repeat(" ", n+32)
		}
		blanksLock.Unlock()
	}

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

// allScalars reports whether all the elements are scalar.
func (v Vector) allScalars() bool {
	for _, x := range v {
		if !isScalarType(x) {
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
	if elems == nil {
		// Really shouldn't happen, so catch it if it does.
		Errorf("internal error: nil vector")
	}
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

func (v Vector) Copy() Value {
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
	sort.SliceStable(x, func(i, j int) bool {
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
	r := v.Copy().(Vector)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return r
}

// membership creates a vector of size len(u) reporting
// whether each element of u is an element of v.
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
		return OrderedCompare(c, sortedV[i], sortedV[j]) < 0
	})
	return sortedV
}

// contains reports whether x is in v, which must be already in ascending
// sorted order.
func (v Vector) contains(c Context, x Value) bool {
	pos := sort.Search(len(v), func(j int) bool {
		return OrderedCompare(c, v[j], x) >= 0
	})
	return pos < len(v) && OrderedCompare(c, v[pos], x) == 0
}

func (v Vector) shrink() Value {
	if len(v) == 1 {
		return v[0]
	}
	return v
}
