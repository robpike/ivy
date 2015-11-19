// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*

Ivy is an interpreter for an APL-like language. It is a plaything and a work in
progress.

Unlike APL, the input is ASCII and the results are exact (but see the next paragraph).
It uses exact rational arithmetic so it can handle arbitrary precision. Values to be
input may be integers (3, -1), rationals (1/3, -45/67) or floating point values (1e3,
-1.5 (representing 1000 and -3/2)).

Some functions such as sqrt are irrational. When ivy evaluates an irrational
function, the result is stored in a high-precision floating-point number (default
256 bits of mantissa). Thus when using irrational functions, the values have high
precision but are not exact.

Unlike in most other languages, operators always have the same precedence and
expressions are evaluated in right-associative order. That is, unary operators
apply to everything to the right, and binary operators apply to the operand
immediately to the left and to everything to the right.  Thus, 3*4+5 is 27 (it
groups as 3*(4+5)) and iota 3+2 is 1 2 3 4 5 while 3+iota 2 is 3 4 5. A vector
is a single operand, so 1 2 3 + 3 + 3 4 5 is (1 2 3) + 3 + (3 4 5), or 7 9 11.

Only a subset of APL's functionality is implemented, but the intention is to
have most numerical operations supported eventually.

Semicolons separate multiple statements on a line. Variables are alphanumeric and are
assigned with the = operator.

After each successful expression evaluation, the result is stored in the variable
called _ (underscore) so it can be used in the next expression.

The APL operators, adapted from https://en.wikipedia.org/wiki/APL_syntax_and_symbols,
and their correspondence are listed here. The correspondence is incomplete and inexact.

Unary functions.

	Name              APL   Ivy     Meaning
	Roll              ?B    ?       One integer selected randomly from the first B integers
	Ceiling           ⌈B    ceil    Least integer greater than or equal to B
	Floor             ⌊B    floor   Greatest integer less than or equal to B
	Shape             ⍴B    rho     Number of components in each dimension of B
	Not               ∼B    not     Logical: not 1 is 0, not 0 is 1
	Absolute value    ∣B    abs     Magnitude of B
	Index generator   ⍳B    iota    Vector of the first B integers
	Exponential       ⋆B    **      e to the B power
	Negation          −B    -       Changes sign of B
	Identity          +B    +       No change to B
	Signum            ×B    sgn     ¯1 if B<0; 0 if B=0; 1 if B>0
	Reciprocal        ÷B    /       1 divided by B
	Ravel             ,B    ,       Reshapes B into a vector
	Matrix inverse    ⌹B            Inverse of matrix B
	Pi times          ○B            Multiply by π
	Logarithm         ⍟B    log     Natural logarithm of B
	Reversal          ⌽B    rev     Reverse elements of B along last axis
	Reversal          ⊖B    flip    Reverse elements of B along first axis
	Grade up          ⍋B    up      Indices of B which will arrange B in ascending order
	Grade down        ⍒B    down    Indices of B which will arrange B in descending order
	Execute           ⍎B    ivy     Execute an APL (ivy) expression
	Monadic format    ⍕B    text    A character representation of B
	Monadic transpose ⍉B            Reverse the axes of B
	Factorial         !B            Product of integers 1 to B
	Bitwise not             ^       Bitwise complement of B (integer only)
	Square root       B⋆.5  sqrt    Square root of B.
	Sine                    sin     sin(A); APL uses binary ○ (see below)
	Cosine                  cos     cos(A); ditto
	Tangent                 tan     tan(A); ditto

Binary functions.

	Name                  APL   Ivy     Meaning
	Add                   A+B   +       Sum of A and B
	Subtract              A−B   -       A minus B
	Multiply              A×B   *       A multiplied by B
	Divide                A÷B   /       A divided by B (exact rational division)
	                            div     A divided by B (Euclidean)
	                            idiv    A divided by B (Go)
	Exponentiation        A⋆B   **      A raised to the B power
	Circle                A○B           Trigonometric functions of B selected by A
	                                    A=1: sin(B) A=2: cos(B) A=3: tan(B); ¯A for inverse
	                            sin     sin(B); ivy uses traditional name.
	                            cos     cos(B); ivy uses traditional name.
	                            tan     tan(B); ivy uses traditional name.
	                            asin    arcsin(B); ivy uses traditional name.
	                            acos    arccos(B); ivy uses traditional name.
	                            atan    arctan(B); ivy uses traditional name.
	Deal                  A?B           A distinct integers selected randomly from the first B integers
	Membership            A∈B   in      1 for elements of A present in B; 0 where not.
	Maximum               A⌈B   max     The greater value of A or B
	Minimum               A⌊B   min     The smaller value of A or B
	Reshape               A⍴B   rho     Array of shape A with data B
	Take                  A↑B   take    Select the first (or last) A elements of B according to ×A
	Drop                  A↓B   drop    Remove the first (or last) A elements of B according to ×A
	Decode                A⊥B   decode  Value of a polynomial whose coefficients are B at A
	Encode                A⊤B   encode  Base-A representation of the value of B
	Residue               A∣B           B modulo A
	                            mod     A modulo B (Euclidean)
	                            imod    A modulo B (Go)
	Catenation            A,B   ,       Elements of B appended to the elements of A
	Expansion             A\B   fill    Insert zeros (or blanks) in B corresponding to zeros in A
	                                    In ivy: abs(A) gives count, A <= 0 inserts zero (or blank)
	Compression           A/B   sel     Select elements in B corresponding to ones in A
	                                    In ivy: abs(A) gives count, A <= 0 inserts zero
	Index of              A⍳B   iota    The location (index) of B in A; 1+⌈/⍳⍴A if not found
	                                    In ivy: origin-1 if not found (i.e. 0 if one-indexed)
	Matrix divide         A⌹B           Solution to system of linear equations Ax = B
	Rotation              A⌽B   rot     The elements of B are rotated A positions left
	Rotation              A⊖B           The elements of B are rotated A positions along the first axis
	Logarithm             A⍟B   log     Logarithm of B to base A
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

Type-converting operations

	Name                  Ivy      Meaning
	Code                  code B   The integer Unicode value of char B
	Char                  char B   The character with integer Unicode value B
	Float                 float B  The floating-point representation of B
	Text                  text B   The textual (vector of Char) representation of B
	Evaluate              ivy B    The result of evaluating B as ivy program text

Pre-defined constants

The constants e (base of natural logarithms) and pi (π) are pre-defined to high
precision, about 3000 decimal digits truncated according to the floating point
precision setting.

Character data

Strings are vectors of "chars", which are Unicode code points (not bytes).
Syntactically, string literals are very similar to those in Go, with back-quoted
raw strings and double-quoted interpreted strings. Unlike Go, single-quoted strings
are equivalent to double-quoted, a nod to APL syntax. A string with a single char
is just a singleton char value; all others are vectors. Thus ``, "", and '' are
empty vectors, `a`, "a", and 'a' are equivalent representations of a single char,
and `ab`, `a` `b`, "ab", "a" "b", 'ab', and 'a' 'b' are equivalent representations
of a two-char vector.

Unlike in Go, a string in ivy comprises code points, not bytes; as such it can
contain only valid Unicode values. Thus in ivy "\x80" is illegal, although it is
a legal one-byte string in Go.

Strings can be printed. If a vector contains only chars, it is printed without
spaces between them.

Chars have restricted operations. Printing, comparison, indexing and so on are
legal but arithmetic is not, and chars cannot be converted automatically into other
singleton values (ints, floats, and so on). The unary operators char and code
enable transcoding between integer and char values.

User-defined operators

Users can define unary and binary operators, which then behave just like
built-in operators. Both a unary and a binary operator may be defined for the
same name.

The syntax of a definition is the 'op' keyword, the operator and formal
arguments, an equals sign, and then the body. The names of the operator and its
arguments must be identifiers.  For unary operators, write "op name arg"; for
binary write "op leftarg name rightarg". The final expression in the body is the
return value. Operators may have recursive definitions, but since there are
no conditional or looping constructs (yet), such operators are problematic
when executed.

The body may be a single line (possibly containing semicolons) on the same line
as the 'op', or it can be multiple lines. For a multiline entry, there is a
newline after the '=' and the definition ends at the first blank line (ignoring
spaces).

Example: average of a vector (unary):
	op avg x = (+/x)/rho x
	avg iota 11
	result: 6

Example: n largest entries in a vector (binary):
	op n largest x = n take x[down x]
	3 largest 7 1 3 24 1 5 12 5 51
	result: 51 24 12

Example: multiline operator definition (binary):
	op a sum b =
		a = a+b
		a

	iota 3 sum 4
	result: 1 2 3 4 5 6 7

To declare an operator but not define it, omit the equals sign and what follows.
	op foo x
	op bar x = foo x
	op foo x = -x
	bar 3
	result: -3
	op foo x = /x
	bar 3
	result: 1/3

Within a user-defined operator, identifiers are local to the invocation unless
they are undefined in the operator but defined globally, in which case they refer to
the global variable. A mechanism to declare locals may come later.

Special commands

Ivy accepts a number of special commands, introduced by a right paren
at the beginning of the line. Most report the current value if a new value
is not specified. For these commands, numbers are always base 10 and
must be non-negative.

	) help
		Print this list of special commands.
	) base 0
		Set the number base for input and output. The commands ibase and
		obase control setting of the base for input and output alone,
		respectively.  Base 0 allows C-style input: decimal, with 037 being
		octal and 0x10 being hexadecimal. If the base is greater than 10,
		any identifier formed from valid numerals in the base system, such
		as abe for base 16, is taken to be a number. TODO: To output
		large integers and rationals, base must be one of 0 2 8 10 16.
	) debug name 0|1
		Toggle or set the named debugging flag. With no argument, lists
		the settings.
	) format ""
		Set the format for printing values. If empty, the output is printed
		using the output base. If non-empty, the format determines the
		base used in printing. The format is in the style of golang.org/pkg/fmt.
		For floating-point formats, flags and width are ignored.
	) get "save.ivy"
		Read input from the named file; return to interactive execution
		afterwards. If no file is specified, read from "save.ivy".
		(Unimplemented on mobile.)
	) maxbits 1e6
		To avoid consuming too much memory, if an integer result would
		require more than this many bits to store, abort the calculation.
		If maxbits is 0, there is no limit; the default is 1e6.
	) maxdigits 1e4
		To avoid overwhelming amounts of output, if an integer has more
		than this many digits, print it using the defined floating-point
		format. If maxdigits is 0, integers are always printed as integers.
	) op X
		Show the definition of the user-defined operator X. Inside the
		definition, numbers are always shown base 10, ignoring the ibase
		and obase.
	) origin 1
		Set the origin for indexing a vector or matrix.
	) prec 256
		Set the precision (mantissa length) for floating-point values.
		The value is in bits. The exponent always has 32 bits.
	) prompt ""
		Set the interactive prompt.
	) save "save.ivy"
		Write definitions of user-defined operators and variables to the
		named file, as ivy textual source. If no file is specified, save to
		"save.ivy".
		(Unimplemented on mobile.)
	) seed 0
		Set the seed for the ? operator.

*/
package main
