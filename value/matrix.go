// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"
	"io"
	"math/bits"
	"slices"
	"sort"
	"strings"

	"robpike.io/ivy/config"
)

/*
    3 4 ⍴ ⍳12

 1  2  3  4
 5  6  7  8
 9 10 11 12
*/

type Matrix struct {
	shape []int
	data  *Vector
}

// Shape returns the shape of the matrix.
func (m *Matrix) Shape() []int {
	return m.shape
}

func (m *Matrix) Rank() int {
	return len(m.shape)
}

func (m *Matrix) shrink() Value {
	if len(m.shape) == 1 {
		return m.data
	}
	return m
}

// Data returns the data of the matrix as a vector.
func (m *Matrix) Data() *Vector {
	return m.data
}

// widths is a type that records the maximum widths of each column,
// then reports them back as needed. It handles two cases: If all
// widths are small, it always replies with the maximum width seen;
// otherwise it replies with the maximum for this column alone.
type widths struct {
	wid []int // Maximum width seen in each column.
	max int   // The maximum over all columns.
}

const widthThreshold = 5 // All widths below this -> regular grid.

// addColumn records the width for column i, either updating that
// column or adding a new one. i is never more than len(w.wid).
func (w *widths) addColumn(i, wid int) {
	switch {
	case i < len(w.wid):
		w.wid[i] = max(wid, w.wid[i])
	case i == len(w.wid):
		w.wid = append(w.wid, wid)
	default:
		Errorf("cannot happen: out of range in addColumn")
	}
	w.max = max(w.max, wid)
}

// column returns the width to use to display column i.
func (w *widths) column(i int) int {
	if w.max < widthThreshold {
		return w.max
	}
	return w.wid[i]
}

// columns returns the number of columns.
func (w *widths) columns() int {
	return len(w.wid)
}

// write2d formats the 2d grid elements into lines.
func (m *Matrix) write2d(elems [][]string, cols int, nested bool, w *widths) []string {
	var lines []string
	for row := range len(elems) / cols {
		if row > 0 && nested {
			lines = append(lines, "")
		}
		cells := elems[row*cols : (row+1)*cols]
		lines = append(lines, formatRow(cells, w)...)
	}
	return lines
}

func (m *Matrix) fprintf(c Context, w io.Writer, format string) {
	rank := len(m.shape)
	if rank == 0 || m.data.Len() == 0 {
		return
	}
	counters := make([]int, len(m.shape))
	verb := verbOf(format)
	printSpace := false
	for i, v := range m.data.All() {
		if printSpace {
			fmt.Fprint(w, " ")
		}
		formatOne(c, w, format, verb, v)
		printSpace = true
		for k := rank - 1; k >= 0; k-- {
			// Little-endian counter iterates the indexes.
			counters[k]++
			if counters[k] < m.shape[k] {
				break
			}
			// Each time a counter overflows, add a newline.
			// This puts 0 lines between rows, 1 between
			// each 2-d block, 2 between each 3-d block, etc.
			if i < m.data.Len()-1 {
				w.Write([]byte{'\n'})
				printSpace = false
			}
			counters[k] = 0
		}
	}
}

func (m *Matrix) String() string {
	return "(" + m.Sprint(debugConf) + ")"
}

func (m *Matrix) Sprint(conf *config.Config) string {
	return strings.Join(m.sprint(conf), "\n")
}

func (m *Matrix) sprint(conf *config.Config) []string {
	// If the matrix is mostly nested elements, space it out a bit more.
	numNested := 0
	for _, e := range m.data.All() {
		_, ok := e.(*Matrix)
		if ok && len(m.shape) > 1 {
			numNested++
		}
	}
	// Heuristic avoids spacing out matrices with few nested elements.
	nested := numNested >= m.data.Len()/2

	switch m.Rank() {
	case 0:
		Errorf("matrix is rank 0") // Can this ever happen?
		return nil
	case 1:
		return m.data.sprint(conf)
	case 2:
		ncols := m.shape[1]
		nrows := m.shape[0]
		if nrows == 0 || ncols == 0 {
			return nil
		}
		// If it's all chars, print it without padding or quotes.
		if m.data.AllChars() {
			var lines []string
			for i := 0; i < nrows; i++ {
				// TODO what about embedded newlines?
				lines = append(lines, NewVectorSeq(m.data.Slice(i*ncols, (i+1)*ncols)).Sprint(conf))
			}
			return lines
		}
		cells, width := m.data.cells(conf, ncols)
		return m.write2d(cells, ncols, nested, width)
	case 3:
		// As for 2d: print the vector elements, compute the
		// global width, and use that to print each 2d submatrix.
		n2d := m.shape[0] // number of 2d submatrices.
		nrows := m.shape[1]
		ncols := m.shape[2]
		size := m.ElemSize() // number of elems in each submatrix.

		// If it's all chars, print it without padding or quotes.
		if m.data.AllChars() {
			var lines []string
			start := 0
			for i := range n2d {
				if i > 0 {
					lines = append(lines, "")
				}
				for range nrows {
					lines = append(lines, NewVectorSeq(m.data.Slice(start, start+ncols)).sprint(conf)...)
					start += ncols
				}
			}
			return lines
		}

		cells, width := m.data.cells(conf, ncols)
		var lines []string
		for i := range n2d {
			if i > 0 {
				lines = append(lines, "")
			}
			m := Matrix{
				shape: m.shape[1:],
				// no data; write2d uses cells, not data
			}
			lines = append(lines, m.write2d(cells[i*size:(i+1)*size], ncols, nested, width)...)
		}
		return lines
	default:
		return m.higherDim(conf, "[", 0)
	}
}

func (m *Matrix) ProgString() string {
	// There is no such thing as a matrix in program listings.
	panic("matrix.ProgString - cannot happen")
}

func (m *Matrix) higherDim(conf *config.Config, prefix string, indentation int) []string {
	if m.Rank() <= 3 {
		return indent(indentation, m.sprint(conf))
	}
	dim := m.shape[0]
	rest := strings.Repeat(" *", m.Rank()-1)[1:]
	var lines []string
	for i := 0; i < dim; i++ {
		inner := Matrix{
			shape: m.shape[1:],
			data:  NewVectorSeq(m.data.Slice(i*m.ElemSize(), m.data.Len())),
		}
		if i > 0 {
			lines = append(lines, "")
		}
		innerPrefix := fmt.Sprintf("%s%d ", prefix, i+conf.Origin())
		lines = append(lines, fmt.Sprintf("%s%s]:", innerPrefix, rest))
		lines = append(lines, inner.higherDim(conf, innerPrefix, indentation+1)...)
	}
	return lines
}

// indent add indentation to each element in lines,
// returning a new slice of lines.
func indent(indentation int, lines []string) []string {
	var out []string
	for _, line := range lines {
		out = append(out, spaces(indentation)+line)
	}
	return out
}

// spaces returns 2*n space characters, maxing out at 2*10.
func spaces(n int) string {
	if n > 10 {
		n = 10
	}
	return blanks(2 * n)
}

// Size returns number of elements of the matrix.
// Given shape [a, b, c, ...] it is a*b*c*....
func (m *Matrix) Size() int {
	return size(m.shape)
}

// ElemSize returns the size of each top-level element of the matrix.
// Given shape [a, b, c, ...] it is b*c*....
func (m *Matrix) ElemSize() int {
	return size(m.shape[1:])
}

func size(shape []int) int {
	size := 1
	for _, i := range shape {
		hi, lo := bits.Mul(uint(size), uint(i))
		if int(lo) < 0 || hi != 0 {
			Errorf("matrix too large")
		}
		size = int(lo)
	}
	return size
}

// NewMatrix makes a new matrix. The number of elements must fit in an Int.
func NewMatrix(shape []int, data *Vector) *Matrix {
	// Check consistency and sanity.
	nelems := 0
	if len(shape) > 0 {
		// Can't use size function here: must avoid overflow.
		n := int64(shape[0])
		for i := 1; i < len(shape); i++ {
			n *= int64(shape[i])
			if n > maxInt {
				Errorf("matrix too large")
			}
		}
		nelems = int(n)
	}
	if nelems != data.Len() {
		Errorf("inconsistent shape (%d) and data size (%d) for new matrix", shape, data.Len())
	}
	return &Matrix{
		shape: shape,
		data:  data,
	}
}

func (m *Matrix) Eval(Context) Value {
	return m
}

func (m *Matrix) Inner() Value {
	return m
}

func (m *Matrix) toType(op string, conf *config.Config, which valueType) Value {
	switch which {
	case matrixType:
		return m
	}
	Errorf("%s: cannot convert matrix to %s", op, which)
	return nil
}

func (x *Matrix) sameShape(y *Matrix) {
	if !sameShape(x.shape, y.shape) {
		Errorf("shape mismatch: %s != %s", NewIntVector(x.shape...), NewIntVector(y.shape...))
	}
}

func sameShape(x, y []int) bool {
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

// reshape implements binary rho
// A⍴B: Array of shape A with data B
func reshape(A, B *Vector) Value {
	if B.Len() == 0 {
		// Peculiar APL definition of reshape of empty vector: Use fill values.
		B = NewIntVector(0)
	}
	if A.Len() == 0 {
		return NewVector()
	}
	nelems := Int(1)
	shape := make([]int, A.Len())
	for i := range A.All() {
		n, ok := A.At(i).Inner().(Int)
		if !ok || n < 0 || maxInt < n {
			Errorf("bad shape for rho: %s is not a small integer", A.At(i))
		}
		nelems *= n
		if nelems > maxInt {
			Errorf("rho has too many elements")
		}
		shape[i] = int(n)
	}
	t := newVectorEditor(int(nelems), nil)
	blen := B.Len()
	for i := 0; i < blen && i < int(nelems); i++ {
		t.Set(i, B.At(i))
	}
	// replicate as needed
	for i := blen; i < int(nelems); i++ {
		t.Set(i, t.At(i-blen))
	}
	v := t.Publish()
	if A.Len() == 1 {
		return v
	}
	return NewMatrix(shape, v)
}

// rotate returns a copy of v with elements rotated left by n.
// Rotation occurs on the rightmost axis.
func (m *Matrix) rotate(n int) Value {
	if m.Rank() == 0 {
		return &Matrix{}
	}
	elems := newVectorEditor(m.data.Len(), nil)
	dim := m.shape[m.Rank()-1]
	n %= dim
	if n < 0 {
		n += dim
	}
	pfor(true, dim, m.data.Len()/dim, func(lo, hi int) {
		for i := lo; i < hi; i++ {
			j := i * dim
			doRotate(elems, j, dim, m.data, j, n)
		}
	})
	return NewMatrix(m.shape, elems.Publish())
}

// vrotate returns a copy of v with elements rotated down by n.
// Rotation occurs on the leftmost axis.
func (m *Matrix) vrotate(n int) Value {
	if m.Rank() == 0 {
		return &Matrix{}
	}
	if m.Rank() == 1 {
		return m
	}

	elems := newVectorEditor(m.data.Len(), nil)
	dim := m.data.Len() / m.shape[0]

	n *= dim
	n %= m.data.Len()
	if n < 0 {
		n += m.data.Len()
	}

	pfor(true, dim, m.data.Len()/dim, func(lo, hi int) {
		for i := lo; i < hi; i++ {
			j := i * dim
			n := (n + j) % m.data.Len()
			for k := range dim {
				elems.Set(j+k, m.data.At(n+k))
			}
		}
	})

	return NewMatrix(m.shape, elems.Publish())
}

// transpose returns (as a new matrix) the transposition of the argument.
func (m *Matrix) transpose(c Context) *Matrix {
	// Fast version for common 2d case.
	if len(m.shape) == 2 {
		data := newVectorEditor(m.data.Len(), nil)
		xdim, ydim := m.shape[0], m.shape[1] // For new matrix.
		pfor(true, 1, data.Len(), func(lo, hi int) {
			nx := lo / ydim
			ny := lo % ydim
			for _, v := range m.data.Slice(lo, hi) {
				data.Set(ny*xdim+nx, v)
				ny++
				if ny >= ydim {
					nx++
					ny = 0
				}
			}
		})
		return NewMatrix([]int{ydim, xdim}, data.Publish())
	}
	nShape := newVectorEditor(len(m.shape), nil)
	origin := c.Config().Origin()
	for i := range nShape.Len() {
		nShape.Set(len(m.shape)-1-i, Int(i+origin))
	}
	return m.binaryTranspose(c, nShape.Publish())
}

// binaryTranspose returns the transposition of m specified by v,
// defined by (v transp m)[i] = m[i[v]] (i is in general an index vector).
// APL calls this operator the dyadic transpose.
func (m *Matrix) binaryTranspose(c Context, v *Vector) *Matrix {
	origin := c.Config().Origin()
	if v.Len() != m.Rank() {
		Errorf("transp: vector length %d != matrix rank %d", v.Len(), m.Rank())
	}

	// Extract old-to-new index mapping and determine rank.
	oldToNew := make([]int, v.Len())
	rank := -1
	for i := range v.All() {
		vi := v.intAt(i, "transp index")
		if vi < origin || vi >= origin+m.Rank() {
			Errorf("transp: out-of-range index %v", vi)
		}
		vi -= origin
		oldToNew[i] = vi
		if rank <= vi {
			rank = vi + 1
		}
	}

	// Determine shape of result.
	// Each dimension is the min of the old dimensions mapping to it.
	oldShape := m.shape
	shape := make([]int, rank)
	for i := range shape {
		shape[i] = -1
	}
	for oi, dim := range oldShape {
		if i := oldToNew[oi]; shape[i] == -1 || shape[i] > dim {
			shape[i] = dim
		}
	}
	sz := 1
	for i, dim := range shape {
		if dim == -1 {
			Errorf("transp: partial index: missing %v", i+origin)
		}
		sz *= dim
	}

	old := m.data
	data := newVectorEditor(sz, nil)
	pfor(true, 1, data.Len(), func(lo, hi int) {
		// Compute starting index
		index := make([]int, rank)
		i := lo
		for j := rank - 1; j >= 0; j-- {
			if shape[j] > 0 {
				index[j] = i % shape[j]
				i /= shape[j]
			}
		}
		for i := lo; i < hi; i++ {
			// Compute old index for this new entry.
			oi := 0
			for j := range v.All() {
				oi = oi*m.shape[j] + index[oldToNew[j]]
			}

			data.Set(i, old.At(oi))

			// Increment index.
			for j := rank - 1; j >= 0; j-- {
				if index[j]++; index[j] < shape[j] {
					break
				}
				index[j] = 0
			}
		}
	})

	return NewMatrix(shape, data.Publish())
}

// catenate returns the catenation x, y, along the last axis.
// It handles the following shape combinations:
//
//	(n ...), (...) -> (n+1 ...)  # list, elem
//	(...), (n ...) -> (n+1 ...)  # elem, list
//	(n ...), (m ...) -> (n+m ...)  # list, list
//	(1), (n ...) -> (n+1 ...)  # scalar (extended), list
//	(n ...), (1) -> (n+1 ...)  # list, scalar (extended)
func (x *Matrix) catenate(y *Matrix) *Matrix {
	if x.Rank() == 0 || y.Rank() == 0 {
		Errorf("rank 0 matrix for ,")
	}
	var shape []int
	data := newVectorEditor(0, nil)
	var nrows int
	setShape := func(m *Matrix, extra int) {
		shape = slices.Clone(m.shape)
		shape[len(shape)-1] += extra
		nrows = size(shape[:len(shape)-1])
	}
	copyElems := func(nLeft, advLeft, nRight, advRight int) {
		di, li, ri := 0, 0, 0
		for i := 0; i < nrows; i++ {
			for k := range nLeft {
				data.Set(di+k, x.data.At(li+k))
			}
			di += nLeft
			li += advLeft
			for k := range nRight {
				data.Set(di+k, y.data.At(ri+k))
			}
			di += nRight
			ri += advRight
		}
	}
	switch {
	default:
		Errorf("catenate shape mismatch: %v, %v", NewIntVector(x.shape...), NewIntVector(y.shape...))

	case x.Rank() == y.Rank() && sameShape(x.shape[:len(x.shape)-1], y.shape[:len(y.shape)-1]):
		// list, list
		setShape(x, y.shape[len(y.shape)-1])
		data.Resize(x.data.Len() + y.data.Len())
		xsize, ysize := x.shape[len(x.shape)-1], y.shape[len(y.shape)-1]
		copyElems(xsize, xsize, ysize, ysize)

	case x.Rank() == y.Rank()+1 && sameShape(x.shape[:len(x.shape)-1], y.shape):
		// list, elem
		setShape(x, 1)
		data.Resize(x.data.Len() + y.data.Len())
		xsize := x.shape[len(x.shape)-1]
		ysize := y.Size() / nrows
		copyElems(xsize, xsize, ysize, ysize)

	case x.Rank()+1 == y.Rank() && sameShape(x.shape, y.shape[:len(y.shape)-1]):
		// elem, list
		setShape(y, 1)
		data.Resize(y.data.Len() + x.data.Len())
		xsize := x.Size() / nrows
		ysize := y.shape[len(y.shape)-1]
		copyElems(xsize, xsize, ysize, ysize)

	case x.Rank() == 1 && x.shape[0] == 1 && y.Rank() > 1:
		// scalar extension, list
		setShape(y, 1)
		data.Resize(y.data.Len() + nrows)
		ysize := y.shape[len(y.shape)-1]
		copyElems(1, 0, ysize, ysize)

	case x.Rank() > 1 && y.Rank() == 1 && y.shape[0] == 1:
		// list, scalar extension
		setShape(x, 1)
		data.Resize(x.data.Len() + nrows)
		xsize := x.shape[len(x.shape)-1]
		copyElems(xsize, xsize, 1, 0)
	}
	return NewMatrix(shape, data.Publish())
}

// catenateFirst returns the catenation x, y, along the first axis.
func (x *Matrix) catenateFirst(y *Matrix) *Matrix {
	if x.Rank() == 0 || y.Rank() == 0 {
		Errorf("rank 0 matrix for ,%%")
	}
	var shape []int
	var data *vectorEditor
	switch {
	default:
		Errorf("catenateFirst shape mismatch: %v, %v", NewIntVector(x.shape...), NewIntVector(y.shape...))

	case x.Rank() == y.Rank() && sameShape(x.shape[1:], y.shape[1:]):
		// list, list
		shape = slices.Clone(x.shape)
		shape[0] = x.shape[0] + y.shape[0]

	case x.Rank() == y.Rank()+1 && sameShape(x.shape[1:], y.shape):
		// list, elem
		shape = slices.Clone(x.shape)
		shape[0]++

	case x.Rank()+1 == y.Rank() && sameShape(x.shape, y.shape[1:]):
		// elem, list
		shape = slices.Clone(y.shape)
		shape[0]++

	case x.Rank() == 1 && x.shape[0] == 1 && y.Rank() > 1:
		// scalar extension, list
		shape = slices.Clone(y.shape)
		shape[0]++
		a := x.data.At(0)
		data = newVectorEditor(0, nil)
		elem := y.ElemSize()
		for range elem {
			data.Append(a)
		}
		for _, e := range y.data.All() {
			data.Append(e)
		}

	case x.Rank() > 1 && y.Rank() == 1 && y.shape[0] == 1:
		// list, scalar extension
		shape = slices.Clone(x.shape)
		shape[0]++
		elem := x.ElemSize()
		b := y.data.At(0)
		data = x.data.edit()
		for range elem {
			data.Append(b)
		}
	}
	if data == nil {
		data = x.data.edit()
		for _, e := range y.data.All() {
			data.Append(e)
		}
	}
	return NewMatrix(shape, data.Publish())
}

// sel returns the selection of m according to v.
// The selection applies to the final axis.
func (m *Matrix) sel(c Context, v *Vector) *Matrix {
	// All lhs values must be small integers.
	if !v.AllInts() {
		Errorf("sel: left operand must be small integers")
	}

	var count int64
	for _, x := range v.All() {
		x := x.(Int)
		if x < 0 {
			count -= int64(x)
		} else {
			count += int64(x)
		}
	}
	if v.Len() != 1 && v.Len() != m.shape[len(m.shape)-1] {
		Errorf("sel: bad length %d for shape %s", v.Len(), NewIntVector(m.shape...))
	}
	if v.Len() == 1 {
		count *= int64(m.shape[len(m.shape)-1])
	}

	shape := slices.Clone(m.shape)
	shape[len(shape)-1] = int(count)

	for _, dim := range shape[:len(shape)-1] {
		count *= int64(dim)
	}
	if count > 1e8 {
		Errorf("sel: result too large: %d elements", count)
	}

	result := newVectorEditor(0, nil)
	for i, y := range m.data.All() {
		c := v.At(i % v.Len()).(Int)
		if c < 0 {
			c = -c
			y = zero
		}
		for ; c > 0; c-- {
			result.Append(y)
		}
	}

	return NewMatrix(shape, result.Publish())
}

// take returns v take m.
func (m *Matrix) take(c Context, v *Vector) *Matrix {
	if !v.AllInts() {
		Errorf("take: left operand must be small integers")
	}
	// Extend short vector to full rank using shape.
	if v.Len() > m.Rank() {
		// Rank mismatch, but if m is of unit size, we can just raise its rank.
		if m.Size() != 1 {
			Errorf("take: bad rank %d for shape %s", m.Rank(), v)
		}
		// Create a 1x1x1... matrix and use that as the argument.
		shape := make([]int, v.Len())
		for i := range shape {
			shape[i] = 1
		}
		m = NewMatrix(shape, m.data)
	}
	if v.Len() < m.Rank() {
		ext := v.edit()
		for ext.Len() < m.Rank() {
			ext.Append(Int(m.shape[ext.Len()]))
		}
		v = ext.Publish()
	}

	// Compute new shape.
	shape := make([]int, m.Rank())
	type pos struct {
		min, max int
	}
	// mBounds is the box, in m space, that we will be taking.
	mBounds := make([]pos, m.Rank())
	// origin is the location, in result space, of the upper left corner of the full m.
	origin := make([]int, m.Rank())
	count := int64(1) // Number of elements in result.
	for i, x := range v.All() {
		var mb pos
		var o int
		y := int(x.(Int))
		if y < 0 {
			y = -y
			mb.max = m.shape[i]
			mb.min = max(mb.max-y, 0)
			o = y - m.shape[i]
		} else {
			mb.min = 0
			mb.max = min(m.shape[i], y)
			o = 0
		}
		shape[i] = y
		count *= int64(y)
		mBounds[i] = mb
		origin[i] = o
	}
	if count > maxInt { // Do this before allocating!
		Errorf("take: result matrix too large")
	}

	// TODO Is there a faster way?
	fill := fillValue(m.data)
	rCoords := make([]int, len(shape)) // Matrix coordinates in result.
	result := newVectorEditor(int(count), nil)
	for i := range result.Len() {
		inside := true
		mi := 0
		// See if this location is inside the bounding box for m.
		// As we do this, calculate the vector index (mi) for m.
		for k, rc := range rCoords {
			mi *= m.shape[k]
			loc := rc - origin[k]
			if loc < mBounds[k].min || mBounds[k].max <= loc {
				inside = false
				break
			}
			mi += loc
		}
		if inside {
			result.Set(i, m.data.At(mi)) // TODO
		} else {
			result.Set(i, fill)
		}
		// Increment destination indexes.
		for k := len(rCoords) - 1; k >= 0; k-- {
			rCoords[k]++
			if rCoords[k] < shape[k] {
				break
			}
			rCoords[k] = 0
		}
	}
	return NewMatrix(shape, result.Publish())
}

// partition returns a vector of the subblocks of m, selected and grouped
// by the values in score. Subblocks with score 0 are ignored.
// Subblocks with non-zero score are included, grouped with boundaries
// at every point where the score exceeds the previous score.
func (m *Matrix) partition(scoreM *Matrix) Value {
	if len(scoreM.shape) != 1 {
		Errorf("part: left argument must be scalar or vector")
	}
	score := scoreM.data
	lastDim := m.shape[len(m.shape)-1]
	if scoreM.shape[0] == 1 {
		// Make a new score the width of the matrix.
		n := score.uintAt(0, "part: score")
		x := make([]Value, lastDim)
		for i := range x {
			x[i] = Int(n)
		}
		score = NewVector(x...)
	}
	if score.Len() != lastDim {
		Errorf("part: length mismatch")
	}
	res, dim := m.data.doPartition(score)
	newShape := append([]int{}, m.shape...)
	newShape[len(newShape)-1] = dim
	return NewMatrix(newShape, res)
}

// drop returns v drop m.
func (m *Matrix) drop(c Context, v *Vector) *Matrix {
	// Extend short vector to full rank using zeros.
	if v.Len() > m.Rank() {
		Errorf("take: argument %v too large for matrix with shape %s", v, NewIntVector(m.shape...))
	}
	if !v.AllInts() {
		Errorf("drop: left operand must be small integers")
	}
	if v.Len() < m.Rank() {
		ext := v.edit()
		for range m.Rank() - v.Len() {
			ext.Append(zero)
		}
		v = ext.Publish()
	}

	// All lhs values must be small integers in range for m's shape.
	// Convert to parameters for take.
	//	1 drop x = (1 - N) take x
	//	-1 drop x = (N - 1) take x
	take := v.edit()
	for i, x := range v.All() {
		x := int(x.(Int))
		switch {
		case x < -m.shape[i], x > m.shape[i]:
			take.Set(i, zero)
		case x >= 0:
			take.Set(i, Int(x-m.shape[i]))
		case x < 0:
			take.Set(i, Int(m.shape[i]+x))
		}
	}

	return m.take(c, take.Publish())
}

// split reduces the matrix by one dimension.
func (m *Matrix) split() Value {
	if len(m.shape) < 2 {
		// TODO?
		Errorf("cannot split rank %d matrix", len(m.shape))
	}
	// Matrix of vectors.
	shape, n := m.shape[:len(m.shape)-1], m.shape[len(m.shape)-1]
	mData := newVectorEditor(size(shape), nil)
	for i := range mData.Len() {
		mData.Set(i, NewVectorSeq(m.data.Slice(i*n, (i+1)*n)))
	}
	return NewMatrix(shape, mData.Publish()).shrink()
}

// mix builds a matrix from the elements of the nested matrix.
func (m *Matrix) mix(c Context) Value {
	// If it's all scalar, nothing to do.
	if m.data.allScalars() {
		return m
	}
	shape := []int{0}
	for _, e := range m.data.All() {
		switch e := e.(type) {
		default:
			if shape[len(shape)-1] == 0 {
				shape[len(shape)-1] = 1
			}
		case *Vector:
			if shape[len(shape)-1] < e.Len() {
				shape[len(shape)-1] = e.Len()
			}
		case *Matrix:
			for len(e.shape) > len(shape) {
				shape = append(shape, 0)
			}
			for i, s := range e.shape {
				if shape[i] < s {
					shape[i] = s
				}
			}
		}
	}
	data := newVectorEditor(0, nil)
	vshape := NewIntVector(shape...)
	for _, e := range m.data.All() {
		var nm *Matrix
		takeShape := make([]int, len(shape))
		for i := range takeShape {
			takeShape[i] = 1
		}
		switch e := e.(type) {
		default:
			nm = NewMatrix(takeShape, NewVector(e)).take(c, vshape)
		case *Vector:
			takeShape[len(takeShape)-1] = e.Len()
			nm = NewMatrix(takeShape, e).take(c, vshape)
		case *Matrix:
			offset := vshape.Len() - len(e.shape)
			same := offset == 0
			for i := range e.shape {
				if e.shape[i] != takeShape[offset+i] {
					same = false
				}
				takeShape[offset+i] = e.shape[i]
			}
			nm = e
			if !same {
				nm = NewMatrix(takeShape, e.data).take(c, vshape)
			}
		}
		for _, elem := range nm.data.All() {
			data.Append(elem)
		}
	}
	return NewMatrix(append(m.shape, shape...), data.Publish())
}

// grade returns as a Vector the indexes that sort the rows of m
// into increasing order.
func (m *Matrix) grade(c Context) *Vector {
	x := make([]int, m.shape[0])
	for i := range x {
		x[i] = i
	}
	v := m.data
	stride := v.Len() / m.shape[0]
	sort.Slice(x, func(i, j int) bool {
		i = x[i] * stride
		j = x[j] * stride
		for k := 0; k < stride; k++ {
			cmp := OrderedCompare(c, v.At(i+k), v.At(j+k))
			if cmp == 0 {
				continue
			}
			return cmp < 0
		}
		return false
	})
	origin := c.Config().Origin()
	for i := range x {
		x[i] += origin
	}
	return NewIntVector(x...)
}

// inverse returns the matrix inverse of m. Note: although the code forbids
// non-scalar elements, they actually "work", but they are probably more confusing
// than helpful:
//
//	x = 2 2 rho 1 2 3 4; x[1;1]=2 3; inv x
//	      (2 2/3)   (-1 -1/3)
//	  (-3/2 -1/2)     (1 1/2)
//	x+.*inv x
//	  (1 1) (0 0)
//	  (0 0) (1 1)
//	inv inv x # This one is clearly nuts.
//	  (2 3) (2 2)
//	  (3 3) (4 4)
//
// So they are forbidden.
func (m *Matrix) inverse(c Context) Value {
	const (
		nonInvertible = "inverse of non-invertible matrix"
		nonScalar     = "inverse of matrix with non-scalar element"
	)
	switch len(m.shape) {
	case 0:
		Errorf("inverse of empty matrix")
	case 1:
		return NewMatrix(m.shape, m.data.inverse(c).(*Vector))
	case 2:
		// OK
	case 3:
		Errorf("cannot compute inverse of matrix with dimension > 2")
	}
	dim := m.shape[0]
	if m.shape[1] != dim {
		Errorf("inverse of non-square matrix")
	}

	// Gaussian elimination.
	// First we build a double-wide matrix, t,  by appending the identity matrix.
	t := make([][]Value, dim)
	for i := range t {
		t[i] = make([]Value, 2*dim)
	}
	i := 0
	for y := 0; y < dim; y++ {
		row := t[y]
		for x := 0; x < dim; x++ {
			row[x] = m.data.At(i)
			i++
			if x%dim == y {
				row[dim+x] = one
			} else {
				row[dim+x] = zero
			}
		}
	}

	// Convert left half to the identity matrix using whole-row operations.
	for x := 0; x < dim; x++ {
		for y := 0; y < dim; y++ {
			thisRow := t[y]
			val := thisRow[x]
			if !IsScalarType(val) {
				Errorf(nonScalar)
			}
			if y == x {
				if isZero(val) {
					Errorf(nonInvertible)
				}
				// This is the diagonal. We want a one here.
				scale := c.EvalUnary("/", val) // Invert so we can multiply in loop.
				for i := 0; i < 2*dim; i++ {
					if i == x {
						thisRow[i] = one
						continue
					}
					thisRow[i] = c.EvalBinary(thisRow[i], "*", scale)
				}
				continue
			}
			// This is off the diagonal. We want a zero here, which we can
			// get by subtracting a scaled row that is already zero to the left.
			if isZero(t[y][x]) {
				continue
			}
			// Find a row with a non-zero element in this column.
			target := -1
			for row := x; row < dim; row++ {
				if row != y && !isZero(t[row][x]) {
					target = row
					break
				}
			}
			if target < 0 {
				Errorf(nonInvertible)
			}
			// Subtract scaled target row to get a zero.
			row := t[target]
			ratio := c.EvalBinary(thisRow[x], "/", row[x])
			for i := 0; i < 2*dim; i++ {
				if i == x {
					thisRow[i] = zero
					continue
				}
				thisRow[i] = c.EvalBinary(thisRow[i], "-", c.EvalBinary(ratio, "*", row[i]))
			}
		}
	}
	// Now extract the right hand side of the working area.
	data := newVectorEditor(0, nil)
	for _, row := range t {
		data.Append(row[dim:]...)
	}
	return NewMatrix(m.shape, data.Publish())
}
