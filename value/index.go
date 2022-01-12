// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

// An indexState holds the state needed to locate
// the values denoted by an index expression left[index],
// which is evaluated to lhs[indexes].
type indexState struct {
	lhs   Value
	slice []Value // underlying data slice for lhs
	shape []int   // underlying shape for lhs

	indexes []Vector // Vectors of all Int, all in range for shape

	xshape []int // output shape (nil is scalar)
	xsize  int   // output size (# scalars)
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
	ix.xshape = nil // common case - scalar indexes covering entire rank → scalar result
	for i := len(index) - 1; i >= 0; i-- {
		x := index[i].Eval(context).Inner()
		switch x := x.(type) {
		default:
			Errorf("invalid index %s (%s) in %s", index[i].ProgString(), whichType(x), top.ProgString())
		case Int:
			ix.indexes[i] = Vector{x}
		case Vector:
			ix.indexes[i] = x
			ix.xshape = append(ix.xshape, len(x))
		case *Matrix:
			ix.indexes[i] = x.Data()
			ix.xshape = append(ix.xshape, x.Shape()...)
		}
		for _, v := range ix.indexes[i] {
			if _, ok := v.(Int); !ok {
				Errorf("invalid index %v (%s) in %s in %s", v, whichType(v), index[i].ProgString(), top.ProgString())
			}
		}
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
	if len(ix.indexes) > len(ix.shape) {
		Errorf("too many dimensions in %s indexing shape %v", top.ProgString(), NewIntVector(ix.shape))
	}
	ix.xshape = append(ix.xshape, ix.shape[len(index):]...)
	ix.xsize = size(ix.xshape)

	// Check indexes are all valid.
	origin := Int(context.Config().Origin())
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

	if len(ix.xshape) == 0 {
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

	data := make(Vector, ix.xsize)
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

	if len(ix.xshape) == 0 {
		return data[0]
	}
	if len(ix.xshape) == 1 {
		return data
	}
	return NewMatrix(ix.xshape, data)
}

// IndexAssign handles general assignment to indexed expressions on the LHS.
// Left and index will be evaluated (right to left),
// while top is only for its ProgString method.
// The caller must check that left is a variable expression,
// so that the assignment is not being written into a temporary.
func IndexAssign(context Context, top, left Expr, index []Expr, right Expr, rhs Value) {
	var ix indexState
	ix.init(context, top, left, index)

	// RHS must be scalar or have same shape as indexed expression.
	var rscalar Value
	var rslice []Value
	switch rhs := rhs.(type) {
	default:
		rscalar = rhs
	case *Matrix:
		if !sameShape(ix.xshape, rhs.Shape()) {
			Errorf("shape mismatch %v != %v in assignment %v = %v",
				NewIntVector(ix.xshape), NewIntVector(rhs.Shape()),
				top.ProgString(), right.ProgString())
		}
		rslice = rhs.Data()
		if rhs == ix.lhs {
			// Assigning entire rhs to some permutation of lhs.
			// Make copy of values to avoid problems with overwriting
			// values we need to read later. Uncommon.
			rslice = make([]Value, len(rslice))
			copy(rslice, rhs.Data())
		}
	case Vector:
		if len(ix.xshape) != 1 || ix.xshape[0] != len(rhs) {
			Errorf("shape mismatch %v != %v in assignment %v = %v",
				NewIntVector(ix.xshape), NewIntVector([]int{len(rhs)}),
				top.ProgString(), right.ProgString())
		}
		rslice = rhs
	}

	origin := Int(context.Config().Origin())
	if len(ix.xshape) == 0 {
		// Trivial scalar case.
		offset := 0
		for j := 0; j < len(ix.indexes); j++ {
			if j > 0 {
				offset *= ix.shape[j]
			}
			offset += int(ix.indexes[j][0].(Int) - origin)
		}
		ix.slice[offset] = rscalar
	}

	copySize := int(size(ix.shape[len(ix.indexes):]))
	n := ix.xsize / copySize
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
				for i := range dst {
					dst[i] = rscalar
				}
			} else {
				copy(dst, rslice[i*copySize:(i+1)*copySize])
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
