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
	lines, _ := v.multiLineSprint(conf, v.allScalars(), allChars, !allChars, trimTrailingSpace)
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

// Constants to make it easier to read calls to the printing routines.
const (
	withParens        = true
	withSpaces        = true
	trimTrailingSpace = true
)

// oneLineSprint prints a vector as a single line (assuming
// there are no hidden newlines within) and returns the result.
// Flags report whether parentheses will be needed and
// whether to put spaces between the elements.
func (v Vector) oneLineSprint(conf *config.Config, parens, spaces bool) (string, []int) {
	var b bytes.Buffer
	if parens {
		spaces = true
	}
	cols := make([]int, len(v))
	for i, elem := range v {
		if spaces && i > 0 {
			fmt.Fprint(&b, " ")
		}
		if parens && !isScalarType(elem) {
			fmt.Fprintf(&b, "(%s)", elem.Sprint(conf))
		} else {
			fmt.Fprintf(&b, "%s", elem.Sprint(conf))
		}
		cols[i] = b.Len()
	}
	return b.String(), cols
}

// multiLineSprint formats a vector that may span multiple lines,
// returning the result as a slice of strings, one per line.
// Lots of flags:
//	allScalars: the vector is all scalar values and can be printed without parens.
//	allChars: the vector is all chars and can be printed extra simply.
//	spaces: put spaces between elements.
//	trim: remove trailing spaces from each line.
// If trim is not set, the lines are all of equal length, bytewise.
//
// The return values are the printed lines and, along the other axis,
// byte positions after each column.
func (v Vector) multiLineSprint(conf *config.Config, allScalars, allChars, spaces, trim bool) ([]string, []int) {
	if allScalars {
		// Easy case, might as well be efficient.
		str, cols := v.oneLineSprint(conf, false, spaces)
		return []string{str}, cols
	}
	cols := make([]int, len(v))
	if allChars {
		// Special handling as the array may contain newlines.
		// Ignore all the other flags.
		// TODO: We can still get newlines for individual elements
		// in the general case handled below.
		b := strings.Builder{}
		for i, c := range v {
			b.WriteRune(rune(c.Inner().(Char)))
			cols[i] = b.Len()
		}
		return strings.Split(b.String(), "\n"), cols // We shouldn't need cols, but be safe.
	}
	lines := []*strings.Builder{}
	lastColumn := []int{} // For each line, last column with a non-padding character.
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
		doParens := !allScalars && !isScalarType(elem)
		if doParens {
			lines[0].WriteString("(")
			lastColumn[0] = lines[0].Len()
		}
		for n, s := range strs {
			if s == "" {
				if _, ok := elem.(*Matrix); ok {
					// Blank line in matrix output; ignore
					continue
				}
			}
			line := lines[n]
			if doParens && n > 0 {
				line.WriteString("|")
				lastColumn[n] = line.Len()
			}
			line.WriteString(s)
			lastColumn[n] = line.Len()
			if doParens && n < len(strs)-1 {
				line.WriteString("|")
				lastColumn[n] = line.Len()
			}
		}
		cols[i] = lines[0].Len() // By construction all lines have same length.
		if len(strs) < len(lines) {
			// Right-fill the lines below this element.
			padding := blanks(cols[i] - lines[len(lines)-1].Len())
			for j := len(strs); j < len(lines); j++ {
				lines[j].WriteString(padding)
			}
		}
		if doParens {
			last := len(strs) - 1
			line := lines[last]
			line.WriteString(")")
			lastColumn[last] = line.Len()
			if line.Len() > cols[i] {
				cols[i] = line.Len()
			}
		}
	}
	s := make([]string, len(lines))
	for i := range s {
		s[i] = lines[i].String()
		if trim {
			s[i] = s[i][:lastColumn[i]]
		}
	}
	return s, cols
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
