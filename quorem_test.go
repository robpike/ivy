// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Verify that quoRem satisfies the identity
//	quo = x div y  such that
//	rem = x - y*quo  with 0 <= rem < |y|
// See doc for math/big.Int.DivMod.

package main

import (
	"math/big"
	"testing"

	"robpike.io/ivy/exec"
	"robpike.io/ivy/value"
)

type pair struct {
	x, y int
}

var quoRemTests = []pair{
	// We run all the tests with all four signs for 5, 3.
	// The correct results are:
	// 5,3 -> quo 1 rem 2
	// -5,3 -> quo -2 rem 1
	// 5,-3 -> quo -1 rem 2
	// -5,-3 -> quo 2 rem 1
	{5, 3},
	{-5, 3},
	{5, -3},
	{-5, -3},
	// Now check that they work with remainder 0.
	// 5,5 -> quo 1 rem 0
	// -5,5 -> quo -2 rem 0
	// 5,-5 -> quo -1 rem 0
	// -5,-5 -> quo 2 rem 0
	{5, 5},
	{-5, 5},
	{5, -5},
	{-5, -5},
}

func TestQuoRem(t *testing.T) {
	c := exec.NewContext(&testConf)
	for _, test := range quoRemTests {
		verifyQuoRemInt(t, c, test.x, test.y)
		verifyQuoRemBigInt(t, c, test.x, test.y)
		verifyQuoRemBigRat(t, c, test.x, test.y)
		verifyQuoRemBigFloat(t, c, test.x, test.y)
	}
}

func verifyQuoRemInt(t *testing.T, c value.Context, x, y int) {
	t.Helper()
	quoV, remV := value.QuoRem("test", c, value.Int(x), value.Int(y))
	quo := int(quoV.(value.Int))
	rem := int(remV.(value.Int))
	absY := y
	if y < 0 {
		absY = -y
	}
	if rem < 0 || absY <= rem {
		t.Errorf("Int %d QuoRem %d = %d,%d (remainder out of range)", x, y, quo, rem)
	}
	expect := x - y*quo
	if rem != expect {
		t.Errorf("Int %d QuoRem %d = %d,%d yielding %d", x, y, quo, rem, expect)
	}
}

func bigInt(x int64) value.Value {
	return value.BigInt{Int: big.NewInt(x)}
}

func bigRat(x, y int64) value.Value {
	return value.BigRat{Rat: big.NewRat(x, y)}
}

func bigFloat(x float64) value.Value {
	return value.BigFloat{Float: big.NewFloat(x)}
}

func verifyQuoRemBigInt(t *testing.T, c value.Context, X, Y int) {
	t.Helper()
	x, y := int64(X), int64(Y)
	quoV, remV := value.QuoRem("test", c, bigInt(x), bigInt(y))
	// For our tests, we get ints back.
	quo := int64(quoV.(value.Int))
	rem := int64(remV.(value.Int))
	absY := y
	if y < 0 {
		absY = -y
	}
	if rem < 0 || absY <= rem {
		t.Errorf("BigInt %d QuoRem %d = %d,%d (remainder out of range)", x, y, quo, rem)
	}
	expect := x - y*quo
	if rem != expect {
		t.Errorf("BigInt %d QuoRem %d = %d,%d yielding %d", x, y, quo, rem, expect)
	}
}

func verifyQuoRemBigRat(t *testing.T, c value.Context, X, Y int) {
	t.Helper()
	x, y := int64(X), int64(Y)
	quoV, remV := value.QuoRem("test", c, bigRat(x, 1), bigRat(y, 1))
	// For our tests, we get ints back.
	quo := int64(quoV.(value.Int))
	rem := int64(remV.(value.Int))
	absY := y
	if y < 0 {
		absY = -y
	}
	if rem < 0 || absY <= rem {
		t.Errorf("BigRat %d QuoRem %d = %d,%d (remainder out of range)", x, y, quo, rem)
	}
	expect := x - y*quo
	if rem != expect {
		t.Errorf("BigRat %d QuoRem %d = %d,%d yielding %d", x, y, quo, rem, expect)
	}
}

func verifyQuoRemBigFloat(t *testing.T, c value.Context, X, Y int) {
	t.Helper()
	x, y := float64(X), float64(Y)
	quoV, remV := value.QuoRem("test", c, bigFloat(x), bigFloat(y))
	// For our tests, we get ints back.
	quo := float64(quoV.(value.Int))
	rem := float64(remV.(value.Int))
	absY := y
	if y < 0 {
		absY = -y
	}
	if rem < 0 || absY <= rem {
		t.Errorf("BigFloat %g QuoRem %g = %g,%g (remainder out of range)", x, y, quo, rem)
	}
	expect := x - y*quo
	if rem != expect {
		t.Errorf("BigFloat %g QuoRem %g = %g,%g yielding %g", x, y, quo, rem, expect)
	}
}
