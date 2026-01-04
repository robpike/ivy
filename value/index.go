// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"slices"
	"sync"
)

// An indexState holds the state needed to locate
// the values denoted by an index expression left[index],
// which is evaluated to lhs[indexes].
type indexState struct {
	lhs    Value
	vector *Vector
	edit   *vectorEditor
	shape  []int // underlying shape for lhs

	indexes     []*Vector // Vectors of all Int, all in range for shape...
	indexVector *Vector   // ... or vector of index vectors
	indexDim    int       // number of dimensions indexed

	outShape []int // output shape (nil is scalar)
	outSize  int   // output size (# scalars)
}

// init initializes ix to describe top, which is left[index].
// Left and index will be evaluated (right to left),
// while top is only for its ProgString method.
// If lvarx is not nil, then it is the variable expression
// corresponding to left, to be used for assignments.
func (ix *indexState) init(context Context, top, left Expr, lvarx *VarExpr, index []Expr) {
	// Evaluate indexes, make sure all are Vector of Int.
	// Compute shape of result as we go.
	// Scalar indexes drop a dimension,
	// while vector and matrix indexes replace the dimension with their shape.
	ix.indexes = make([]*Vector, len(index))
	ix.outShape = nil          // common case - scalar indexes covering entire rank → scalar result
	var outShapeToUpdate []int // indexes of outShape entries that need updating after lhs eval.
	missing := make([]bool, len(index))
	for i := len(index) - 1; i >= 0; i-- {
		if index[i] == nil {
			// Make this iota(dimension), to be filled in after evaluating lhs.
			missing[i] = true
			outShapeToUpdate = append(outShapeToUpdate, len(ix.outShape))
			ix.outShape = append(ix.outShape, 0) // Fixed below, after we have evaluated left.
			continue
		}
		x := index[i].Eval(context).Inner()
		switch x := x.(type) {
		default:
			context.Errorf("invalid index %s (type %s) in %s", index[i].ProgString(), whichType(x), top.ProgString())
		case Int:
			ix.indexes[i] = NewVector(x)
		case *Vector:
			ix.indexes[i] = x
			ix.outShape = append(ix.outShape, x.Len())
		case *Matrix:
			ix.indexes[i] = x.Data()
			// Append shape in reverse, because ix.shape will be reversed below.
			shape := x.Shape()
			for j := len(shape) - 1; j >= 0; j-- {
				ix.outShape = append(ix.outShape, shape[j])
			}
		}
	}

	// Walked indexes right-to-left, so reverse shape.
	slices.Reverse(ix.outShape)

	// The offsets stored in outShapeToUpdate must also be flipped.
	for i, o := range outShapeToUpdate {
		outShapeToUpdate[i] = len(ix.outShape) - o - 1
	}

	// Can now safely evaluate left side
	// (must wait until indexes have been evaluated, R-to-L).
	var lvar *Var
	if lvarx != nil {
		if lvarx.Local >= 1 {
			lvar = context.Local(lvarx.Local)
		} else {
			lvar = context.Global(lvarx.Name)
			if lvar == nil {
				context.Errorf("undefined global variable %q", lvarx.Name)
			}
		}
		ix.lhs = lvar.value
	} else {
		ix.lhs = left.Eval(context)
	}
	switch lhs := ix.lhs.(type) {
	default:
		context.Errorf("cannot index %s (%v)", left.ProgString(), whichType(lhs))
	case *Matrix:
		ix.vector = lhs.data
		if lvar != nil {
			ix.edit = lvar.editor()
		}
		ix.shape = lhs.Shape()
	case *Vector:
		ix.vector = lhs
		if lvar != nil {
			ix.edit = lvar.editor()
		}
		ix.shape = []int{lhs.Len()}
	}

	origin := Int(context.Config().Origin())
	if ix.initVectorIndex(context, origin) {
		return
	}

	// Finish the result shape.
	if len(ix.indexes) > len(ix.shape) {
		context.Errorf("too many dimensions in %s indexing shape %v", top.ProgString(), NewIntVector(ix.shape...))
	}
	// Replace nil index entries, created above, with iota(dimension).
	j := 0
	for i := range ix.indexes {
		if missing[i] {
			x := newIota(context, int(origin), ix.shape[i])
			ix.indexes[i] = x
			ix.outShape[outShapeToUpdate[j]] = x.Len()
			j++
		}
	}
	ix.outShape = append(ix.outShape, ix.shape[len(index):]...)
	ix.outSize = size(context, ix.outShape)
	ix.indexDim = len(index)

	// Check indexes are all valid.
	for i, v := range ix.indexes {
		for _, vj := range v.All() {
			if _, ok := vj.(Int); !ok {
				context.Errorf("invalid index %s (type %s) in %s", vj, whichType(vj), top.ProgString())
			}
		}
		for j := range v.All() {
			vj := v.At(j).(Int)
			if vj < origin || vj-origin >= Int(ix.shape[i]) {
				s := left.ProgString() + "["
				for k := range ix.indexes {
					if k > 0 {
						s += "; "
					}
					if k == i {
						s += vj.String()
					} else {
						s += "_"
					}
				}
				s += "]"
				context.Errorf("index %s out of range for shape %v", s, NewIntVector(ix.shape...))
			}
		}
	}
}

func (ix *indexState) initVectorIndex(c Context, origin Int) bool {
	// Check for single index, vector of vectors,
	// all of which have the same length <= rank of lhs.
	if len(ix.indexes) != 1 || ix.indexes[0] == nil || ix.indexes[0].Len() == 0 {
		return false
	}
	data := ix.indexes[0]
	v, ok := data.At(0).(*Vector)
	if !ok {
		return false
	}
	if v.Len() > len(ix.shape) {
		c.Errorf("index vector (%v) too long for shape %v", v, NewIntVector(ix.shape...))
	}
	n := v.Len()
	for _, x := range data.All() {
		v, ok := x.(*Vector)
		if !ok {
			c.Errorf("mixed vector and non-vector indices %v and %v", data.At(0), x)
		}
		if v.Len() != n {
			c.Errorf("index vectors of mixed lengths %v and %v", data.At(0), v)
		}
		// Check vector content against shape.
		for j, vj := range v.All() {
			k, ok := vj.(Int)
			if !ok {
				c.Errorf("index vector %v contains invalid index %v (type %s)", v, vj, whichType(vj))
			}
			if k < origin || k-origin >= Int(ix.shape[j]) {
				c.Errorf("index vector %v out of range for shape %v", v, NewIntVector(ix.shape...))
			}
		}
	}
	ix.indexes = nil
	ix.indexVector = data
	ix.indexDim = n
	ix.outShape = append(ix.outShape, ix.shape[n:]...)
	ix.outSize = size(c, ix.outShape)
	return true
}

// Index returns left[index].
// Left and index will be evaluated (right to left),
// while top is only for its ProgString method.
func Index(context Context, top, left Expr, index []Expr) Value {
	var ix indexState
	ix.init(context, top, left, nil, index)
	origin := Int(context.Config().Origin())

	if len(ix.outShape) == 0 {
		// Trivial scalar case.
		offset := 0
		for j := 0; j < len(ix.indexes); j++ {
			if j > 0 {
				offset *= ix.shape[j]
			}
			offset += int(ix.indexes[j].At(0).(Int) - origin)
		}
		return ix.vector.At(offset)
	}

	data := newVectorEditor(ix.outSize, nil)
	copySize := int(size(context, ix.shape[ix.indexDim:]))
	n := data.Len() / copySize

	if ix.indexVector != nil {
		for i, v := range ix.indexVector.All() {
			offset := 0
			for j, vj := range v.(*Vector).All() {
				if j > 0 {
					offset *= ix.shape[j]
				}
				offset += int(vj.(Int) - origin)
			}
			for k := range copySize {
				data.Set(i*copySize+k, ix.vector.At(offset*copySize+k))
			}
		}
	} else {
		coord := make([]int, ix.indexDim)
		for i := 0; i < n; i++ {
			// Copy data for indexes[coord].
			offset := 0
			for j := 0; j < len(ix.indexes); j++ {
				if j > 0 {
					offset *= ix.shape[j]
				}
				offset += int(ix.indexes[j].At(coord[j]).(Int) - origin)
			}
			for k := range copySize {
				data.Set(i*copySize+k, ix.vector.At(offset*copySize+k))
			}

			// Increment coord.
			for j := len(coord) - 1; j >= 0; j-- {
				if coord[j]++; coord[j] < ix.indexes[j].Len() {
					break
				}
				coord[j] = 0
			}
		}
	}

	if len(ix.outShape) == 0 {
		return data.At(0)
	}
	if len(ix.outShape) == 1 {
		return data.Publish()
	}
	return NewMatrix(context, ix.outShape, data.Publish())
}

// IndexAssign handles general assignment to indexed expressions on the LHS.
// Left and index will be evaluated (right to left),
// while top is only for its ProgString method.
// The caller must check that left is a variable expression
// and pass lvar, the variable corresponding to left.
func IndexAssign(context Context, top, left Expr, lvarx *VarExpr, index []Expr, right Expr, rhs Value) {
	var ix indexState
	ix.init(context, top, left, lvarx, index)

	// Unless assigning to a single cell, RHS must be scalar or
	// have same shape as indexed expression.
	var rscalar Value
	var rvector *Vector
	if len(ix.outShape) == 0 {
		rscalar = rhs
	} else {
		badShape := func(rshape ...int) {
			var where string
			if right == nil {
				where = "to " + top.ProgString()
			} else {
				where = top.ProgString() + " = " + right.ProgString()
			}
			context.Errorf("shape mismatch %v != %v in assignment %v",
				NewIntVector(ix.outShape...), NewIntVector(rshape...),
				where)
		}

		switch rhs := rhs.(type) {
		default:
			rscalar = rhs
		case *Vector:
			if len(ix.outShape) != 1 || ix.outShape[0] != rhs.Len() {
				badShape(rhs.Len())
			}
			rvector = rhs
		case *Matrix:
			if !sameShape(ix.outShape, rhs.Shape()) {
				badShape(rhs.Shape()...)
			}
			rvector = rhs.data
		}
	}

	origin := Int(context.Config().Origin())
	if len(ix.outShape) == 0 {
		// Trivial scalar case.
		offset := 0
		for j := 0; j < len(ix.indexes); j++ {
			if j > 0 {
				offset *= ix.shape[j]
			}
			offset += int(ix.indexes[j].At(0).(Int) - origin)
		}
		ix.edit.Set(offset, rscalar)
		return
	}

	copySize := int(size(context, ix.shape[ix.indexDim:]))
	n := ix.outSize / copySize
	pfor(true, copySize, n, func(lo, hi int) {
		if ix.indexVector != nil {
			for i := lo; i < hi; i++ {
				v := ix.indexVector.At(i).(*Vector)
				offset := 0
				for j, vj := range v.All() {
					if j > 0 {
						offset *= ix.shape[j]
					}
					offset += int(vj.(Int) - origin)
				}
				dstOff := offset * copySize
				if rscalar != nil {
					for j := range copySize {
						ix.edit.Set(dstOff+j, rscalar)
					}
				} else {
					for j := range copySize {
						ix.edit.Set(dstOff+j, rvector.At(j+i*copySize))
					}
				}
			}
			return
		}

		// Compute starting coordinate index.
		coord := make([]int, len(ix.indexes))
		i := lo
		for j := len(coord) - 1; j >= 0; j-- {
			if n := ix.indexes[j].Len(); n > 0 {
				coord[j] = i % n
				i /= n
			}
		}

		for i := lo; i < hi; i++ {
			// Copy data for indexes[coord].
			offset := 0
			for j := 0; j < len(ix.indexes); j++ {
				if j > 0 {
					offset *= ix.shape[j]
				}
				offset += int(ix.indexes[j].At(coord[j]).(Int) - origin)
			}
			dstOff := offset * copySize
			if rscalar != nil {
				for j := range copySize {
					ix.edit.Set(dstOff+j, rscalar)
				}
			} else {
				for j := range copySize {
					ix.edit.Set(dstOff+j, rvector.At(j+i*copySize))
				}
			}

			// Increment coord.
			for j := len(coord) - 1; j >= 0; j-- {
				if coord[j]++; coord[j] < ix.indexes[j].Len() {
					break
				}
				coord[j] = 0
			}
		}
	})
}

var (
	iotaLock   sync.RWMutex
	staticIota []Value
)

// constIota generates a slice equivalent to the result of "iota n".
// The returned value's elements are shared and must not be overwritten.
func constIota(origin, n int) []Value {
	for {
		iotaLock.RLock()
		if len(staticIota) >= origin+n {
			result := staticIota[origin : origin+n]
			iotaLock.RUnlock()
			return result
		}
		iotaLock.RUnlock()
		growIota(origin + n)
	}
}

// newIota returns the result of 'iota n' as a new Vector.
func newIota(c Context, origin, n int) *Vector {
	if n < 0 || maxInt < n {
		c.Errorf("bad iota %d", n)
	}
	return NewVector(constIota(origin, int(n))...)
}

func growIota(n int) {
	iotaLock.Lock()
	if len(staticIota) < n {
		m := make([]Value, n+32)
		for i := range m {
			m[i] = Int(i)
		}
		staticIota = m
	}
	iotaLock.Unlock()
}
