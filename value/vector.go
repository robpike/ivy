// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"iter"
	"math"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

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
	return "(" + v.Sprint(debugContext) + ")"
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

func (v *Vector) Rank() int {
	return 1
}

func (v *Vector) ProgString() string {
	// There is no such thing as a vector in program listings; they
	// are represented as a VectorExpr.
	// Use DebugProgString if this panic happens.
	panic("vector.ProgString - cannot happen")
}

// Sprint returns the formatting of v according to conf.
func (v *Vector) Sprint(c Context) string {
	return strings.Join(v.sprint(c), "\n")
}

func (v *Vector) sprint(c Context) []string {
	if v.AllChars() {
		b := strings.Builder{}
		for _, i := range v.All() {
			b.WriteRune(rune(i.Inner().(Char)))
		}
		return strings.Split(b.String(), "\n")
	}
	cells, width := v.cells(c, v.Len())
	width.max = 1e9
	return formatRow(cells, width)
}

// cells returns the content of each element in v
// as a cell, which is a []string giving the lines of output.
func (v *Vector) cells(c Context, ncol int) ([][]string, *widths) {
	var w widths
	var out [][]string
	for i, elem := range v.All() {
		var cell []string
		switch elem := elem.(type) {
		case *Vector:
			if elem.AllChars() && elem.Len() > 0 {
				// TODO what about newlines
				cell = elem.sprint(c)
			} else {
				cell = drawBox(elem.sprint(c), vectorCorners)
			}
		case *Matrix:
			cell = drawBox(elem.sprint(c), matrixCorners)
		default:
			cell = strings.Split(elem.Sprint(c), "\n")
		}
		for _, line := range cell {
			w.addColumn(c, i%ncol, utf8.RuneCountInString(line))
		}
		out = append(out, cell)
	}
	return out, &w
}

// formatRow formats a row of cells, aligning to the widths in width.
// It returns the lines of output for that row.
func formatRow(cells [][]string, width *widths) []string {
	// If there are any heading corners in cells, place them all on the first line
	// and align all actual content starting on the second line.
	heading := false
	head := make([]int, len(cells))
	for col, cell := range cells {
		if len(cell) > 0 && isHead(cell[0]) {
			heading = true
			head[col] = 1
		}
	}
	height := 1
	for col, cell := range cells {
		height = max(height, len(cell)-head[col])
	}

	// Concatenate each line of each cell into a line of the row.
	var lines []string
	if heading {
		var b strings.Builder
		blank := 0
		for col, cell := range cells {
			if head[col] == 0 {
				blank += width.column(col) + 1
				continue
			}
			s := cell[0]
			b.WriteString(blanks(blank + width.column(col) - utf8.RuneCountInString(s)))
			b.WriteString(s)
			blank = 1
		}
		lines = append(lines, b.String())
	}
	for h := range height {
		var b strings.Builder
		blank := 0
		for col, cell := range cells {
			s := ""
			if h+head[col] < len(cell) {
				s = cell[h+head[col]]
			}
			if s == "" {
				blank += width.column(col) + 1
				continue
			}
			b.WriteString(blanks(blank + width.column(col) - utf8.RuneCountInString(s)))
			b.WriteString(s)
			blank = 1
		}
		lines = append(lines, b.String())
	}
	return lines
}

func isHead(line string) bool {
	return strings.Trim(line, " ╭╮┌┐") == "" && strings.ContainsAny(line, "╭╮┌┐")
}

func isTail(line string) bool {
	return strings.Trim(line, " ╰╯└┘") == "" && strings.ContainsAny(line, "╰╯└┘")
}

var (
	vectorCorners = []string{`(`, `)`, `╭`, `╮`, `╰`, `╯`}
	matrixCorners = []string{`[`, `]`, `┌`, `┐`, `└`, `┘`}
)

func drawBox(lines, corners []string) []string {
	if corners == nil {
		return lines
	}
	switch len(lines) {
	case 0:
		return []string{corners[0] + corners[1]}
	case 1:
		if corners[0] == "(" {
			// Common case: one-line vector uses ordinary parens.
			return []string{corners[0] + lines[0] + corners[1]}
		}
	}
	wid := 0
	for _, line := range lines {
		wid = max(wid, utf8.RuneCountInString(line))
	}

	var head, tail string
	if len(lines) >= 1 && isHead(lines[0]) {
		// Add corners to existing head line to limit nested vertical expansion.
		line := lines[0]
		head = line + blanks(wid-utf8.RuneCountInString(line))
		lines = lines[1:]
	} else {
		// Introduce new head line.
		head = blanks(wid)
	}
	if len(lines) >= 1 && isTail(lines[len(lines)-1]) {
		// Add corners to existing tail line to limit nested vertical expansion.
		line := lines[len(lines)-1]
		tail = line + blanks(wid-utf8.RuneCountInString(line))
		lines = lines[:len(lines)-1]
	} else {
		// Introduce new tail line.
		tail = blanks(wid)
	}

	var boxed []string
	boxed = append(boxed, corners[2]+head+corners[3])
	for _, line := range lines {
		boxed = append(boxed, "│"+line+blanks(wid-utf8.RuneCountInString(line))+"│")
	}
	boxed = append(boxed, corners[4]+tail+corners[5])
	return boxed
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
func fillValue(c Context, v *Vector) Value {
	if v.Len() == 0 {
		return zero
	}
	var fill Value = zero
	if v.AllChars() {
		fill = Char(' ')
	}
	first := v.At(0)
	if IsScalarType(c, first) {
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
		return NewMatrix(c, v.shape, newVectorEditor(v.data.Len(), fill).Publish())
	}
	return zero
}

// fillValue returns a zero or a space as the appropriate fill type for the vector
func (v *Vector) fillValue(c Context) Value {
	return fillValue(c, v)
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
func (v *Vector) allScalars(c Context) bool {
	for _, x := range v.All() {
		if !IsScalarType(c, x) {
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

func (v *Vector) toType(op string, c Context, which valueType) Value {
	switch which {
	case vectorType:
		return v
	case matrixType:
		return NewMatrix(c, []int{v.Len()}, v)
	}
	c.Errorf("%s: cannot convert vector to %s", op, which)
	return nil
}

func (v *Vector) sameLength(c Context, x *Vector) {
	if v.Len() != x.Len() {
		c.Errorf("length mismatch: %d %d", v.Len(), x.Len())
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
func (v *Vector) sel(c Context, n *Vector, elemCount int) *Vector {
	if n.Len() != 1 && n.Len() != elemCount {
		c.Errorf("sel length mismatch")
	}
	result := newVectorEditor(0, nil)
	for i := range v.Len() {
		count := n.intAt(c, i%n.Len(), "sel count")
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
func (v *Vector) uintAt(c Context, i int, msg string) int {
	n, ok := v.At(i).(Int)
	if !ok || n < 0 {
		c.Errorf("%s must be a non-negative integer: %s", msg, v.At(i))
	}
	return int(n)
}

// intAt returns the ith element of v, which must be an Int.
// The vector is known to be long enough.
func (v *Vector) intAt(c Context, i int, msg string) int {
	n, ok := v.At(i).(Int)
	if !ok {
		c.Errorf("%s must be a small integer: %s", msg, v.At(i))
	}
	return int(n)
}

// partition returns a vector of the elements of v, selected and grouped
// by the values in score. Elements with score 0 are ignored.
// Elements with non-zero score are included, grouped with boundaries
// at every point where the score exceeds the previous score.
func (v *Vector) partition(c Context, score *Vector) Value {
	if score.Len() != v.Len() {
		c.Errorf("part: length mismatch")
	}
	res, _ := v.doPartition(c, score)
	return res
}

// doPartition iterates along the vector to do the partitioning. It is called from
// vector and matrix partitioning code, which use different length checks. The
// integer returned is the width (last dimension) to use when partitioning a
// matrix.
func (v *Vector) doPartition(c Context, score *Vector) (*Vector, int) {
	accum := newVectorEditor(0, nil)
	result := newVectorEditor(0, nil)
	dim := -1
	for i, sc, prev := 0, 0, 0; i < v.Len(); i, prev = i+1, sc {
		j := i % score.Len()
		sc = score.uintAt(c, j, "part: score")
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
		c.Errorf("inverse of empty vector")
	}
	if v.Len() == 1 {
		return inverse(c, v)
	}
	// We could do this evaluation using "conj" and "+.*" but avoid the overhead.
	conj := v.edit()
	for i, x := range conj.All() {
		if !IsScalarType(c, x) {
			c.Errorf("inverse of vector with non-scalar element")
		}
		if cmplx, ok := x.(Complex); ok {
			conj.Set(i, NewComplex(c, cmplx.real, c.EvalUnary("-", cmplx.imag)).shrink())
		}
	}
	mag := Value(zero)
	for i, x := range v.All() {
		mag = c.EvalBinary(mag, "+", c.EvalBinary(x, "*", conj.At(i)))
	}
	if isZero(mag) {
		c.Errorf("inverse of zero vector")
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
