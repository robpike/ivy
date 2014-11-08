// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*

Ivy is an interpreter for an APL-like language. It is a plaything and a work in
progress.

Unlike APL, the input is ASCII and the results are exact. It uses exact rational
arithmetic so it can handle arbitrary precision but does not implement any
irrational calculations. Values to be input may be integers (3, -1), rationals
(1/3, -45/67) or floating point values (1e3, -1.5 (representing 1000 and -3/2)).

Only a subset of APL's functionality is implemented, but the intention is to
have most numerical operations supported eventually. To achieve this, some form of
high-precision floating-point arithmetic may appear.

The APL operators, adapted from http://en.wikipedia.org/wiki/APL_syntax_and_symbols,
and their correpondence are listed here. The correspondence is incomplete and inexact.

Unary functions.

Name              APL   Ivy     Meaning
Roll              ?B            One integer selected randomly from the first B integers
Ceiling           ⌈B    ceil    Least integer greater than or equal to B
Floor             ⌊B    floor   Greatest integer less than or equal to B
Shape             ⍴B    rho     Number of components in each dimension of B
Not               ∼B    ~       Logical: ∼1 is 0, ∼0 is 1
Absolute value    ∣B    abs      Magnitude of B
Index generator   ⍳B    iota    Vector of the first B integers
Exponential       ⋆B            e to the B power
Negation          −B    -       Changes sign of B
Identity          +B    +       No change to B
Signum            ×B    sgn     ¯1 if B<0; 0 if B=0; 1 if B>0
Reciprocal        ÷B    /       1 divided by B
Ravel             ,B    ,       Reshapes B into a vector
Matrix inverse    ⌹B            Inverse of matrix B
Pi times          ○B            Multiply by π
Logarithm         ⍟B            Natural logarithm of B
Reversal          ⌽B            Reverse elements of B along last axis
Reversal          ⊖B            Reverse elements of B along first axis
Grade up          ⍋B            Indices of B which will arrange B in ascending order
Grade down        ⍒B            Indices of B which will arrange B in descending order
Execute           ⍎B            Execute an APL expression
Monadic format    ⍕B            A character representation of B
Monadic transpose ⍉B            Reverse the axes of B
Factorial         !B            Product of integers 1 to B

Binary functions.

Name                  APL   Ivy     Meaning
Add                   A+B   +       Sum of A and B
Subtract              A−B   -       A minus B
Multiply              A×B   *       A multiplied by B
Divide                A÷B   /       A divided by B (exact rational division)
                            div     A divided by B (Euclidean)
                            idiv    A divided by B (Go)
Exponentiation        A⋆B           A raised to the B power
                            **      A raised to the B power; B must be an integer.
Circle                A○B           Trigonometric functions of B selected by A
                                    A=1: sin(B) A=2: cos(B) A=3: tan(B)
Deal                  A?B           A distinct integers selected randomly from the first B integers
Membership            A∈B          1 for elements of A present in B; 0 where not.
Maximum               A⌈B   max    The greater value of A or B
Minimum               A⌊B   min    The smaller value of A or B
Reshape               A⍴B   rho    Array of shape A with data B
Take                  A↑B          Select the first (or last) A elements of B according to ×A
Drop                  A↓B          Remove the first (or last) A elements of B according to ×A
Decode                A⊥B          Value of a polynomial whose coefficients are B at A
Encode                A⊤B          Base-A representation of the value of B
Residue               A∣B          B modulo A
                            mod    A modulo B (Euclidean)
                            imod   A modulo B (Go)
Catenation            A,B   ,      Elements of B appended to the elements of A
Expansion             A\B          Insert zeros (or blanks) in B corresponding to zeros in A
Compression           A/B          Select elements in B corresponding to ones in A
Index of              A⍳B          The location (index) of B in A; 1+⌈/⍳⍴A if not found
Matrix divide         A⌹B          Solution to system of linear equations Ax = B
Rotation              A⌽B          The elements of B are rotated A positions
Rotation              A⊖B          The elements of B are rotated A positions along the first axis
Logarithm             A⍟B          Logarithm of B to base A
Dyadic format         A⍕B          Format B into a character matrix according to A
General transpose     A⍉B          The axes of B are ordered by A
Combinations          A!B          Number of combinations of B taken A at a time
Less than             A<B   <      Comparison: 1 if true, 0 if false
Less than or equal    A≤B   <=     Comparison: 1 if true, 0 if false
Equal                 A=B   ==     Comparison: 1 if true, 0 if false
Greater than or equal A≥B   >=     Comparison: 1 if true, 0 if false
Greater than          A>B   >      Comparison: 1 if true, 0 if false
Not equal             A≠B   !=     Comparison: 1 if true, 0 if false
Or                    A∨B   or     Logic: 0 if A and B are 0; 1 otherwise
And                   A∧B   and    Logic: 1 if A and B are 1; 0 otherwise
Nor                   A⍱B          Logic: 1 if both A and B are 0; otherwise 0
Nand                  A⍲B          Logic: 0 if both A and B are 1; otherwise 1

Operators and axis indicator

Name                APL  Ivy  APL Example  Ivy Example  Meaning (of example)
Reduce (last axis)  /    /    +/B          +/b          Sum across B
Reduce (first axis) ⌿         +⌿B                       Sum down B
Scan (last axis)    \         +\B                       Running sum across B
Scan (first axis)   ⍀         +⍀B                       Running sum down B
Inner product       .    .    A+.×B        +.*          Matrix product of A and B
Outer product       ∘.        A∘.×B                     Outer product of A and B

*/
package main
