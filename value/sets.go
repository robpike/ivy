// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "sort"

// Operations on "sets", which are really just lists that
// can contain duplicates rather than in the mathematical
// definition of sets. APL's like that.

func union(c Context, u, v Value) Value {
	uType := whichType(u)
	vType := whichType(v)
	if uType < vectorType && vType < vectorType {
		// Scalars
		if scalarEqual(c, u, v) {
			return u
		}
		return NewVector([]Value{u, v})
	}
	// Neither can be a matrix.
	if uType == matrixType || vType == matrixType {
		Errorf("binary union not implemented on type matrix")
	}
	// At least one is a Vector.
	switch {
	case vType != vectorType:
		uu := u.(*Vector).Copy().(*Vector)
		for _, x := range uu.All() {
			if scalarEqual(c, x, v) {
				return uu
			}
		}
		return NewVector(append(uu.All(), v))
	case uType != vectorType:
		vv := v.(*Vector)
		elems := []Value{u}
		for _, x := range vv.All() {
			if !scalarEqual(c, u, x) {
				elems = append(elems, x)
			}
		}
		return NewVector(elems)
	default: // Both vectors.
		uu := u.(*Vector).Copy().(*Vector).All()
		vv := v.(*Vector)
		present := membership(c, vv, NewVector(uu))
		for i, x := range vv.All() {
			if present[i] != one {
				uu = append(uu, x)
			}
		}
		return NewVector(uu)
	}
}

func intersect(c Context, u, v Value) Value {
	uType := whichType(u)
	vType := whichType(v)
	if uType < vectorType && vType < vectorType {
		// Scalars
		if scalarEqual(c, u, v) {
			return u
		}
		return empty
	}
	// Neither can be a matrix. Yet. TODO
	if uType == matrixType || vType == matrixType {
		Errorf("binary intersect not implemented on type matrix")
	}
	// At least one is a Vector.
	elems := []Value{}
	switch {
	case vType != vectorType:
		uu := u.(*Vector)
		for _, x := range uu.All() {
			if scalarEqual(c, x, v) {
				elems = append(elems, x)
			}
		}
	case uType != vectorType:
		vv := v.(*Vector)
		for _, x := range vv.All() {
			if scalarEqual(c, u, x) {
				return oneElemVector(u)
			}
		}
		return empty
	default: // Both vectors.
		uu := u.(*Vector)
		present := membership(c, uu, v.(*Vector))
		for i, x := range uu.All() {
			if present[i] == one {
				elems = append(elems, x)
			}
		}
	}
	return NewVector(elems)
}

func without(c Context, u, v Value) Value {
	uType := whichType(u)
	vType := whichType(v)
	if uType < vectorType && vType < vectorType {
		// Scalars
		if scalarEqual(c, u, v) {
			return u
		}
		return empty
	}
	// Neither can be a matrix. Yet. TODO
	if uType == matrixType || vType == matrixType {
		Errorf("binary not not implemented on type matrix")
	}
	// At least one is a Vector.
	elems := []Value{}
	switch {
	case vType != vectorType:
		uu := u.(*Vector)
		for _, x := range uu.All() {
			if !scalarEqual(c, x, v) {
				elems = append(elems, x)
			}
		}
	case uType != vectorType:
		vv := v.(*Vector)
		for _, x := range vv.All() {
			if scalarEqual(c, u, x) {
				return empty
			}
		}
		return oneElemVector(u)
	default: // Both vectors.
		uu := u.(*Vector)
		present := membership(c, uu, v.(*Vector))
		for i, x := range uu.All() {
			if present[i] != one {
				elems = append(elems, x)
			}
		}
	}
	return NewVector(elems)
}

func unique(c Context, v Value) Value {
	vType := whichType(v)
	if vType < vectorType {
		// Scalar
		return v
	}
	if vType == matrixType {
		Errorf("unary unique not implemented on type matrix")
	}
	vv := v.(*Vector)
	if vv.Len() == 0 {
		return vv
	}
	// We could just sort and dedup, but that loses the original
	// order of elements in the vector, which must be preserved.
	type indexedValue struct {
		i int
		v Value
	}
	sorted := make([]indexedValue, vv.Len())
	for i, x := range vv.All() {
		sorted[i] = indexedValue{i, x}
	}
	// Sort based on the values, preserving index information.
	sort.Slice(sorted, func(i, j int) bool {
		c := OrderedCompare(c, sorted[i].v, sorted[j].v)
		if c == 0 {
			// Choose lower type. You need to choose one, so pick lowest.
			return whichType(sorted[i].v) < whichType(sorted[j].v)
		}
		return c < 0
	})
	// Remove duplicates to make a unique list.
	prev := sorted[0]
	uniqued := []indexedValue{prev}
	for _, x := range sorted[1:] {
		if OrderedCompare(c, prev.v, x.v) != 0 {
			uniqued = append(uniqued, x)
			prev = x
		}
	}
	// Restore the original order by sorting on the indexes.
	sort.Slice(uniqued, func(i, j int) bool {
		return uniqued[i].i < uniqued[j].i
	})
	elems := make([]Value, len(uniqued))
	for i, x := range uniqued {
		elems[i] = x.v
	}
	return NewVector(elems)
}

// scalarEqual is faster(ish) comparison to make set ops more efficient.
// The arguments must be scalars.
func scalarEqual(c Context, u, v Value) bool {
	return OrderedCompare(c, u, v) == 0
}

// equal returns 1 or 0 according to whether u and v are equal as a unit
// as opposed to elementwise.
func equal(c Context, u, v Value) Value {
	return toInt(OrderedCompare(c, u, v) == 0)
}

// notEqual is the inverse of equal.
func notEqual(c Context, u, v Value) Value {
	return toInt(OrderedCompare(c, u, v) != 0)
}

// OrderedCompare returns -1, 0, or 1 according to whether u is less than, equal
// to, or greater than v, according to total ordering rules. Total ordering is not
// the usual mathematical definition, as we honor things like 1.0 == 1, comparison
// of int and char is forbidden, and complex numbers do not implement <. Plus other
// than for scalars we don't need to follow any particular rules at all, just what
// works for sorting sets.
//
// Thus we amend the usual orderings:
// - Char is below all other types
// - All other scalars are compared directly, except...
// - ...Complex is above all other scalars, unless on the real line: 1j0 == 1.
// - Vector is above all other types except...
// - ...Matrix, which is above all other types.
//
// When comparing identically-typed values:
//   - Complex is ordered first by real component, then by imaginary.
//   - Vector and Matrix are ordered first by number of elements,
//     then in lexical order of elements.
//
// These are unusual rules, but they are provide a unique ordering of elements
// sufficient for set membership. // Exported for testing, which is done by the
// parent directory to avoid a dependency cycle.
func OrderedCompare(c Context, u, v Value) int {
	uType := whichType(u)
	vType := whichType(v)
	if uType != vType {
		// If either is a Char, that orders below all others.
		if uType == charType {
			return -1
		}
		if vType == charType {
			return 1
		}
		// Need to do it the hard way.
		// First, if one is a Matrix, it orders above all others.
		if uType == matrixType {
			return 1
		}
		if vType == matrixType {
			return -1
		}
		// Next, if one is a Vector, it orders above all others.
		if uType == vectorType {
			return 1
		}
		if vType == vectorType {
			return -1
		}
		// If a complex is on the real line, treat it as a number.
		if uC, ok := u.(Complex); ok && uC.isReal() {
			return OrderedCompare(c, uC.real, v)
		}
		if vC, ok := v.(Complex); ok && vC.isReal() {
			return OrderedCompare(c, u, vC.real)
		}
		// If either is still a Complex, that orders above all others.
		if uType == complexType {
			return 1
		}
		if vType == complexType {
			return -1
		}
		return sgn2(c, u, v)
	}
	switch uType {
	case intType:
		return sgn2Int(int(u.(Int)), int(v.(Int)))
	case charType:
		return sgn2Int(int(u.(Char)), int(v.(Char)))
	case bigIntType:
		return u.(BigInt).Cmp(v.(BigInt).Int)
	case bigRatType:
		return u.(BigRat).Cmp(v.(BigRat).Rat)
	case bigFloatType:
		return u.(BigFloat).Cmp(v.(BigFloat).Float)
	case complexType:
		// We can choose an ordering for Complex, even if math can't.
		// Order by the real part, then the imaginary part.
		uu, vv := u.(Complex), v.(Complex)
		s := OrderedCompare(c, uu.real, vv.real)
		if s != 0 {
			return s
		}
		return OrderedCompare(c, uu.imag, vv.imag)
	case vectorType:
		uu := u.(*Vector)
		vv := v.(*Vector)
		if uu.Len() != vv.Len() {
			return sgn2Int(uu.Len(), vv.Len())
		}
		for i, x := range uu.All() {
			s := OrderedCompare(c, x, vv.At(i))
			if s != 0 {
				return s
			}
		}
		return 0
	case matrixType:
		uu := u.(*Matrix)
		vv := v.(*Matrix)
		if uu.data.Len() != vv.data.Len() {
			return sgn2Int(uu.data.Len(), vv.data.Len())
		}
		for i, x := range uu.data.All() {
			s := OrderedCompare(c, x, vv.data.At(i))
			if s != 0 {
				return s
			}
		}
		return 0
	}
	Errorf("internal error: unknown type %T in OrderedCompare", u)
	return -1

}

// sgn2 returns the signum of a-b.
func sgn2(c Context, a, b Value) int {
	if c.EvalBinary(a, "<", b) == one {
		return -1
	}
	if c.EvalBinary(a, "==", b) == one {
		return 0
	}
	return 1
}

// sgn2Int returns the signum of a-b.
func sgn2Int(a, b int) int {
	if a < b {
		return -1
	}
	if a == b {
		return 0
	}
	return 1
}
