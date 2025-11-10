// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"bytes"
	"fmt"
	"iter"
	"math"
	"sort"
	"strings"
	"sync"

	"robpike.io/ivy/config"
	"robpike.io/ivy/value/persist"
)

type Vector struct {
	s *persist.Slice[Value]
}

// Len returns the number of elements in v.
func (v *Vector) Len() int { return v.s.Len() }

// At returns the i'th element of v.
func (v *Vector) At(i int) Value { return v.s.At(i) }

// All returns all the elements in v, for reading.
func (v *Vector) All() iter.Seq2[int, Value] { return v.s.All() }

// Slice returns a slice v[i:j], for reading.
func (v *Vector) Slice(i, j int) iter.Seq2[int, Value] { return v.s.Slice(i, j) }

func (v *Vector) String() string {
	return "(" + v.Sprint(debugConf) + ")"
}

// edit returns a vectorEditor for creating a modified copy of v.
func (v *Vector) edit() *vectorEditor {
	return &vectorEditor{v.s.Transient()}
}

// A vectorEditor prepares a new vector by applying a sequence
// of edits to a copy of an existing vector.
type vectorEditor struct {
	t *persist.TransientSlice[Value]
}

// newVectorEditor returns a vectorEditor editing a vector of length size
// with all elements set to def.
func newVectorEditor(size int, def Value) *vectorEditor {
	t := new(persist.TransientSlice[Value])
	if size > 0 {
		t.Resize(size)
		for i := range size {
			t.Set(i, def)
		}
	}
	return &vectorEditor{t: t}
}

// All returns all the elements in v, for reading.
func (v *vectorEditor) All() iter.Seq2[int, Value] { return v.t.All() }

// Len returns the number of elements in v.
func (v *vectorEditor) Len() int { return v.t.Len() }

// At returns the i'th element of v.
func (v *vectorEditor) At(i int) Value { return v.t.At(i) }

// Set sets the i'th element of v to x.
func (v *vectorEditor) Set(i int, x Value) { v.t.Set(i, x) }

// Append appends the values to v.
func (v *vectorEditor) Append(values ...Value) { v.t.Append(values...) }

// Resize resizes v to have n elements.
// The value of newly accessible elements is undefined.
// (It is expected that the caller will set them.)
func (v *vectorEditor) Resize(n int) { v.t.Resize(n) }

// Publish returns the edited state as an immutable Vector.
// The vectorEditor must not be used after Publish.
func (v *vectorEditor) Publish() *Vector {
	return &Vector{v.t.Persist()}
}

// NewVectorSeq creates a new vector from a sequence.
func NewVectorSeq(seq ...iter.Seq2[int, Value]) *Vector {
	edit := newVectorEditor(0, nil)
	for _, s := range seq {
		for _, v := range s {
			edit.Append(v)
		}
	}
	return edit.Publish()
}

// repeat returns a sequence of n copies of v.
func repeat(v Value, n int) iter.Seq2[int, Value] {
	return func(yield func(int, Value) bool) {
		for i := range n {
			if !yield(int(i), v) {
				return
			}
		}
	}
}

// Sprint returns the formatting of v according to conf.
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

// isAllChars reports whether v is an all-chars vector.
// The empty vector is not considered "all chars".
func isAllChars(v Value) bool {
	vv, ok := v.(*Vector)
	return ok && vv.Len() > 0 && vv.AllChars()
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
		doParens := !allScalars && !IsScalarType(elem) && !isAllChars(elem)
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
		// All lines should have the same length (except for empty lines, which
		// are never the zeroth line.)
		cols[i] = lines[0].Len()
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
		// Finally, if we missed any alignment because of all the fiddling and flags, fix it now.
		// One day we should rewrite this code to make it more robust and clear.
		wid := 0
		for _, line := range lines {
			wid = max(wid, line.Len())
		}
		cols[i] = wid
		for _, line := range lines {
			if line.Len() < wid {
				line.WriteString(blanks(wid - line.Len()))
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
func fillValue(v *Vector) Value {
	if v.Len() == 0 {
		return zero
	}
	var fill Value = zero
	if v.AllChars() {
		fill = Char(' ')
	}
	first := v.At(0)
	if IsScalarType(first) {
		return fill
	}
	switch v := first.(type) {
	case *Vector:
		data := make([]Value, v.Len())
		for i := range data {
			data[i] = fill
		}
		return newVectorEditor(v.Len(), fill).Publish()
	case *Matrix:
		return NewMatrix(v.shape, newVectorEditor(v.data.Len(), fill).Publish())
	}
	return zero
}

// fillValue returns a zero or a space as the appropriate fill type for the vector
func (v *Vector) fillValue() Value {
	return fillValue(v)
}

// AllChars reports whether the vector contains only Chars.
func (v *Vector) AllChars() bool {
	for _, c := range v.All() {
		if _, ok := c.Inner().(Char); !ok {
			return false
		}
	}
	return true
}

// allScalars reports whether all the elements are scalar.
func (v *Vector) allScalars() bool {
	for _, x := range v.All() {
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

func NewVector(elems ...Value) *Vector {
	edit := newVectorEditor(0, nil)
	edit.Append(elems...)
	return edit.Publish()
}

func oneElemVector(elem Value) *Vector {
	return newVectorEditor(1, elem).Publish()
}

func NewIntVector(elems ...int) *Vector {
	edit := newVectorEditor(len(elems), nil)
	for i, elem := range elems {
		edit.Set(i, Int(elem))
	}
	return edit.Publish()
}

func (v *Vector) Eval(Context) Value {
	return v
}

func (v *Vector) Inner() Value {
	return v
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
	edit := v.edit()
	doRotate(edit, 0, v.Len(), v, 0, n)
	return edit.Publish()
}

// sel returns a Vector with each element repeated n times. n must be either one
// integer or a vector of the same length as v. elemCount is the number of elements
// we are to duplicate; this will be number of columns for a matrix's data.
// If the count is negative, we replicate zeros of the appropriate shape.
func (v *Vector) sel(n *Vector, elemCount int) *Vector {
	if n.Len() != 1 && n.Len() != elemCount {
		Errorf("sel length mismatch")
	}
	result := newVectorEditor(0, nil)
	for i := range v.Len() {
		count := n.intAt(i%n.Len(), "sel count")
		val := v.At(i)
		if count < 0 { // Thanks, APL.
			count = -count
			val = allZeros(val)
		}
		for range count {
			result.Append(val)
		}
	}
	return result.Publish()
}

// zeros returns a value with the shape of v, but all zeroed out.
func allZeros(v Value) Value {
	switch v := v.(type) {
	case Char:
		return Char(' ')
	case *Vector:
		u := newVectorEditor(v.Len(), nil)
		for i := range u.Len() {
			u.Set(i, allZeros(v.At(i)))
		}
		return u.Publish()
	case *Matrix:
		return &Matrix{shape: v.shape, data: allZeros(v.data).(*Vector)}
	default:
		return zero
	}
}

func doRotate(dst *vectorEditor, i, n int, src *Vector, j, off int) {
	for k := range n {
		dst.Set(i+k, src.At(j+(off+k)%n))
	}
}

// uintAt returns the ith element of v, erroring out if it is not a
// non-negative integer. It's called uintAt but returns an int.
// The vector is known to be long enough.
func (v *Vector) uintAt(i int, msg string) int {
	n, ok := v.At(i).(Int)
	if !ok || n < 0 {
		Errorf("%s must be a non-negative integer: %s", msg, v.At(i))
	}
	return int(n)
}

// intAt returns the ith element of v, which must be an Int.
// The vector is known to be long enough.
func (v *Vector) intAt(i int, msg string) int {
	n, ok := v.At(i).(Int)
	if !ok {
		Errorf("%s must be a small integer: %s", msg, v.At(i))
	}
	return int(n)
}

// partition returns a vector of the elements of v, selected and grouped
// by the values in score. Elements with score 0 are ignored.
// Elements with non-zero score are included, grouped with boundaries
// at every point where the score exceeds the previous score.
func (v *Vector) partition(score *Vector) Value {
	if score.Len() != v.Len() {
		Errorf("part: length mismatch")
	}
	res, _ := v.doPartition(score)
	return res
}

// doPartition iterates along the vector to do the partitioning. It is called from
// vector and matrix partitioning code, which use different length checks. The
// integer returned is the width (last dimension) to use when partitioning a
// matrix.
func (v *Vector) doPartition(score *Vector) (*Vector, int) {
	accum := newVectorEditor(0, nil)
	result := newVectorEditor(0, nil)
	dim := -1
	for i, sc, prev := 0, 0, 0; i < v.Len(); i, prev = i+1, sc {
		j := i % score.Len()
		sc = score.uintAt(j, "part: score")
		if sc != 0 { // Ignore elements with zero score.
			if i > 0 && (sc > prev || j == 0) && accum.Len() > 0 { // Add current subvector, start new one.
				result.Append(accum.Publish())
				accum.Resize(0)
			}
			accum.Append(v.At(i))
		}
		if dim < 0 && i > 0 && j == 0 { // Score rolled over for first time; set dim.
			dim = result.Len()
		}
	}
	if accum.Len() > 0 {
		result.Append(accum.Publish())
	}
	return result.Publish(), dim
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
	r := v.edit()
	for i, j := 0, r.Len()-1; i < j; i, j = i+1, j-1 {
		ri, rj := r.At(i), r.At(j)
		r.Set(i, rj)
		r.Set(j, ri)
	}
	return r.Publish()
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
	conj := v.edit()
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
	return conj.Publish()
}

// membership creates a vector of size len(u) reporting
// whether each element of u is an element of v.
// Algorithm is O(nV log nV + nU log nV) where nU==len(u) and nV==len(V).
func membership(c Context, u, v *Vector) *Vector {
	values := newVectorEditor(u.Len(), nil)
	sortedV := v.sortedCopy(c)
	work := 2 * (1 + int(math.Log2(float64(v.Len()))))
	pfor(true, work, values.Len(), func(lo, hi int) {
		for i := lo; i < hi; i++ {
			values.Set(i, toInt(sortedV.contains(c, u.At(i))))
		}
	})
	return values.Publish()
}

type vectorByOrderedCompare struct {
	c Context
	e *vectorEditor
}

func (v *vectorByOrderedCompare) Len() int {
	return v.e.Len()
}

func (v *vectorByOrderedCompare) Swap(i, j int) {
	vi, vj := v.e.At(i), v.e.At(j)
	v.e.Set(i, vj)
	v.e.Set(j, vi)
}

func (v *vectorByOrderedCompare) Less(i, j int) bool {
	return OrderedCompare(v.c, v.e.At(i), v.e.At(j)) < 0
}

// sortedCopy returns a copy of v, in ascending sorted order.
func (v *Vector) sortedCopy(c Context) *Vector {
	edit := v.edit()
	sort.Sort(&vectorByOrderedCompare{c, edit})
	return edit.Publish()
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

// catenate returns the concatenation v, x.
func (v *Vector) catenate(x *Vector) *Vector {
	edit := v.edit()
	for _, e := range x.All() {
		edit.Append(e)
	}
	return edit.Publish()
}
