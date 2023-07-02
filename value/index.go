// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "sync"

// An indexState holds the state needed to locate
// the values denoted by an index expression left[index],
// which is evaluated to lhs[indexes].
type indexState struct {
	lhs   Value
	slice []Value // underlying data slice for lhs
	shape []int   // underlying shape for lhs

	indexes []Vector // Vectors of all Int, all in range for shape

	outShape []int // output shape (nil is scalar)
	outSize  int   // output size (# scalars)
}

// init initializes ix to describe top, which is left[index].
// Left and index will be evaluated (right to left),
// while top is only for its ProgString method.
func (ix *indexState) init(context Context, top, left Expr, index []Expr) {
	// Evaluate indexes, make sure all are Vector of Int.
	// Compute shape of result as we go.
	// Scalar indexes drop a dimension,
	// while vector and matrix indexes replace the dimension with their shape.
	ix.indexes = make([]Vector, len(index))
	ix.outShape = nil          // common case - scalar indexes covering entire rank → scalar result
	var outShapeToUpdate []int // indexes of outShape entries that need updating after lhs eval.
	for i := len(index) - 1; i >= 0; i-- {
		if index[i] == nil {
			// Make this iota(dimension), to be filled in after evaluating lhs.
			ix.indexes[i] = nil
			outShapeToUpdate = append(outShapeToUpdate, len(ix.outShape))
			ix.outShape = append(ix.outShape, 0) // Fixed below, after we have evaluated left.
			continue
		}
		x := index[i].Eval(context).Inner()
		switch x := x.(type) {
		default:
			Errorf("invalid index %s (%s) in %s", index[i].ProgString(), whichType(x), top.ProgString())
		case Int:
			ix.indexes[i] = Vector{x}
		case Vector:
			ix.indexes[i] = x
			ix.outShape = append(ix.outShape, len(x))
		case *Matrix:
			ix.indexes[i] = x.Data()
			// Append shape in reverse, because ix.shape will be reversed below.
			shape := x.Shape()
			for j := len(shape) - 1; j >= 0; j-- {
				ix.outShape = append(ix.outShape, shape[j])
			}
		}
		for _, v := range ix.indexes[i] {
			if _, ok := v.(Int); !ok {
				Errorf("invalid index %v (%s) in %s in %s", v, whichType(v), index[i].ProgString(), top.ProgString())
			}
		}
	}

	// Walked indexes right-to-left, so reverse shape.
	for i, j := 0, len(ix.outShape)-1; i < j; i, j = i+1, j-1 {
		ix.outShape[i], ix.outShape[j] = ix.outShape[j], ix.outShape[i]
	}
	// The offsets stored in outShapeToUpdate must also be flipped.
	for i, o := range outShapeToUpdate {
		outShapeToUpdate[i] = len(ix.outShape) - o - 1
	}

	// Can now safely evaluate left side
	// (must wait until indexes have been evaluated, R-to-L).
	ix.lhs = left.Eval(context)
	switch lhs := ix.lhs.(type) {
	default:
		Errorf("cannot index %s (%v)", left.ProgString(), whichType(lhs))
	case *Matrix:
		ix.slice = lhs.Data()
		ix.shape = lhs.Shape()
	case Vector:
		ix.slice = lhs
		ix.shape = []int{len(lhs)}
	}

	// Finish the result shape.
	origin := Int(context.Config().Origin())
	if len(ix.indexes) > len(ix.shape) {
		Errorf("too many dimensions in %s indexing shape %v", top.ProgString(), NewIntVector(ix.shape))
	}
	// Replace nil index entries, created above, with iota(dimension).
	j := 0
	for i, v := range ix.indexes {
		if v == nil {
			x := constIota(int(origin), ix.shape[i])
			ix.indexes[i] = NewVector(x)
			ix.outShape[outShapeToUpdate[j]] = len(x)
			j++
		}
	}
	ix.outShape = append(ix.outShape, ix.shape[len(index):]...)
	ix.outSize = size(ix.outShape)

	// Check indexes are all valid.
	for i, v := range ix.indexes {
		for j := range v {
			vj := v[j].(Int)
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
				Errorf("index %s out of range for shape %v", s, NewIntVector(ix.shape))
			}
		}
	}
}

// Index returns left[index].
// Left and index will be evaluated (right to left),
// while top is only for its ProgString method.
func Index(context Context, top, left Expr, index []Expr) Value {
	var ix indexState
	ix.init(context, top, left, index)
	origin := Int(context.Config().Origin())

	if len(ix.outShape) == 0 {
		// Trivial scalar case.
		offset := 0
		for j := 0; j < len(ix.indexes); j++ {
			if j > 0 {
				offset *= ix.shape[j]
			}
			offset += int(ix.indexes[j][0].(Int) - origin)
		}
		return ix.slice[offset]
	}

	data := make(Vector, ix.outSize)
	copySize := int(size(ix.shape[len(ix.indexes):]))
	n := len(data) / copySize
	coord := make([]int, len(ix.indexes))
	for i := 0; i < n; i++ {
		// Copy data for indexes[coord].
		offset := 0
		for j := 0; j < len(ix.indexes); j++ {
			if j > 0 {
				offset *= ix.shape[j]
			}
			offset += int(ix.indexes[j][coord[j]].(Int) - origin)
		}
		copy(data[i*copySize:(i+1)*copySize], ix.slice[offset*copySize:(offset+1)*copySize])

		// Increment coord.
		for j := len(coord) - 1; j >= 0; j-- {
			if coord[j]++; coord[j] < len(ix.indexes[j]) {
				break
			}
			coord[j] = 0
		}
	}

	if len(ix.outShape) == 0 {
		return data[0]
	}
	if len(ix.outShape) == 1 {
		return data
	}
	return NewMatrix(ix.outShape, data)
}

// IndexAssign handles general assignment to indexed expressions on the LHS.
// Left and index will be evaluated (right to left),
// while top is only for its ProgString method.
// The caller must check that left is a variable expression,
// so that the assignment is not being written into a temporary.
func IndexAssign(context Context, top, left Expr, index []Expr, right Expr, rhs Value) {
	var ix indexState
	ix.init(context, top, left, index)

	// Unless assigning to a single cell, RHS must be scalar or
	// have same shape as indexed expression.
	var rscalar Value
	var rslice []Value
	if len(ix.outShape) == 0 {
		rscalar = rhs
	} else {
		switch rhs := rhs.(type) {
		default:
			rscalar = rhs
		case Vector:
			if len(ix.outShape) != 1 || ix.outShape[0] != len(rhs) {
				Errorf("shape mismatch %v != %v in assignment %v = %v",
					NewIntVector(ix.outShape), NewIntVector([]int{len(rhs)}),
					top.ProgString(), right.ProgString())
			}
			rslice = rhs
		case *Matrix:
			if !sameShape(ix.outShape, rhs.Shape()) {
				Errorf("shape mismatch %v != %v in assignment %v = %v",
					NewIntVector(ix.outShape), NewIntVector(rhs.Shape()),
					top.ProgString(), right.ProgString())
			}
			rslice = rhs.Data()
			if rhs == ix.lhs {
				// Assigning entire rhs to some permutation of lhs.
				// Make copy of slice to avoid problems with overwriting
				// values we need to read later. Uncommon.
				rslice = make([]Value, len(rslice))
				copy(rslice, rhs.Data())
			}
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
			offset += int(ix.indexes[j][0].(Int) - origin)
		}
		ix.slice[offset] = rscalar.Copy()
		return
	}

	copySize := int(size(ix.shape[len(ix.indexes):]))
	n := ix.outSize / copySize
	pfor(true, copySize, n, func(lo, hi int) {
		// Compute starting coordinate index.
		coord := make([]int, len(ix.indexes))
		i := lo
		for j := len(coord) - 1; j >= 0; j-- {
			if n := len(ix.indexes[j]); n > 0 {
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
				offset += int(ix.indexes[j][coord[j]].(Int) - origin)
			}
			dst := ix.slice[offset*copySize : (offset+1)*copySize]
			if rscalar != nil {
				for j := range dst {
					dst[j] = rscalar.Copy()
				}
			} else {
				for j := range dst {
					dst[j] = rslice[j+i*copySize].Copy()
				}
			}

			// Increment coord.
			for j := len(coord) - 1; j >= 0; j-- {
				if coord[j]++; coord[j] < len(ix.indexes[j]) {
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
