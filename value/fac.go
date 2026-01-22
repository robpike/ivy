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

// factorial returns !z for any z in the complex plane except negative integers
// (the poles of the gamma function). It is exact for natural numbers, an accurate
// approximate for all other values.
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

var (
	// The parameters for generating coefficients for our gamma apprximation. See
	// [The Gamma Function via Interpolation by Matthew F. Causley]: https://arxiv.org/pdf/2104.00697v1
	// The method is a refinement of Spouge's (1994) approximation for the gamma
	// function. For most values, the technique allows us to create a
	// significantly more accurate approximation than Lanczos, which only gets
	// to about 12 digits. In some parts of the complex plane this code can get
	// above 50 digits of accuracy. See ../testdata/gamma for details. N is the
	// number of coefficients. r is chosen as optimizing for best fit at z=6.
	// That choice is arbitrary. 256 bits of mantissa suffice to hold these
	// constants, with room.
	N                = 100
	r          Value = BigFloat{stringToFloat("r", "126.69", 256)}
	cInf       Value = BigFloat{stringToFloat("cInf", "2.5066", 256)}
	gammaCoeff []Value
)

// initGammaCoeff initializes the gamma coefficents for the modified Spouge
// approximation. We compute them with 256 bits of precision ~= 78 digits; our
// implementation with N=100 can only go to about 60 digits at best so there is no
// point in working harder.
func initGammaCoeff(c Context) {
	prevPrec := c.Config().FloatPrec()
	c.Config().SetFloatPrec(256)
	gammaCoeff = make([]Value, N)
	for i := range N {
		gammaCoeff[i] = cₙ(c, int64(i))
	}
	c.Config().SetFloatPrec(prevPrec)
}

// cₙ returns the nth constant for the modified Spouge approximation.
func cₙ(c Context, n int64) Value {
	// Shorthand for easier expression. We compute in Ivy Values because it's much easier.
	B := c.EvalBinary

	// In Ivy:
	//	t = (1 -1)[n&1]/!n
	//	u = e**r-n
	//	v = (r-n)**n+.5
	//	t*u*v

	sign := 1
	if n&1 == 1 {
		sign = -1
	}
	t := B(Int(sign), "/", BigInt{intFactorial(c, n)})
	rMinusN := B(r, "-", Int(n))
	u := exp(c, rMinusN)
	v := power(c, rMinusN, B(Int(n), "+", BigFloat{floatHalf}))
	return B(t, "*", B(u, "*", v))
}

// gamma returns an approximation to the gamma function 𝚪(z) using Causley's
// refinement to Spouge's approximation. It's good for 15 or more digits, and the
// parameters have been selected to give very high precision for areas near the
// origin. For modest integer z the values are almost indistinguishable from an
// integer (but integers are handled above.) The test in
// ../testdata/unary_bigfloat.ivy compares the value at -0.5 and shows an accuracy
// of 59+ digits.
// See ../testdata/gamma for details.
func gamma(c Context, z Value) Value {
	if gammaCoeff == nil {
		initGammaCoeff(c)
	}

	// Helpful constants.
	𝛑 := BigFloat{floatPi}
	half := BigFloat{floatHalf}
	// Shorthands for easier expression. We compute in Ivy Values because it
	// handles the mixed types well and performance is not critical.
	B := c.EvalBinary
	U := c.EvalUnary

	// Redirect for values with real(z)<0.5 using the reflection formula: 𝚪(z) = 𝛑/(sin(z𝛑)*𝚪(1-z)).
	real := z
	if cm, ok := z.(Complex); ok {
		real = cm.real
	}
	if isTrue(c, "!", B(real, "<", half)) {
		return B(𝛑, "/", B(sin(c, B(z, "*", 𝛑)), "*", gamma(c, B(one, "-", z))))
	}

	// In Ivy as a loop for easy comparison:
	//  p = (z+r)**z-.5
	//  q = ** -(z+r)
	//  sum = cinf
	//  n = 0
	//  :while n <= N-1
	//    sum = sum+c[n]/(z+n)
	//    n = n+1
	//  :end
	//  p*q*sum

	zPlusR := B(z, "+", r)
	p := power(c, zPlusR, B(z, "-", half))
	q := exp(c, U("-", zPlusR))
	sum := cInf
	for n, cn := range gammaCoeff {
		sum = B(sum, "+", B(cn, "/", B(z, "+", Int(n))))
	}
	return B(p, "*", B(q, "*", sum))
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
