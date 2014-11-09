// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"bytes"
	"fmt"
	"sort"
)

type Vector []Value

func (v Vector) String() string {
	var b bytes.Buffer
	for i, elem := range v {
		if i > 0 {
			fmt.Fprint(&b, " ")
		}
		fmt.Fprintf(&b, "%s", elem)
	}
	return b.String()
}

func ValueSlice(elem []Value) Vector {
	return Vector(elem)
}

func (v Vector) Eval() Value {
	return v
}

func (v Vector) ToType(which valueType) Value {
	switch which {
	case intType:
		panic("bigint to int")
	case bigIntType:
		panic("vector to big int")
	case bigRatType:
		panic("vector to big rat")
	case vectorType:
		return v
	case matrixType:
		return ValueMatrix([]Value{Int(len(v))}, v)
	}
	panic("Vector.ToType")
}

func (v Vector) Len() int {
	return len(v)
}

func (v Vector) sameLength(x Vector) {
	if v.Len() != x.Len() {
		panic(Errorf("length mismatch: %d %d", v.Len(), x.Len()))
	}
}

// grade returns as a Vector the indexes that sort the vector into increasing order
func (v Vector) grade() Vector {
	x := make([]int, len(v))
	for i := range x {
		x[i] = i
	}
	sort.Sort(&gradeIndex{v: v, x: x})
	origin := conf.Origin()
	result := make([]Value, len(v))
	for i, index := range x {
		n := origin + index
		if n > maxInt { // Unlikely but be careful.
			result[i] = bigInt64(int64(n))
		} else {
			result[i] = Int(n)
		}
	}
	return ValueSlice(result)
}

type gradeIndex struct {
	v Vector
	x []int
}

func (g *gradeIndex) Len() int {
	return len(g.v)
}

func (g *gradeIndex) Less(i, j int) bool {
	return toBool(Binary(g.v[g.x[i]], "<", g.v[g.x[j]]))
}

func (g *gradeIndex) Swap(i, j int) {
	g.x[i], g.x[j] = g.x[j], g.x[i]
}
