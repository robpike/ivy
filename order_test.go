// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"math/big"
	"testing"

	"robpike.io/ivy/config"
	"robpike.io/ivy/exec"
	"robpike.io/ivy/value"
)

type orderTest struct {
	u, v value.Value
	sgn  int
}

var (
	int0 = value.Int(0)
	int1 = value.Int(1)
	int2 = value.Int(2)
	int3 = value.Int(3)

	char1 = value.Char(1)
	char2 = value.Char(2)
	char3 = value.Char(3)

	bigInt0 = value.BigInt{Int: big.NewInt(0)}
	bigInt1 = value.BigInt{Int: big.NewInt(1)}
	bigInt2 = value.BigInt{Int: big.NewInt(2)}
	bigInt3 = value.BigInt{Int: big.NewInt(3)}

	bigRat0o1 = value.BigRat{Rat: big.NewRat(0, 1)}
	bigRat1o1 = value.BigRat{Rat: big.NewRat(1, 1)}
	bigRat2o1 = value.BigRat{Rat: big.NewRat(2, 1)}
	bigRat1o7 = value.BigRat{Rat: big.NewRat(1, 7)}
	bigRat2o7 = value.BigRat{Rat: big.NewRat(2, 7)}
	bigRat3o7 = value.BigRat{Rat: big.NewRat(3, 7)}

	bigFloat0p0 = value.BigFloat{Float: big.NewFloat(0.0)}
	bigFloat1p0 = value.BigFloat{Float: big.NewFloat(1.0)}
	bigFloat2p0 = value.BigFloat{Float: big.NewFloat(2.0)}
	bigFloat1p5 = value.BigFloat{Float: big.NewFloat(1.5)}
	bigFloat2p5 = value.BigFloat{Float: big.NewFloat(2.5)}
	bigFloat3p5 = value.BigFloat{Float: big.NewFloat(3.5)}

	complex1j0 = value.NewComplex(int1, int0)
	complex1j1 = value.NewComplex(int1, int1)
	complex1j2 = value.NewComplex(int1, int2) // Same real, bigger imaginary.
	complex2j1 = value.NewComplex(int2, int1) // Bigger real, lesser imaginary
	complex2j2 = value.NewComplex(int2, int2) // Same real, bigger imaginary

	vector0000 = value.NewIntVector([]int{0, 0, 0, 0})
	vector012  = value.NewIntVector([]int{0, 1, 2})
	vector022  = value.NewIntVector([]int{0, 2, 2})

	matrix000_000 = value.NewMatrix([]int{2, 3}, newMatrixData(0, 0, 0, 0, 0, 0))
	matrix12_34   = value.NewMatrix([]int{2, 2}, newMatrixData(1, 2, 3, 4))
	matrix12_44   = value.NewMatrix([]int{2, 2}, newMatrixData(1, 2, 4, 4))
)

func newMatrixData(data ...int) []value.Value {
	v := make([]value.Value, len(data))
	for i := range data {
		v[i] = value.Int(data[i])
	}
	return v
}

func TestOrderedCompare(t *testing.T) {
	var tests = []orderTest{
		// Same types.
		// Int
		{int1, int1, 0},
		{int1, int2, -1},
		{int1, int3, -1},
		{int2, int1, 1},
		{int2, int2, 0},
		{int2, int3, -1},
		{int3, int1, 1},
		{int3, int2, 1},
		{int3, int3, 0},

		// Char
		{char1, char1, 0},
		{char1, char2, -1},
		{char1, char3, -1},
		{char2, char1, 1},
		{char2, char2, 0},
		{char2, char3, -1},
		{char3, char1, 1},
		{char3, char2, 1},
		{char3, char3, 0},

		// BigInt
		{bigInt1, bigInt1, 0},
		{bigInt1, bigInt2, -1},
		{bigInt1, bigInt3, -1},
		{bigInt2, bigInt1, 1},
		{bigInt2, bigInt2, 0},
		{bigInt2, bigInt3, -1},
		{bigInt3, bigInt1, 1},
		{bigInt3, bigInt2, 1},
		{bigInt3, bigInt3, 0},

		// BigRat
		{bigRat1o7, bigRat1o7, 0},
		{bigRat1o7, bigRat2o7, -1},
		{bigRat1o7, bigRat3o7, -1},
		{bigRat2o7, bigRat1o7, 1},
		{bigRat2o7, bigRat2o7, 0},
		{bigRat2o7, bigRat3o7, -1},
		{bigRat3o7, bigRat1o7, 1},
		{bigRat3o7, bigRat2o7, 1},
		{bigRat3o7, bigRat3o7, 0},

		// BigFloat
		{bigFloat1p5, bigFloat1p5, 0},
		{bigFloat1p5, bigFloat2p5, -1},
		{bigFloat1p5, bigFloat3p5, -1},
		{bigFloat2p5, bigFloat1p5, 1},
		{bigFloat2p5, bigFloat2p5, 0},
		{bigFloat2p5, bigFloat3p5, -1},
		{bigFloat3p5, bigFloat1p5, 1},
		{bigFloat3p5, bigFloat2p5, 1},
		{bigFloat3p5, bigFloat3p5, 0},

		// Complex
		{complex1j1, complex1j1, 0},
		{complex1j1, complex1j2, -1},
		{complex1j1, complex2j1, -1},
		{complex1j1, complex2j2, -1},
		{complex1j2, complex1j1, 1},
		{complex1j2, complex1j2, 0},
		{complex1j2, complex2j1, -1},
		{complex1j2, complex2j2, -1},
		{complex2j1, complex1j1, 1},
		{complex2j1, complex1j2, 1},
		{complex2j1, complex2j1, 0},
		{complex2j1, complex2j2, -1},
		{complex2j2, complex1j1, 1},
		{complex2j2, complex1j2, 1},
		{complex2j2, complex2j1, 1},
		{complex2j2, complex2j2, 0},

		// Int less than every possible scalar type.
		{int0, bigInt1, -1},
		{int0, bigRat1o1, -1},
		{int0, bigFloat1p0, -1},
		{int0, complex1j0, -1},

		// Int equal to every possible scalar type.
		{int1, bigInt1, 0},
		{int1, bigRat1o1, 0},
		{int1, bigFloat1p0, 0},
		{int1, complex1j0, 0},

		// Int greater than every possible scalar type.
		{int2, bigInt1, 1},
		{int2, bigRat1o1, 1},
		{int2, bigFloat1p0, 1},
		{int2, complex1j0, 1},

		// BigInt less than every possible scalar type.
		{bigInt0, int1, -1},
		{bigInt0, bigRat1o1, -1},
		{bigInt0, bigFloat1p0, -1},
		{bigInt0, complex1j0, -1},

		// BigInt equal to every possible scalar type.
		{bigInt1, int1, 0},
		{bigInt1, bigRat1o1, 0},
		{bigInt1, bigFloat1p0, 0},
		{bigInt1, complex1j0, 0},

		// BigInt greater than every possible scalar type.
		{bigInt2, int1, 1},
		{bigInt2, bigRat1o1, 1},
		{bigInt2, bigFloat1p0, 1},
		{bigInt2, complex1j0, 1},

		// BigRat less than every possible scalar type.
		{bigRat0o1, int1, -1},
		{bigRat0o1, bigInt1, -1},
		{bigRat0o1, bigFloat1p0, -1},
		{bigRat0o1, complex1j0, -1},

		// BigRat equal to every possible scalar type.
		{bigRat1o1, int1, 0},
		{bigRat1o1, bigInt1, 0},
		{bigRat1o1, bigFloat1p0, 0},
		{bigRat1o1, complex1j0, 0},

		// BigRat greater than every possible scalar type.
		{bigRat2o1, int1, 1},
		{bigRat2o1, bigInt1, 1},
		{bigRat2o1, bigFloat1p0, 1},
		{bigRat2o1, complex1j0, 1},

		// BigFloat less than every possible scalar type.
		{bigFloat0p0, int1, -1},
		{bigFloat0p0, bigInt1, -1},
		{bigFloat0p0, bigFloat1p0, -1},
		{bigFloat0p0, complex1j0, -1},

		// BigFloat equal to every possible scalar type.
		{bigFloat1p0, int1, 0},
		{bigFloat1p0, bigInt1, 0},
		{bigFloat1p0, bigFloat1p0, 0},
		{bigFloat1p0, complex1j0, 0},

		// BigFloat greater than every possible scalar type.
		{bigFloat2p0, int1, 1},
		{bigFloat2p0, bigInt1, 1},
		{bigFloat2p0, bigFloat1p0, 1},
		{bigFloat2p0, complex1j0, 1},

		// Special cases involving char and complex.

		// Char is always less than every other type.
		{char1, int1, -1},
		{char1, bigInt1, -1},
		{char1, bigRat1o1, -1},
		{char1, bigFloat1p0, -1},
		{char1, complex1j0, -1},

		// Complex that is actually real is like a float.
		{complex1j0, int1, 0},
		{complex1j0, char1, 1}, // Note: can't compare with char. See next block of tests.
		{complex1j0, bigInt1, 0},
		{complex1j0, bigRat1o1, 0},
		{complex1j0, bigFloat1p0, 0},

		// Complex with imaginary part is always greater than every other scalar type.
		{complex1j1, int1, 1},
		{complex1j1, char1, 1},
		{complex1j1, bigInt1, 1},
		{complex1j1, bigRat1o1, 1},
		{complex1j1, bigFloat1p0, 1},

		// Vector bigger than every type.
		{vector012, int3, 1},
		{vector012, char3, 1},
		{vector012, char3, 1},
		{vector012, bigInt3, 1},
		{vector012, bigRat3o7, 1},
		{vector012, bigFloat3p5, 1},
		{vector012, complex2j2, 1},

		// Vector comparisons.
		{vector0000, vector012, 1}, // Length dominates.
		{vector012, vector022, -1},
		{vector012, vector012, 0},
		{vector022, vector012, 1},

		// Matrix bigger than every type.
		{matrix12_34, int3, 1},
		{matrix12_34, char3, 1},
		{matrix12_34, char3, 1},
		{matrix12_34, bigInt3, 1},
		{matrix12_34, bigRat3o7, 1},
		{matrix12_34, bigFloat3p5, 1},
		{matrix12_34, complex2j2, 1},
		{matrix12_34, vector012, 1},

		// Matrix comparisons.
		{matrix000_000, matrix12_34, 1}, // Length dominates.
		{matrix12_34, matrix12_44, -1},
		{matrix12_34, matrix12_34, 0},
		{matrix12_44, matrix12_34, 1},
	}
	var testConf config.Config
	c := exec.NewContext(&testConf)
	for _, test := range tests {
		got := value.OrderedCompare(c, test.u, test.v)
		if got != test.sgn {
			t.Errorf("orderedCompare(%T(%v), %T(%v)) = %d, expected %d", test.u, test.u, test.v, test.v, got, test.sgn)
		}
	}
}
