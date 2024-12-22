// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The precise format and contents of this file are depended
// on by parse/helpgen.go. Do not edit without verifying that
// )help still works properly.

/*
Ivy is an interpreter for an APL-like language. It is a plaything and a work in
progress.

Unlike APL, the input is ASCII and the results are exact (but see
the next paragraph).  It uses exact rational arithmetic so it can
handle arbitrary precision. Values to be input may be integers (3,
-1), rationals (1/3, -45/67) or floating point values (1e3, -1.5
(representing 1000 and -3/2)).

Some functions such as sqrt are irrational. When ivy evaluates an
irrational function, the result is stored in a high-precision
floating-point number (default 256 bits of mantissa). Thus when
using irrational functions, the values have high precision but are
not exact.

Unlike in most other languages, operators always have the same
precedence and expressions are evaluated in right-associative order.
That is, unary operators apply to everything to the right, and
binary operators apply to the operand immediately to the left and
to everything to the right.  Thus, 3*4+5 is 27 (it groups as 3*(4+5))
and iota 3+2 is 1 2 3 4 5 while 3+iota 2 is 4 5. A vector is a
single operand, so 1 2 3 + 3 + 3 4 5 is (1 2 3) + 3 + (3 4 5), or
7 9 11.

As a special but important case, note that 1/3, with no intervening
spaces, is a single rational number, not the expression 1 divided
by 3. This can affect precedence: 3/6*4 is 2 while 3 / 6*4 is 1/8
since the spacing turns the / into a division operator. Use parentheses
or spaces to disambiguate: 3/(6*4) or 3 /6*4.

Ivy has complex numbers, which are constructed using the unary or
binary j operator. As with rationals, the token 1j2 (the representation
of 1+2i) is a single token. The individual parts can be rational,
so 1/2j-3/2 is the complex number 0.5-1.5i and scans as a single
value.

Indexing uses [] notation: x[1], x[1; 2], and so on. Indexing by a
vector selects multiple elements: x[1 2] creates a new item from
x[1] and x[2]. An empty index slot is a shorthand for all the
elements along that dimension, so x[] is equivalent to x, and x[;3]
gives the third column of two-dimensional array x.

Only a subset of APL's functionality is implemented, but all numerical
operations are supported.

Semicolons separate multiple statements on a line. Variables are
alphanumeric and are assigned with the = operator. Assignment is
an expression.

After each successful expression evaluation, the result is stored
in the variable called _ (underscore) so it can be used in the next
expression.

The APL operators, adapted from
https://en.wikipedia.org/wiki/APL_syntax_and_symbols, and their
correspondence are listed here. The correspondence is incomplete
and inexact.

Unary operators

	Name              APL   Ivy     Meaning
	Roll              ?B    ?       One integer selected randomly from the first B integers
	Ceiling           ⌈B    ceil    Least integer greater than or equal to B
	                                If B is complex, the complex ceiling, as defined by McDonnell
	Floor             ⌊B    floor   Greatest integer less than or equal to B
	                                If B is complex, the complex floor, as defined by McDonnell
	Shape             ⍴B    rho     Vector of number of components in each dimension of B
	Count             ≢B    count   Scalar number of elements at top level of B
	Flatten           ∊B    flatten Vector of all the scalar elements within B
	Not               ∼B    not     Logical: not 1 is 0, not 0 is 1
	Absolute value    ∣B    abs     Magnitude of B
	Index generator   ⍳B    iota    Vector of the first B integers
	                                If B is a vector, matrix of coordinates
	Unique            ∪B    unique  Remove all duplicate elements from B
	Enclose           ⊂B    box     Wrap B in one level of nesting
	Disclose          ⊃B    first   First element of B in ravel order
	Split             ↓B    split   Create vector of nested elements from matrix B; inverse of mix
	Mix               ↑B    mix     Create matrix from elements of vector B; inverse of split
	Exponential       ⋆B    **      e to the B power
	Negation          −B    -       Change sign of B
	Identity          +B    +       No change to B
	Signum            ×B    sgn     -1 if B<0; 0 if B=0; 1 if B>0. More generally: B/abs B if B!=0
	Reciprocal        ÷B    /       1 divided by B
	Ravel             ,B    ,       Reshapes B into a vector
	Matrix inverse    ⌹B    inv     Inverse of B; for vector (conj v)/v+.*conj v
	Pi times          ○B            Multiply by π
	Logarithm         ⍟B    log     Natural logarithm of B
	Reversal          ⌽B    rot     Reverse elements of B along last axis
	Reversal          ⊖B    flip    Reverse elements of B along first axis
	Grade up          ⍋B    up      Indices of B which will arrange B in ascending order
	Grade down        ⍒B    down    Indices of B which will arrange B in descending order
	Execute           ⍎B    ivy     Execute an APL (ivy) expression
	Monadic format    ⍕B    text    A character representation of B
	Monadic transpose ⍉B    transp  Reverse the axes of B
	Factorial         !B    !       Product of integers 1 to B
	Bitwise not             ^       Bitwise complement of B (integer only)
	Square root       B⋆.5  sqrt    Square root of B.
	Sine                    sin     sin(A); APL uses binary ○ (see below)
	Cosine                  cos     cos(A); ditto
	Tangent                 tan     tan(A); ditto
	Arcsine                 asin    arcsin(B)
	Arccosine               acos    arccos(B)
	Arctangent              atan    arctan(B)
	Hyperbolic sine         sinh    sinh(B)
	Hyperbolic cosine       cosh    cosh(B)
	Hyperbolic tangent      tanh    tanh(B)
	Hyperbolic arcsine      asinh   arcsinh(B)
	Hyperbolic arccosine    acosh   arccosh(B)
	Hyperbolic arctangent   atanh   arctanh(B)
	Rotation by 90°         j       Multiplication by sqrt(-1)
	Real part               real    Real component of the value
	Imaginary part          imag    Imaginary component of the value
	Phase                   phase   Phase of the value in the complex plane (-π to π)
	Conjugate         +B    conj    Complex conjugate of the value
	System functions  ⎕     sys     Argument is a string; run "sys 'help'" for details
	Print                   print   Print and evaluate to argument; useful for debugging

Binary operators

	Name                  APL   Ivy       Meaning
	Add                   A+B   +         Sum of A and B
	Subtract              A−B   -         A minus B
	Multiply              A×B   *         A multiplied by B
	Divide                A÷B   /         A divided by B (exact rational division)
	                            div       A divided by B (Euclidean)
	                            idiv      A divided by B (Go)
	Exponentiation        A⋆B   **        A raised to the B power
	Circle                A○B             Trigonometric functions of B selected by A
	                                      A=1: sin(B) A=2: cos(B) A=3: tan(B); ¯A for inverse
	                            sin       sin(B); ivy uses traditional name.
	                            cos       cos(B); ivy uses traditional name.
	                            tan       tan(B); ivy uses traditional name.
	Deal                  A?B   ?         A distinct integers selected randomly from the first B integers
	Membership            A∈B   in        1 for elements of A present in B; 0 where not.
	Intersection          A∩B   intersect A with all elements not in B removed
	Union                 A∪B   union     A followed by all members of B not already in A
	Maximum               A⌈B   max       The greater value of A or B
	Minimum               A⌊B   min       The smaller value of A or B
	Reshape               A⍴B   rho       Array of shape A with data B
	Take                  A↑B   take      Select the first (or last) A elements of B according to sgn A
	Drop                  A↓B   drop      Remove the first (or last) A elements of B according to sgn A
	Decode                A⊥B   decode    Value of a polynomial whose coefficients are B at A
	                                      'T' decode B creates a seconds value from the time vector B
	Encode                A⊤B   encode    Base-A representation of the value of B
	                                      'T' encode B creates a time vector from the seconds value B
	Residue               A∣B              B modulo A
	                            mod       A modulo B (Euclidean)
	                            imod      A modulo B (Go)
	Catenation            A,B   ,         Elements of B appended to the elements of A along last axis
	Catenation            A,B   ,%        Elements of B appended to the elements of A along first axis
	Expansion             A\B   fill      Insert zeros (or blanks) in B corresponding to zeros in A
	                                      In ivy: abs(A) gives count, A <= 0 inserts zero (or blank)
	Compression           A/B   sel       Select elements in B corresponding to ones in A
	                                      In ivy: abs(A) gives count, A <= 0 inserts zero
	Index of              A⍳B   iota      The location (index) of B in A; 1+⌈/⍳⍴A if not found
	                                      In ivy: origin-1 if not found (i.e. 0 if one-indexed)
	Matrix divide         A⌹B   mdiv      Solution to system of linear equations Bx = A
	                                      For real vectors, the magnitude of A projected on B
	Rotation              A⌽B   rot       The elements of B are rotated A positions left
	Rotation              A⊖B   flip      The elements of B are rotated A positions along the first axis
	Logarithm             A⍟B   log       Logarithm of B to base A
	Dyadic format         A⍕B   text      Format B into a character matrix according to A
	                                      A is the textual format (see format special command);
	                                      otherwise result depends on length of A:
	                                      1 gives decimal count, 2 gives width and decimal count,
	                                      3 gives width, decimal count, and style ('d', 'e', 'f', etc.).
	                                      'T' text B formats seconds value B as a Unix date
	General transpose     A⍉B   transp    The axes of B are ordered by A
	Combinations          A!B   !         Number of combinations of B taken A at a time
	Less than             A<B   <         Comparison: 1 if true, 0 if false
	Less than or equal    A≤B   <=        Comparison: 1 if true, 0 if false
	Equal                 A=B   ==        Comparison: 1 if true, 0 if false
	Greater than or equal A≥B   >=        Comparison: 1 if true, 0 if false
	Greater than          A>B   >         Comparison: 1 if true, 0 if false
	Not equal             A≠B   !=        Comparison: 1 if true, 0 if false
	Or                    A∨B   or        Logic: 0 if A and B are 0; 1 otherwise
	And                   A∧B   and       Logic: 1 if A and B are 1; 0 otherwise
	Nor                   A⍱B   nor       Logic: 1 if both A and B are 0; otherwise 0
	Nand                  A⍲B   nand      Logic: 0 if both A and B are 1; otherwise 1
	Xor                         xor       Logic: 1 if A != B; otherwise 0
	Bitwise and                 &         Bitwise A and B (integer only)
	Bitwise or                  |         Bitwise A or B (integer only)
	Bitwise xor                 ^         Bitwise A exclusive or B (integer only)
	Left shift                  <<        A shifted left B bits (integer only)
	Right Shift                 >>        A shifted right B bits (integer only)
	Complex construction        j         The complex number A+Bi

Operators and axis indicator

	Name                APL  Ivy  APL Example  Ivy Example  Meaning (of example)
	Reduce (last axis)  /    /    +/B          +/B          Sum across B
	Reduce (first axis) ⌿    /%   +⌿B                       Sum down B
	Scan (last axis)    \    \    +\B          +\B          Running sum across B
	Scan (first axis)   ⍀    \%   +⍀B                       Running sum down B
	Inner product       .    .    A+.×B        A +.* B      Matrix product of A and B
	Outer product       ∘.   o.   A∘.×B        A o.* B      Outer product of A and B
	                                                        (lower case o;
	                                                        may need preceding space)
	Each left                @f                A @f B       (A[1] f B), (A[2] f B), ...
	                                                        as vector or matrix
	Each right          f¨   f@   A f¨ B       A f@ B       (A f B[1]), (A f B[2]), ...
	                                                        as vector or matrix

Type-converting operations

	Name              APL   Ivy     Meaning
	Code                    code B  The integer Unicode value of char B
	Char                    char B  The character with integer Unicode value B
	Float                   float B The floating-point representation of B;
	                                for complex numbers, the result is
	                                (float A)j(float B)

# Pre-defined constants

The constants e (base of natural logarithms) and pi (π) are pre-defined to high
precision, about 3000 decimal digits truncated according to the floating point
precision setting.

# Character data

Strings are vectors of "chars", which are Unicode code points (not bytes).
Syntactically, string literals are very similar to those in Go, with back-quoted
raw strings and double-quoted interpreted strings. Unlike Go, single-quoted strings
are equivalent to double-quoted, a nod to APL syntax. A string with a single char
is just a singleton char value; all others are vectors. Thus “, "", and ” are
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

# User-defined operators

Users can define unary and binary operators, which then behave just like
built-in operators. Both a unary and a binary operator may be defined for the
same name.

The syntax of a definition is the 'op' keyword, the operator and formal
arguments, an equals sign, and then the body. The names of the operator and its
arguments must be identifiers.  For unary operators, write "op name arg"; for
binary write "op leftarg name rightarg". The final expression in the body is the
return value. Operators may have recursive definitions; see the paragraph
about conditional execution for an example.

Each formal argument can be a single name or a parenthesized list of formal
arguments, requiring a vector argument of that same length. Each actual argument
is assigned to its corresponding formal argument at the start of function execution,
creating new local variables.

The body may be a single line (possibly containing semicolons) on the same line
as the 'op', or it can be multiple lines. For a multiline entry, there is a
newline after the '=' and the definition ends at the first blank line (ignoring
spaces).

Conditional execution is done with the ":" binary conditional return operator,
which is valid only within the code for a user-defined operator. The left
operand must be a scalar. If it is non-zero, the right operand is returned as
the value of the function. Otherwise, execution continues normally. The ":"
operator has a lower precedence than any other operator; in effect it breaks
the line into two separate expressions.

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

Example: primes less than N (unary):

	op primes N = (not T in T o.* T) sel T = 1 drop iota N
	primes 50
	result: 2 3 5 7 11 13 17 19 23 29 31 37 41 43 47

Example: greatest common divisor (binary):

	op a gcd b =
		a == b: a
		a > b: b gcd a-b
		a gcd b-a

	1562 gcd !11
	result: 22

Example: modular exponentiation (unary, with a 3-element vector argument):

	op modexp (b e m) =  # (b**e) mod m
		e == 0: 1
		e % 2: (b * modexp b (e-1) m) mod m
		modexp ((b**2) mod m) (e>>1) m

On mobile platforms only, due to I/O restrictions, user-defined operators
must be presented on a single line. Use semicolons to separate expressions:

	op a gcd b = a == b: a; a > b: b gcd a-b; a gcd b-a

To declare an operator but not define it, omit the equals sign and what follows.

	op foo x
	op bar x = foo x
	op foo x = -x
	bar 3
	result: -3
	op foo x = /x
	bar 3
	result: 1/3

Within a user-defined operator body, identifiers are local to the invocation
if they are assigned before being read, and global if read before being written.
To write to a global without reading it first, insert an unused read.

	total = 0
	last = 0
	op save x =
		total = total + x  # total is global because total is read before written
		last; last = x     # unused read makes last global

	save 9; save 3
	total last
	result: 12 3

To remove the definition of a unary or binary user-defined operator,

	opdelete foo x
	opdelete a gcd b

# Special commands

Ivy accepts a number of special commands, introduced by a right paren
at the beginning of the line. Most report the current value if a new value
is not specified. For these commands, numbers are always read and printed
base 10 and must be non-negative on input.

	) help
		Describe the special commands. Run )help <topic> to learn more
		about a topic, )help <op> to learn more about an operator.
	) base 0
		Set the number base for input and output. The commands ibase and
		obase control setting of the base for input and output alone,
		respectively.  Base 0 allows C-style input: decimal, with 037 being
		octal and 0x10 being hexadecimal. Bases above 16 are disallowed.
		To output large integers and rationals, base must be one of
		0 2 8 10 16. Floats are always printed base 10.
	) cpu
		Print the duration of the last interactive calculation.
	) debug name 0|1
		Toggle or set the named debugging flag. With no argument, lists
		the settings.
	) demo
		Run a line-by-line interactive demo. On mobile platforms,
		use the Demo menu option instead.
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
	) maxstack 1e5
		To avoid using too much stack, the number of nested active calls to
		user-defined operators is limited to maxstack.
	) op X
		If X is absent, list all user-defined operators. Otherwise,
		show the definition of the user-defined operator X. Inside the
		definition, numbers are always shown base 10, ignoring the ibase
		and obase.
	) origin 1
		Set the origin for indexing a vector or matrix. Must be non-negative.
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
	) timezone "Local"
		Set the time zone to be used for display. If the argument is
		missing, print the name and zone offset in seconds east.
	) var X
		If X is absent, list all defined variables. Otherwise, show the
		definition of the variable X in a form that can be evaluated
		to recreate the value.
*/
package main
