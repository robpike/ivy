// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"errors"
	"fmt"
	"math/big"
)

type BigRat struct {
	*big.Rat
}

func SetBigRatString(s string) (BigRat, error) {
	r, ok := big.NewRat(0, 1).SetString(s)
	if !ok {
		return BigRat{}, errors.New("rational number syntax")
	}
	return BigRat{r}, nil
}

func (r BigRat) String() string {
	return fmt.Sprintf(conf.RatFormat(), r.Num(), r.Denom())
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
	if !r.IsInt() {
		return r
	}
	return BigInt{r.Num()}.shrink()
}
