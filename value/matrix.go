// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"bytes"
	"fmt"
	"io"
	"math/bits"
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
	data  Vector
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
		return NewVector(m.data)
	}
	return m
}

// Data returns the data of the matrix as a vector.
func (m *Matrix) Data() Vector {
	return m.data
}

func (m *Matrix) Copy() Value {
	shape := make([]int, len(m.shape))
	data := make([]Value, len(m.data))
	copy(shape, m.shape)
	copy(data, m.data)
	return &Matrix{
		shape: shape,
		data:  data,
	}
}

// elemStrs returns the formatted elements of the matrix and the width of the widest element.
// Each element is represented by a slice of lines, that is, the return value is indexed by
// [elem][line].
func (m *Matrix) elemStrs(conf *config.Config) ([][]string, int) {
	// Format the matrix as a vector, and then in write2d we rearrange the pieces.
	// In the formatting, there's no need for spacing the elements as we'll cut
	// them apart ourselves using column information. Spaces will be added
	// when needed in write2d.
	v := NewVector(m.data)
	lines, cols := v.multiLineSprint(conf, v.allScalars(), v.AllChars(), !withSpaces, !trimTrailingSpace)
	strs := make([][]string, len(m.data))
	wid := 0
	for i := range m.data {
		rows := make([]string, len(lines))
		for j, line := range lines {
			if i == 0 {
				rows[j] = line[:cols[0]]
			} else {
				rows[j] = line[cols[i-1]:cols[i]]
			}
		}
		if len(rows[0]) > wid {
			wid = len(rows[0])
		}
		strs[i] = rows
	}
	return strs, wid
}

// write2d prints the 2d matrix m into the buffer.
// elems is a slice (of slices) of already-printed values.
// The receiver provides only the shape of the matrix.
func (m *Matrix) write2d(b *bytes.Buffer, elems [][]string, width int) {
	nrows := m.shape[0]
	ncols := m.shape[1]
	index := 0
	for row := 0; row < nrows; row++ {
		if row > 0 {
			b.WriteByte('\n')
		}
		// Don't print the line if it has no content.
		nonBlankLine := 0
		for col := 0; col < ncols; col++ {
			strs := elems[index+col]
			for line := nonBlankLine; line < len(strs); line++ {
				for _, r := range strs[line] {
					if r != ' ' {
						nonBlankLine = line
						break
					}
				}
			}
		}
		for line := 0; line < nonBlankLine+1; line++ {
			if line > 0 {
				b.WriteByte('\n')
			}
			for col := 0; col < ncols; col++ {
				str := elems[index+col][line]
				b.WriteString(blanks(width - len(str)))
				b.WriteString(str)
				if (col+1)%ncols != 0 {
					b.WriteString(" ")
				}
			}
		}
		index += ncols
	}
}

func (m *Matrix) fprintf(c Context, w io.Writer, format string) {
	rank := len(m.shape)
	if rank == 0 || len(m.data) == 0 {
		return
	}
	counters := make([]int, len(m.shape))
	verb := verbOf(format)
	printSpace := false
	for i, v := range m.data {
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
			if i < len(m.data)-1 {
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
	var b bytes.Buffer
	switch m.Rank() {
	case 0:
		Errorf("matrix is rank 0") // Can this ever happen?
	case 1:
		return m.data.Sprint(conf)
	case 2:
		nrows := m.shape[0]
		ncols := m.shape[1]
		if nrows == 0 || ncols == 0 {
			return ""
		}
		// If it's all chars, print it without padding or quotes.
		if m.data.AllChars() {
			for i := 0; i < nrows; i++ {
				if i > 0 {
					b.WriteByte('\n')
				}
				fmt.Fprintf(&b, "%s", m.data[i*ncols:(i+1)*ncols].Sprint(conf))
			}
			break
		}
		strs, wid := m.elemStrs(conf)
		m.write2d(&b, strs, wid)
	case 3:
		// If it's all chars, print it without padding or quotes.
		if m.data.AllChars() {
			nelems := m.shape[0]
			ElemSize := m.ElemSize()
			index := 0
			for i := 0; i < nelems; i++ {
				if i > 0 {
					b.WriteString("\n\n")
				}
				fmt.Fprintf(&b, "%s", NewMatrix(m.shape[1:], m.data[index:index+ElemSize]).Sprint(conf))
				index += ElemSize
			}
			break
		}
		// As for 2d: print the vector elements, compute the
		// global width, and use that to print each 2d submatrix.
		n2d := m.shape[0]    // number of 2d submatrices.
		size := m.ElemSize() // number of elems in each submatrix.
		strs, wid := m.elemStrs(conf)
		start := 0
		for i := 0; i < n2d; i++ {
			if i > 0 {
				b.WriteString("\n\n")
			}
			m := Matrix{
				shape: m.shape[1:],
				data:  m.data[start : start+size],
			}
			m.write2d(&b, strs[start:start+size], wid)
			start += size
		}
	default:
		return m.higherDim(conf, "[", 0)
	}
	return b.String()
}

func (m *Matrix) ProgString() string {
	// There is no such thing as a matrix in program listings.
	panic("matrix.ProgString - cannot happen")
}

func (m *Matrix) higherDim(conf *config.Config, prefix string, indentation int) string {
	if m.Rank() <= 3 {
		return indent(indentation, "%s", m.Sprint(conf))
	}
	dim := m.shape[0]
	rest := strings.Repeat(" *", m.Rank()-1)[1:]
	var b bytes.Buffer
	for i := 0; i < dim; i++ {
		inner := Matrix{
			shape: m.shape[1:],
			data:  m.data[i*m.ElemSize():],
		}
		if i > 0 {
			b.WriteString("\n\n")
		}
		innerPrefix := fmt.Sprintf("%s%d ", prefix, i+conf.Origin())
		b.WriteString(indent(indentation, "%s%s]:\n", innerPrefix, rest))
		b.WriteString(inner.higherDim(conf, innerPrefix, indentation+1))
	}
	return b.String()
}

// indent prints the args, indenting each line by the specified amount.
func indent(indentation int, format string, args ...interface{}) string {
	s := fmt.Sprintf(format, args...)
	if indentation == 0 {
		return s
	}
	var b bytes.Buffer
	lines := strings.SplitAfter(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for _, line := range lines {
		if len(line) > 0 {
			b.WriteString(spaces(indentation))
		}
		b.WriteString(line)
	}
	return b.String()
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
func NewMatrix(shape []int, data []Value) *Matrix {
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
	if nelems != len(data) {
		Errorf("inconsistent shape (%d) and data size (%d) for new matrix", shape, len(data))
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
func reshape(A, B Vector) Value {
	if len(B) == 0 {
		// Peculiar APL definition of reshape of empty vector: Use fill values.
		B = NewIntVector(0)
	}
	if len(A) == 0 {
		return Vector{}
	}
	nelems := one
	shape := make([]int, len(A))
	for i := range A {
		n, ok := A[i].Inner().(Int)
		if !ok || n < 0 || maxInt < n {
			Errorf("bad shape for rho: %s is not a small integer", A[i])
		}
		nelems *= n
		if nelems > maxInt {
			Errorf("rho has too many elements")
		}
		shape[i] = int(n)
	}
	values := make([]Value, nelems)
	n := copy(values, B)
	// replicate as needed by doubling in values.
	for n < len(values) {
		n += copy(values[n:], values[:n])
	}
	if len(A) == 1 {
		return NewVector(values)
	}
	return NewMatrix(shape, NewVector(values))
}

// rotate returns a copy of v with elements rotated left by n.
// Rotation occurs on the rightmost axis.
func (m *Matrix) rotate(n int) Value {
	if m.Rank() == 0 {
		return &Matrix{}
	}
	elems := make([]Value, len(m.data))
	dim := m.shape[m.Rank()-1]
	n %= dim
	if n < 0 {
		n += dim
	}
	pfor(true, dim, len(m.data)/dim, func(lo, hi int) {
		for i := lo; i < hi; i++ {
			j := i * dim
			doRotate(elems[j:j+dim], m.data[j:j+dim], n)
		}
	})
	return NewMatrix(m.shape, elems)
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

	elems := make([]Value, len(m.data))
	dim := len(m.data) / m.shape[0]

	n *= dim
	n %= len(m.data)
	if n < 0 {
		n += len(m.data)
	}

	pfor(true, dim, len(m.data)/dim, func(lo, hi int) {
		for i := lo; i < hi; i++ {
			j := i * dim
			n := (n + j) % len(m.data)
			copy(elems[j:j+dim], m.data[n:n+dim])
		}
	})

	return NewMatrix(m.shape, elems)
}

// transpose returns (as a new matrix) the transposition of the argument.
func (m *Matrix) transpose(c Context) *Matrix {
	// Fast version for common 2d case.
	if len(m.shape) == 2 {
		data := make([]Value, len(m.data))
		xdim, ydim := m.shape[0], m.shape[1] // For new matrix.
		pfor(true, 1, len(data), func(lo, hi int) {
			nx := lo / ydim
			ny := lo % ydim
			for _, v := range m.data[lo:hi] {
				data[ny*xdim+nx] = v
				ny++
				if ny >= ydim {
					nx++
					ny = 0
				}
			}
		})
		return NewMatrix([]int{ydim, xdim}, data)
	}
	nShape := make(Vector, len(m.shape))
	origin := c.Config().Origin()
	for i := range nShape {
		nShape[len(nShape)-1-i] = Int(i + origin)
	}
	return m.binaryTranspose(c, nShape)
}

// binaryTranspose returns the transposition of m specified by v,
// defined by (v transp m)[i] = m[i[v]] (i is in general an index vector).
// APL calls this operator the dyadic transpose.
func (m *Matrix) binaryTranspose(c Context, v Vector) *Matrix {
	origin := c.Config().Origin()
	if len(v) != m.Rank() {
		Errorf("transp: vector length %d != matrix rank %d", len(v), m.Rank())
	}

	// Extract old-to-new index mapping and determine rank.
	oldToNew := make([]int, len(v))
	rank := -1
	for i := range v {
		vi, ok := v[i].(Int)
		if !ok {
			Errorf("transp: non-int index %v", v[i])
		}
		if vi < Int(origin) || vi >= Int(origin+m.Rank()) {
			Errorf("transp: out-of-range index %v", vi)
		}
		vi -= Int(origin)
		oldToNew[i] = int(vi)
		if rank <= int(vi) {
			rank = int(vi) + 1
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
	data := make([]Value, sz)
	pfor(true, 1, len(data), func(lo, hi int) {
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
			for j := range v {
				oi = oi*m.shape[j] + index[oldToNew[j]]
			}

			data[i] = old[oi]

			// Increment index.
			for j := rank - 1; j >= 0; j-- {
				if index[j]++; index[j] < shape[j] {
					break
				}
				index[j] = 0
			}
		}
	})

	return NewMatrix(shape, data)
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
	var data Vector
	var nrows int
	setShape := func(m *Matrix, extra int) {
		shape = make([]int, m.Rank())
		copy(shape, m.shape)
		shape[len(shape)-1] += extra
		nrows = size(shape[:len(shape)-1])
	}
	copyElems := func(nLeft, advLeft, nRight, advRight int) {
		di, li, ri := 0, 0, 0
		for i := 0; i < nrows; i++ {
			copy(data[di:di+nLeft], x.data[li:li+nLeft])
			di += nLeft
			li += advLeft
			copy(data[di:di+nRight], y.data[ri:ri+nRight])
			di += nRight
			ri += advRight
		}
	}
	switch {
	default:
		Errorf("catenate shape mismatch: %d, %d", NewIntVector(x.shape...), NewIntVector(y.shape...))

	case x.Rank() == y.Rank() && sameShape(x.shape[:len(x.shape)-1], y.shape[:len(y.shape)-1]):
		// list, list
		setShape(x, y.shape[len(y.shape)-1])
		data = make(Vector, len(x.data)+len(y.data))
		xsize, ysize := x.ElemSize(), y.ElemSize()
		copyElems(xsize, xsize, ysize, ysize)

	case x.Rank() == y.Rank()+1 && sameShape(x.shape[:len(x.shape)-1], y.shape):
		// list, elem
		setShape(x, 1)
		data = make(Vector, len(x.data)+len(y.data))
		xsize := x.shape[len(x.shape)-1]
		ysize := y.Size() / nrows
		copyElems(xsize, xsize, ysize, ysize)

	case x.Rank()+1 == y.Rank() && sameShape(x.shape, y.shape[:len(y.shape)-1]):
		// elem, list
		setShape(y, 1)
		data = make(Vector, len(y.data)+len(x.data))
		xsize := x.Size() / nrows
		ysize := y.shape[len(y.shape)-1]
		copyElems(xsize, xsize, ysize, ysize)

	case x.Rank() == 1 && x.shape[0] == 1 && y.Rank() > 1:
		// scalar extension, list
		setShape(y, 1)
		data = make(Vector, len(y.data)+nrows)
		ysize := y.shape[len(y.shape)-1]
		copyElems(1, 0, ysize, ysize)

	case x.Rank() > 1 && y.Rank() == 1 && y.shape[0] == 1:
		// list, scalar extension
		setShape(x, 1)
		data = make(Vector, len(x.data)+nrows)
		xsize := x.shape[len(x.shape)-1]
		copyElems(xsize, xsize, 1, 0)
	}
	return NewMatrix(shape, data)
}

// catenateFirst returns the catenation x, y, along the first axis.
func (x *Matrix) catenateFirst(y *Matrix) *Matrix {
	if x.Rank() == 0 || y.Rank() == 0 {
		Errorf("rank 0 matrix for ,%%")
	}
	var shape []int
	var data Vector
	switch {
	default:
		Errorf("catenateFirst shape mismatch: %d, %d", NewIntVector(x.shape...), NewIntVector(y.shape...))

	case x.Rank() == y.Rank() && sameShape(x.shape[1:], y.shape[1:]):
		// list, list
		shape = make([]int, x.Rank())
		copy(shape, x.shape)
		shape[0] = x.shape[0] + y.shape[0]

	case x.Rank() == y.Rank()+1 && sameShape(x.shape[1:], y.shape):
		// list, elem
		shape = make([]int, x.Rank())
		copy(shape, x.shape)
		shape[0]++

	case x.Rank()+1 == y.Rank() && sameShape(x.shape, y.shape[1:]):
		// elem, list
		shape = make([]int, y.Rank())
		copy(shape, y.shape)
		shape[0]++

	case x.Rank() == 1 && x.shape[0] == 1 && y.Rank() > 1:
		// scalar extension, list
		shape = make([]int, y.Rank())
		copy(shape, y.shape)
		shape[0]++
		elem := y.ElemSize()
		a := x.data[0]
		data = make(Vector, elem+len(y.data))
		for i := 0; i < elem; i++ {
			data[i] = a
		}
		copy(data[elem:], y.data)

	case x.Rank() > 1 && y.Rank() == 1 && y.shape[0] == 1:
		// list, scalar extension
		shape = make([]int, x.Rank())
		copy(shape, x.shape)
		shape[0]++
		elem := x.ElemSize()
		b := y.data[0]
		data = make(Vector, elem+len(x.data))
		copy(data, x.data)
		ext := data[len(x.data):]
		for i := 0; i < elem; i++ {
			ext[i] = b
		}
	}
	if data == nil {
		data = make(Vector, len(x.data)+len(y.data))
		copy(data, x.data)
		copy(data[len(x.data):], y.data)
	}
	return NewMatrix(shape, data)
}

// sel returns the selection of m according to v.
// The selection applies to the final axis.
func (m *Matrix) sel(c Context, v Vector) *Matrix {
	// All lhs values must be small integers.
	if !v.AllInts() {
		Errorf("sel: left operand must be small integers")
	}

	var count int64
	for _, x := range v {
		x := x.(Int)
		if x < 0 {
			count -= int64(x)
		} else {
			count += int64(x)
		}
	}
	if len(v) != 1 && len(v) != m.shape[len(m.shape)-1] {
		Errorf("sel: bad length %d for shape %s", len(v), NewIntVector(m.shape...))
	}
	if len(v) == 1 {
		count *= int64(m.shape[len(m.shape)-1])
	}

	shape := make([]int, len(m.shape))
	copy(shape, m.shape)
	shape[len(shape)-1] = int(count)

	for _, dim := range shape[:len(shape)-1] {
		count *= int64(dim)
	}
	if count > 1e8 {
		Errorf("sel: result too large: %d elements", count)
	}

	result := make(Vector, 0, count)
	for i, y := range m.data {
		c := v[i%len(v)].(Int)
		if c < 0 {
			c = -c
			y = zero
		}
		for ; c > 0; c-- {
			result = append(result, y)
		}
	}

	return NewMatrix(shape, result)
}

// take returns v take m.
func (m *Matrix) take(c Context, v Vector) *Matrix {
	if !v.AllInts() {
		Errorf("take: left operand must be small integers")
	}
	// Extend short vector to full rank using shape.
	if len(v) > m.Rank() {
		// Rank mismatch, but if m is of unit size, we can just raise its rank.
		if m.Size() != 1 {
			Errorf("take: bad rank %d for shape %s", m.Rank(), v)
		}
		// Create a 1x1x1... matrix and use that as the argument.
		shape := make([]int, len(v))
		for i := range shape {
			shape[i] = 1
		}
		m = NewMatrix(shape, m.data)
	}
	if len(v) < m.Rank() {
		ext := make(Vector, m.Rank())
		copy(ext, v)
		for i := len(v); i < m.Rank(); i++ {
			ext[i] = Int(m.shape[i])
		}
		v = ext
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
	for i, x := range v {
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
	result := make(Vector, count, count)
	for i := range result {
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
			result[i] = m.data[mi] // TODO
		} else {
			result[i] = fill
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
	return NewMatrix(shape, result)
}

// drop returns v drop m.
func (m *Matrix) drop(c Context, v Vector) *Matrix {
	// Extend short vector to full rank using zeros.
	if len(v) > m.Rank() {
		Errorf("take: argument %v too large for matrix with shape %s", v, NewIntVector(m.shape...))
	}
	if !v.AllInts() {
		Errorf("drop: left operand must be small integers")
	}
	if len(v) < m.Rank() {
		ext := make(Vector, m.Rank())
		copy(ext, v)
		for i := len(v); i < m.Rank(); i++ {
			ext[i] = zero
		}
		v = ext
	}

	// All lhs values must be small integers in range for m's shape.
	// Convert to parameters for take.
	//	1 drop x = (1 - N) take x
	//	-1 drop x = (N - 1) take x
	take := make(Vector, len(v))
	for i, x := range v {
		x := int(x.(Int))
		switch {
		case x < -m.shape[i], x > m.shape[i]:
			take[i] = zero
		case x >= 0:
			take[i] = Int(x - m.shape[i])
		case x < 0:
			take[i] = Int(m.shape[i] + x)
		}
	}

	return m.take(c, take)
}

// split reduces the matrix by one dimension.
func (m *Matrix) split() Value {
	if len(m.shape) < 2 {
		// TODO?
		Errorf("cannot split rank %d matrix", len(m.shape))
	}
	// Matrix of vectors.
	n := m.shape[len(m.shape)-1]
	mData := make([]Value, 0, size(m.shape[:len(m.shape)-1]))
	for i := 0; i < len(m.data); i += n {
		mData = append(mData, NewVector(m.data[i:i+n]))
	}
	return NewMatrix(m.shape[:len(m.shape)-1], mData).shrink()
}

// mix builds a matrix from the elements of the nested matrix.
func (m *Matrix) mix(c Context) Value {
	// If it's all scalar, nothing to do.
	if allScalars(m.data) {
		return m.Copy()
	}
	shape := []int{0}
	for _, e := range m.data {
		switch e := e.(type) {
		default:
			if shape[len(shape)-1] == 0 {
				shape[len(shape)-1] = 1
			}
		case Vector:
			if shape[len(shape)-1] < len(e) {
				shape[len(shape)-1] = len(e)
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
	var data []Value
	vshape := NewIntVector(shape...)
	for _, e := range m.data {
		var nm *Matrix
		takeShape := make([]int, len(shape))
		for i := range takeShape {
			takeShape[i] = 1
		}
		switch e := e.(type) {
		default:
			nm = NewMatrix(takeShape, []Value{e}).take(c, vshape)
		case Vector:
			takeShape[len(takeShape)-1] = len(e)
			nm = NewMatrix(takeShape, e).take(c, vshape)
		case *Matrix:
			offset := len(vshape) - len(e.shape)
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
		data = append(data, nm.data...)
	}
	return NewMatrix(append(m.shape, shape...), data)
}

// grade returns as a Vector the indexes that sort the rows of m
// into increasing order.
func (m *Matrix) grade(c Context) Vector {
	x := make([]int, m.shape[0])
	for i := range x {
		x[i] = i
	}
	v := m.data
	stride := len(v) / m.shape[0]
	sort.Slice(x, func(i, j int) bool {
		i = x[i] * stride
		j = x[j] * stride
		for k := 0; k < stride; k++ {
			if toBool(c.EvalBinary(v[i+k], "==", v[j+k])) {
				continue
			}
			return toBool(c.EvalBinary(v[i+k], "<", v[j+k]))
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
//	 x = 2 2 rho 1 2 3 4; x[1;1]=2 3; inv x
//	    (2 2/3)   (-1 -1/3)
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
		return NewMatrix(m.shape, NewVector(m.data).inverse(c).(Vector))
	case 2:
		// OK
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
			row[x] = m.data[i]
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
	data := make([]Value, 0, len(m.data))
	for _, row := range t {
		data = append(data, row[dim:]...)
	}
	return NewMatrix(m.shape, data)
}
