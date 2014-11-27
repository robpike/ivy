// Copyright 2014 Rob Pike. All rights reserved.
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

Semicolons separate multiple statements on a line. Variables are alphanumeric and are
assigned with the = operator.

The APL operators, adapted from http://en.wikipedia.org/wiki/APL_syntax_and_symbols,
and their correpondence are listed here. The correspondence is incomplete and inexact.

Unary functions.

	Name              APL   Ivy     Meaning
	Roll              ?B    ?       One integer selected randomly from the first B integers
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
	Reversal          ⌽B    rev     Reverse elements of B along last axis
	Reversal          ⊖B    flip    Reverse elements of B along first axis
	Grade up          ⍋B    up      Indices of B which will arrange B in ascending order
	Grade down        ⍒B    down    Indices of B which will arrange B in descending order
	Execute           ⍎B            Execute an APL expression
	Monadic format    ⍕B            A character representation of B
	Monadic transpose ⍉B            Reverse the axes of B
	Factorial         !B            Product of integers 1 to B
	Bitwise not             ^       Bitwise complement of B (integer only)

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
	Membership            A∈B           1 for elements of A present in B; 0 where not.
	Maximum               A⌈B   max     The greater value of A or B
	Minimum               A⌊B   min     The smaller value of A or B
	Reshape               A⍴B   rho     Array of shape A with data B
	Take                  A↑B   take    Select the first (or last) A elements of B according to ×A
	Drop                  A↓B   drop    Remove the first (or last) A elements of B according to ×A
	Decode                A⊥B           Value of a polynomial whose coefficients are B at A
	Encode                A⊤B           Base-A representation of the value of B
	Residue               A∣B           B modulo A
	                            mod     A modulo B (Euclidean)
	                            imod    A modulo B (Go)
	Catenation            A,B   ,       Elements of B appended to the elements of A
	Expansion             A\B           Insert zeros (or blanks) in B corresponding to zeros in A
	Compression           A/B           Select elements in B corresponding to ones in A
	Index of              A⍳B           The location (index) of B in A; 1+⌈/⍳⍴A if not found
	Matrix divide         A⌹B           Solution to system of linear equations Ax = B
	Rotation              A⌽B           The elements of B are rotated A positions
	Rotation              A⊖B           The elements of B are rotated A positions along the first axis
	Logarithm             A⍟B           Logarithm of B to base A
	Dyadic format         A⍕B           Format B into a character matrix according to A
		General transpose     A⍉B           The axes of B are ordered by A
	Combinations          A!B           Number of combinations of B taken A at a time
	Less than             A<B   <       Comparison: 1 if true, 0 if false
	Less than or equal    A≤B   <=      Comparison: 1 if true, 0 if false
	Equal                 A=B   ==      Comparison: 1 if true, 0 if false
	Greater than or equal A≥B   >=      Comparison: 1 if true, 0 if false
	Greater than          A>B   >       Comparison: 1 if true, 0 if false
	Not equal             A≠B   !=      Comparison: 1 if true, 0 if false
	Or                    A∨B   or      Logic: 0 if A and B are 0; 1 otherwise
	And                   A∧B   and     Logic: 1 if A and B are 1; 0 otherwise
	Nor                   A⍱B   nor     Logic: 1 if both A and B are 0; otherwise 0
	Nand                  A⍲B   nand    Logic: 0 if both A and B are 1; otherwise 1
	Xor                         xor     Logic: 1 if A != B; otherwise 0
	Bitwise and                 &       Bitwise A and B (integer only)
	Bitwise or                  |       Bitwise A or B (integer only)
	Bitwise xor                 ^       Bitwise A exclusive or B (integer only)
	Left shift                  <<      A shifted left B bits (integer only)
	Right Shift                 >>      A shifted right B bits (integer only)

Operators and axis indicator

	Name                APL  Ivy  APL Example  Ivy Example  Meaning (of example)
	Reduce (last axis)  /    /    +/B          +/B          Sum across B
	Reduce (first axis) ⌿         +⌿B                       Sum down B
	Scan (last axis)    \    \    +\B          +\B          Running sum across B
	Scan (first axis)   ⍀         +⍀B                       Running sum down B
	Inner product       .    .    A+.×B        A +.* B      Matrix product of A and B
	Outer product       ∘.   o.   A∘.×B        A o.* B      Outer product of A and B
                                                            (lower case o; may need preceding space)
User-defined operators

Users can define unary and binary operators, which then behave just like built-in
operators. The syntax of a definition is the 'def' keyword, the operator and
formal arguments, an equals sign, and then the body. The name must be an identifier.
The final expression of the body is the return value. The same name may be defined
both as a unary and as a binary.

Example: average of a vector (unary):
	def avg x = (+/x)/rho x
	avg iota 11
	result: 6

Example: n largest entries in a vector (binary):
	def n largest x = n take x[down x]
	3 largest 7 1 3 24 1 5 12 5 51
	result: 51 24 12

To declare an operator but not define it, omit the equals sign and what follows.
	def foo x
	def bar x = foo x
	def foo x = -x
	bar 3
		-3

Within a user-defined operator, identifiers are local to the invocation unless
they are undefined in the operator but defined globally, in which case they refer to
the global variable. A mechanism to declare locals may come later.

At the moment the body must be a single line but expressions can be separated by semicolons.

Special commands

Ivy accepts a number of special commands, introduced by a right paren
at the beginning of the line. Most report the current value if a new value
is not specified. For these commands, numbers are always base 10.

	) base 0
		Set the number base for input and output. The commands
		ibase and obase control setting of the base for input
		and output alone, respectively.
		Base 0 allows C-style input: decimal, with 037 being octal
		and 0x10 being hexadecimal.
		If the base is greater than 10, any identifier formed from
		valid numerals in the base system, such as abe for base 16,
		is taken to be a number.
		TODO: To output rationals and bigs, obase must be one of 0 2 8 10 16.
	) debug name 0|1
		Toggle or set the named debugging flag. With no argument,
		lists the settings.
	) format ""
		Set the format for printing values. If empty, the output
		is printed using the output base. If non-empty, the
		format determines the base used in printing.
		The format is in the style of golang.org/pkg/fmt.
		For floating-point formats, flags and width are ignored.
	) get "file.ivy"
		Read commands from the named file; return to
		interactive execution afterwards.
	) op X
		Show the definition of the user-defined operator X.
	) origin 1
		Set the origin for indexing a vector or matrix.
	) prompt ""
		Set the interactive prompt.
	) seed 0
		Set the seed for the ? operator.

*/
package main
