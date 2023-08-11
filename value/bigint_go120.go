// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !go1.21

package value

import "math/big"

// Float64 returns the float64 value nearest x,
// and an indication of any rounding that occurred.
//
// The implementation is backported from Go 1.21.0's
// math/big.Int.Float64 implementation.
func (i BigInt) Float64() (float64, big.Accuracy) {
	n := i.Int.BitLen()
	if n == 0 {
		return 0.0, big.Exact
	}

	// Fast path: no more than 53 significant bits.
	if n <= 53 || n < 64 && n-int(i.Int.TrailingZeroBits()) <= 53 {
		f := float64(i.Int.Uint64())
		if i.Int.Sign() == -1 {
			f = -f
		}
		return f, big.Exact
	}

	return new(big.Float).SetInt(i.Int).Float64()
}
