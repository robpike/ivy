// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"errors"
	"math/big"
)

type BigRat struct {
	x big.Rat
}

func SetBigRatString(s string) (BigRat, error) {
	var r BigRat
	_, ok := r.x.SetString(s)
	if !ok {
		return BigRat{}, errors.New("rational number syntax")
	}
	return r, nil
}

func (r BigRat) String() string {
	return r.x.String()
}

func (r BigRat) Eval() Value {
	return r
}

func (r BigRat) ToType(which valueType) Value {
	switch which {
	case intType:
		panic("big rat to int")
	case bigIntType:
		panic("big rat to big int")
	case bigRatType:
		return r
	case vectorType:
		return ValueSlice([]Value{r})
	}
	panic("BigRat.ToType")
}

// shrink pulls, if possible, a BigRat down to a BigInt or Int.
func (r BigRat) shrink() Value {
	if !r.x.IsInt() {
		return r
	}
	var b BigInt
	b.x = *r.x.Num()
	return b.shrink()
}
