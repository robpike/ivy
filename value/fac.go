// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

// Implementation of factorial using the "swinging factorial" algorithm
// for integers. From Peter Luschny, https://oeis.org/A000142/a000142.pdf.

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

// product1 implements one half of product.
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

// intFactorial returns factorial of natural n using a "swinging
// factorial" for roughly 2x speedup.
func intFactorial(c Context, n int64) *big.Int {
	if n < 0 {
		c.Errorf("factorial of negative number: %d", n)
	}
	if n < 2 {
		return big.NewInt(1)
	}
	s := swing(int(n))
	f2 := intFactorial(c, n/2)
	f2.Mul(f2, f2)
	f2.Mul(f2, s)
	return f2
}

// factorial returns an approximation to !z for any z in the complex plane except
// negative integers (the poles of the gamma function).
func factorial(c Context, z Value) Value {
	if i, ok := z.(Int); ok {
		if i < 0 {
			c.Errorf("factorial of negative integer: %d", i)
		}
		return BigInt{intFactorial(c, int64(i))}.shrink()
	}
	// z! = 𝚪(z+1))
	return gamma(c, c.EvalBinary(z, "+", one))
}

// gamma returns an approximation to r using the Lanczos approximation. It's
// only good for about 10-12 digits. Based on the Python code from
// [Wikipedia]: https://en.wikipedia.org/wiki/Lanczos_approximation
// with coefficients c (renamed p here) from Paul Godfrey's work,
// [Implementing the  Gamma Function]: https://www.numericana.com/answer/info/godfrey.htm.
// I have tried many different constant sets and the answer doesn't change much, so
// let's just use what the expert suggests. I believe it is infeasible to expect
// significantly higher precision without substantially more work.
func gamma(c Context, z Value) Value {
	// p values can be recomputed using ../testdata/lanczos.
	p := []float64{
		1.000000000000000174663,
		5716.400188274341379136,
		-14815.30426768413909044,
		14291.49277657478554025,
		-6348.160217641458813289,
		1301.608286058321874105,
		-108.1767053514369634679,
		2.605696505611755827729,
		-0.7423452510201416151527e-2,
		0.5384136432509564062961e-7,
		-0.4023533141268236372067e-8,
	}
	g := Int(len(p) - 2)

	// Helpful constants.
	𝛑 := BigFloat{floatPi}
	half := BigFloat{floatHalf}
	sqrt2pi := sqrt(c, c.EvalBinary(NewComplex(c, 𝛑, Int(0)), "*", Int(2)))

	// Unlike the rest of the numerical functions here, we stay in Ivy value space
	// as we are mixing integers, floats, and complex numbers freely. It's annoying
	// to track it all explicitly and we are not concerned about performance.

	real := z
	if cm, ok := z.(Complex); ok {
		real = cm.real
	}
	if isTrue(c, "!", c.EvalBinary(real, "<", BigFloat{floatHalf})) {
		// Redirect using the reflection formula: 𝚪(z) = 𝛑/(sin(z𝛑)*𝚪(1-z)).
		x := c.EvalBinary(sin(c, c.EvalBinary(𝛑, "*", z)), "*", gamma(c, c.EvalBinary(one, "-", z)))
		return c.EvalBinary(𝛑, "/", x)
	}
	// z = z-1
	z = c.EvalBinary(z, "-", one)
	// x = p[0]
	var x Value = BigFloat{newFloat(c).SetFloat64(p[0])}
	for i := 1; i < len(p); i++ {
		// x += p[i] / (z + i)
		x = c.EvalBinary(x, "+", c.EvalBinary(BigFloat{newFloat(c).SetFloat64(p[i])}, "/", c.EvalBinary(z, "+", Int(i))))
	}
	// t = z + g + 0.5
	t := c.EvalBinary(z, "+", c.EvalBinary(g, "+", half))
	// y = sqrt(2 * pi) * t**(z + 0.5) * exp(-t) * x
	y := sqrt2pi
	y = c.EvalBinary(y, "*", power(c, t, c.EvalBinary(z, "+", half)))
	y = c.EvalBinary(y, "*", c.EvalBinary(exp(c, c.EvalUnary("-", t)), "*", x))
	return y
}

// binomial returns the binomial value, written U!V, which in APL notation is (!V)÷(!U)×!V-U.
// Accuracy outside the natural numbers is limited by the accuracy of our gamma function.
func binomial(c Context, u, v Value) Value {
	vFac := factorial(c, v)
	uFac := factorial(c, u)
	vMinusUFac := factorial(c, c.EvalBinary(v, "-", u))
	t := c.EvalBinary(uFac, "*", vMinusUFac)
	if isZero(t) {
		c.Errorf("zero in binomial")
	}
	return c.EvalBinary(vFac, "/", t)
}
