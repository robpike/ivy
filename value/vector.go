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

type Vector struct {
	data []Value
}

// Len returns the number of elements in v.
func (v *Vector) Len() int { return len(v.data) }

// At returns the i'th element of v.
func (v *Vector) At(i int) Value { return v.data[i] }

// Set sets v[i] = x.
func (v *Vector) Set(i int, x Value) { v.data[i] = x }

// All returns all the elements in v, for reading.
func (v *Vector) All() []Value { return v.data[:len(v.data):len(v.data)] }

// Writable returns all the elements in v, for writing.
func (v *Vector) Writable() []Value { return v.data }

// Slice returns a slice v[i:j], for reading.
func (v *Vector) Slice(i, j int) []Value { return v.data[i:j:j] }

func (v *Vector) String() string {
	return "(" + v.Sprint(debugConf) + ")"
}

func (v *Vector) Sprint(conf *config.Config) string {
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

func (v *Vector) Rank() int {
	return 1
}

func (v *Vector) ProgString() string {
	// There is no such thing as a vector in program listings; they
	// are represented as a VectorExpr.
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
func (v *Vector) oneLineSprint(conf *config.Config, parens, spaces bool) (string, []int) {
	var b bytes.Buffer
	if parens {
		spaces = true
	}
	cols := make([]int, v.Len())
	for i, elem := range v.All() {
		if spaces && i > 0 {
			fmt.Fprint(&b, " ")
		}
		if parens && !IsScalarType(elem) {
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
//
//	allScalars: the vector is all scalar values and can be printed without parens.
//	allChars: the vector is all chars and can be printed extra simply.
//	spaces: put spaces between elements.
//	trim: remove trailing spaces from each line.
//
// If trim is not set, the lines are all of equal length, bytewise.
//
// The return values are the printed lines and, along the other axis,
// byte positions after each column.
func (v *Vector) multiLineSprint(conf *config.Config, allScalars, allChars, spaces, trim bool) ([]string, []int) {
	if allScalars {
		// Easy case, might as well be efficient.
		str, cols := v.oneLineSprint(conf, false, spaces)
		return []string{str}, cols
	}
	cols := make([]int, v.Len())
	if allChars {
		// Special handling as the array may contain newlines.
		// Ignore all the other flags.
		// TODO: We can still get newlines for individual elements
		// in the general case handled below.
		b := strings.Builder{}
		for i, c := range v.All() {
			b.WriteRune(rune(c.Inner().(Char)))
			cols[i] = b.Len()
		}
		return strings.Split(b.String(), "\n"), cols // We shouldn't need cols, but be safe.
	}
	lines := []*strings.Builder{}
	lastColumn := []int{} // For each line, last column with a non-padding character.
	for i, elem := range v.All() {
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
		doParens := !allScalars && !IsScalarType(elem)
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

// fillValue returns a zero or a space as the appropriate fill type for the data
func fillValue(v []Value) Value {
	if len(v) == 0 {
		return zero
	}
	var fill Value = zero
	if allChars(v) {
		fill = Char(' ')
	}
	if IsScalarType(v[0]) {
		return fill
	}
	switch v := v[0].(type) {
	case *Vector:
		data := make([]Value, v.Len())
		for i := range data {
			data[i] = fill
		}
		return NewVector(data)
	case *Matrix:
		data := make([]Value, v.data.Len())
		for i := range data {
			data[i] = fill
		}
		return NewMatrix(v.shape, NewVector(data))
	}
	return zero
}

// fillValue returns a zero or a space as the appropriate fill type for the vector
func (v *Vector) fillValue() Value {
	return fillValue(v.All())
}

// allChars reports whether the top level of the data contains only Chars.
func allChars(v []Value) bool {
	for _, c := range v {
		if _, ok := c.Inner().(Char); !ok {
			return false
		}
	}
	return true
}

// AllChars reports whether the vector contains only Chars.
func (v *Vector) AllChars() bool {
	return allChars(v.All())
}

// allScalars reports whether all the elements are scalar.
func (v *Vector) allScalars() bool {
	return allScalars(v.All())
}

// allScalars reports whether all the elements are scalar.
func allScalars(v []Value) bool {
	for _, x := range v {
		if !IsScalarType(x) {
			return false
		}
	}
	return true
}

// AllInts reports whether the vector contains only Ints.
func (v *Vector) AllInts() bool {
	for _, c := range v.All() {
		if _, ok := c.Inner().(Int); !ok {
			return false
		}
	}
	return true
}

func NewVector(elems []Value) *Vector {
	return &Vector{elems}
}

func oneElemVector(elem Value) *Vector {
	return NewVector([]Value{elem})
}

func NewIntVector(elems ...int) *Vector {
	vec := make([]Value, len(elems))
	for i, elem := range elems {
		vec[i] = Int(elem)
	}
	return NewVector(vec)
}

func (v *Vector) Eval(Context) Value {
	return v
}

func (v *Vector) Inner() Value {
	return v
}

func (v *Vector) Copy() Value {
	elem := make([]Value, v.Len())
	copy(elem, v.All())
	return NewVector(elem)
}

func (v *Vector) toType(op string, conf *config.Config, which valueType) Value {
	switch which {
	case vectorType:
		return v
	case matrixType:
		return NewMatrix([]int{v.Len()}, v)
	}
	Errorf("%s: cannot convert vector to %s", op, which)
	return nil
}

func (v *Vector) sameLength(x *Vector) {
	if v.Len() != x.Len() {
		Errorf("length mismatch: %d %d", v.Len(), x.Len())
	}
}

// rotate returns a copy of v with elements rotated left by n.
func (v *Vector) rotate(n int) Value {
	if v.Len() == 0 {
		return v
	}
	if v.Len() == 1 {
		return v.At(0)
	}
	n %= v.Len()
	if n < 0 {
		n += v.Len()
	}
	elems := make([]Value, v.Len())
	doRotate(elems, v.All(), n%len(elems))
	return NewVector(elems)
}

// repl returns a Vector with each element repeated n times. n must be either one
// integer or a vector of the same length as v. elemCount is the number of elements
// we are to duplicate; this will be number of columns for a matrix's data.
func (v *Vector) repl(n *Vector, elemCount int) *Vector {
	if n.Len() != 1 && n.Len() != elemCount {
		Errorf("repl length mismatch")
	}
	result := make([]Value, 0)
	for i := range v.Len() {
		count, ok := n.At(i % n.Len()).(Int)
		if !ok {
			Errorf("repl count must be small integer")
		}
		val := v.At(i)
		for k := 0; k < int(count); k++ {
			result = append(result, val)
		}
	}
	return NewVector(result)
}

func doRotate(dst, src []Value, j int) {
	n := copy(dst, src[j:])
	copy(dst[n:n+j], src[:j])
}

// partition returns a vector of the elements of v, selected and grouped
// by the values in score. Elements with score 0 are ignored.
// Elements with non-zero score are included, grouped with boundaries
// at every point where the score exceeds the previous score.
func (v *Vector) partition(score *Vector) Value {
	if score.Len() != v.Len() {
		Errorf("part: length mismatch")
	}
	var accum, result []Value
	for i, sc, prev := 0, Int(0), Int(0); i < score.Len(); i, prev = i+1, sc {
		var ok bool
		sc, ok = score.At(i).(Int)
		if !ok || sc < 0 {
			Errorf("part: score must be non-negative integer")
		}
		if sc == 0 { // Ignore elements with zero score.
			continue
		}
		if i > 0 && sc > prev { // Add current subvector, start new one.
			result = append(result, NewVector(accum))
			accum = nil
		}
		accum = append(accum, v.At(i))
	}
	if len(accum) > 0 {
		result = append(result, NewVector(accum))
	}
	return NewVector(result)
}

// grade returns as a Vector the indexes that sort the vector into increasing order
func (v *Vector) grade(c Context) *Vector {
	x := make([]int, v.Len())
	for i := range x {
		x[i] = i
	}
	sort.SliceStable(x, func(i, j int) bool {
		return OrderedCompare(c, v.At(x[i]), v.At(x[j])) < 0
	})
	origin := c.Config().Origin()
	for i := range x {
		x[i] += origin
	}
	return NewIntVector(x...)
}

// reverse returns the reversal of a vector.
func (v *Vector) reverse() *Vector {
	r := v.Copy().(*Vector)
	for i, j := 0, r.Len()-1; i < j; i, j = i+1, j-1 {
		ri, rj := r.At(i), r.At(j)
		r.Set(i, rj)
		r.Set(j, ri)
	}
	return r
}

// inverse returns the inverse of a vector, defined to be (conj v) / v +.* conj v
func (v *Vector) inverse(c Context) Value {
	if v.Len() == 0 {
		Errorf("inverse of empty vector")
	}
	if v.Len() == 1 {
		return inverse(c, v)
	}
	// We could do this evaluation using "conj" and "+.*" but avoid the overhead.
	conj := v.Copy().(*Vector)
	for i, x := range conj.All() {
		if !IsScalarType(x) {
			Errorf("inverse of vector with non-scalar element")
		}
		if cmplx, ok := x.(Complex); ok {
			conj.Set(i, NewComplex(cmplx.real, c.EvalUnary("-", cmplx.imag)).shrink())
		}
	}
	mag := Value(zero)
	for i, x := range v.All() {
		mag = c.EvalBinary(mag, "+", c.EvalBinary(x, "*", conj.At(i)))
	}
	if isZero(mag) {
		Errorf("inverse of zero vector")
	}
	for i, x := range conj.All() {
		conj.Set(i, c.EvalBinary(x, "/", mag))
	}
	return conj
}

// membership creates a vector of size len(u) reporting
// whether each element of u is an element of v.
// Algorithm is O(nV log nV + nU log nV) where nU==len(u) and nV==len(V).
func membership(c Context, u, v *Vector) []Value {
	values := make([]Value, u.Len())
	sortedV := v.sortedCopy(c)
	work := 2 * (1 + int(math.Log2(float64(v.Len()))))
	pfor(true, work, len(values), func(lo, hi int) {
		for i := lo; i < hi; i++ {
			values[i] = toInt(sortedV.contains(c, u.At(i)))
		}
	})
	return values
}

// sortedCopy returns a copy of v, in ascending sorted order.
func (v *Vector) sortedCopy(c Context) *Vector {
	sortedV := make([]Value, v.Len())
	copy(sortedV, v.All())
	sort.Slice(sortedV, func(i, j int) bool {
		return OrderedCompare(c, sortedV[i], sortedV[j]) < 0
	})
	return NewVector(sortedV)
}

// contains reports whether x is in v, which must be already in ascending
// sorted order.
func (v *Vector) contains(c Context, x Value) bool {
	pos := sort.Search(v.Len(), func(j int) bool {
		return OrderedCompare(c, v.At(j), x) >= 0
	})
	return pos < v.Len() && OrderedCompare(c, v.At(pos), x) == 0
}

func (v *Vector) shrink() Value {
	if v.Len() == 1 {
		return v.At(0)
	}
	return v
}
