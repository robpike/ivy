// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

// Implementation of factorial using the "swinging factorial" algorithm.
// from Peter Luschny, https://oeis.org/A000142/a000142.pdf.

// primeGen returns a function that generates primes from 2...n on successive calls.
// Used in the factorization in the swing function. (TODO: Might be fun to make available.)
func primeGen(n int) func() int {
	marked := make([]bool, n+1) // Starts at 0 for indexing simplicity.
	i := 2
	return func() int {
		for ; i <= n; i++ {
			if marked[i] {
				continue
			}
			for j := i; j <= n; j += i {
				marked[j] = true
			}
			return i
		}
		return 0
	}
}

// swing calculates the "swinging factorial" function of n,
// which is n!/⌊n/2⌋!².
// Swinging factorial table for reference.
//
//	n  0 1 2 3 4  5  6   7  8   9  10   11
//	n𝜎 1 1 2 6 6 30 20 140 70 630 252 2772
func swing(n int) *big.Int {
	nextPrime := primeGen(n)
	factors := make([]int, 0, 100)
	for {
		prime := nextPrime()
		if prime == 0 {
			break
		}
		q := n
		p := 1
		for q != 0 {
			q = q / prime
			if q&1 == 1 {
				p *= prime
			}
		}
		if p > 1 {
			factors = append(factors, p)
		}
	}
	return product(true, factors)
}

// product is called by swing to multiply the elements of the list.
// This recursive multiplication looks slower but is
// actually faster for many large numbers, despite the allocations
// required to build the list in the swing factorial calculation.
func product1(f []int) *big.Int {
	switch len(f) {
	case 0:
		return big.NewInt(1)
	case 1:
		return big.NewInt(int64(f[0]))
	}
	n := len(f) / 2
	left := product1(f[:n])
	right := product1(f[n:])
	return left.Mul(left, right)
}

// product is called by swing to multiply the elements of the list.
// This recursive multiplication looks slower but is
// actually faster for many large numbers, despite the allocations
// required to build the list in the swing factorial calculation.
func product(doPar bool, f []int) *big.Int {
	switch len(f) {
	case 0:
		return big.NewInt(1)
	case 1:
		return big.NewInt(int64(f[0]))
	}
	n := len(f) / 2
	left := product1(f[:n])
	right := product1(f[n:])
	r := left.Mul(left, right)
	return r
}

// factorial returns factorial of n using a "swinging
// factorial" for roughly 2x speedup.
func factorial(c Context, n int64) *big.Int {
	if n < 0 {
		c.Errorf("negative value %d for factorial", n)
	}
	if n < 2 {
		return big.NewInt(1)
	}
	s := swing(int(n))
	f2 := factorial(c, n/2)
	f2.Mul(f2, f2)
	f2.Mul(f2, s)
	return f2
}
