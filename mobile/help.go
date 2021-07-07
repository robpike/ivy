package mobile

// GENERATED; DO NOT EDIT
const help = `<!-- auto-generated from robpike.io/ivy package doc -->

<head>
    <style>
        body {
                font-family: Arial, sans-serif;
	        font-size: 10pt;
                line-height: 1.3em;
                max-width: 950px;
                word-break: normal;
                word-wrap: normal;
        }

        pre {
                border-radius: 10px;
                border: 2px solid #8AC007;
		font-family: monospace;
		font-size: 10pt;
                overflow: auto;
                padding: 10px;
                white-space: pre;
        }
    </style>
</head>
<body>
<p>
Ivy is an interpreter for an APL-like language. It is a plaything and a work in
progress.
</p>
<p>
Unlike APL, the input is ASCII and the results are exact (but see the next paragraph).
It uses exact rational arithmetic so it can handle arbitrary precision. Values to be
input may be integers (3, -1), rationals (1/3, -45/67) or floating point values (1e3,
-1.5 (representing 1000 and -3/2)).
</p>
<p>
Some functions such as sqrt are irrational. When ivy evaluates an irrational
function, the result is stored in a high-precision floating-point number (default
256 bits of mantissa). Thus when using irrational functions, the values have high
precision but are not exact.
</p>
<p>
Unlike in most other languages, operators always have the same precedence and
expressions are evaluated in right-associative order. That is, unary operators
apply to everything to the right, and binary operators apply to the operand
immediately to the left and to everything to the right.  Thus, 3*4+5 is 27 (it
groups as 3*(4+5)) and iota 3+2 is 1 2 3 4 5 while 3+iota 2 is 4 5. A vector
is a single operand, so 1 2 3 + 3 + 3 4 5 is (1 2 3) + 3 + (3 4 5), or 7 9 11.
</p>
<p>
As a special but important case, note that 1/3, with no intervening spaces, is a
single rational number, not the expression 1 divided by 3. This can affect precedence:
3/6*4 is 2 while 3 / 6*4 is 1/8 since the spacing turns the / into a division
operator. Use parentheses or spaces to disambiguate: 3/(6*4) or 3 /6*4.
</p>
<p>
Indexing uses [] notation: x[1], x[1][2], and so on. Indexing by a vector
selects multiple elements: x[1 2] creates a new item from x[1] and x[2].
</p>
<p>
Only a subset of APL&#39;s functionality is implemented, but the intention is to
have most numerical operations supported eventually.
</p>
<p>
Semicolons separate multiple statements on a line. Variables are alphanumeric and are
assigned with the = operator. Assignment is an expression.
</p>
<p>
After each successful expression evaluation, the result is stored in the variable
called _ (underscore) so it can be used in the next expression.
</p>
<p>
The APL operators, adapted from <a href="https://en.wikipedia.org/wiki/APL_syntax_and_symbols">https://en.wikipedia.org/wiki/APL_syntax_and_symbols</a>,
and their correspondence are listed here. The correspondence is incomplete and inexact.
</p>
<p>
Unary operators
</p>
<pre>Name              APL   Ivy     Meaning
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
Signum            ×B    sgn     ¯1 if B&lt;0; 0 if B=0; 1 if B&gt;0
Reciprocal        ÷B    /       1 divided by B
Ravel             ,B    ,       Reshapes B into a vector
Matrix inverse    ⌹B            Inverse of matrix B
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
</pre>
<p>
Binary operators
</p>
<pre>Name                  APL   Ivy     Meaning
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
Deal                  A?B   ?       A distinct integers selected randomly from the first B integers
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
                                    In ivy: abs(A) gives count, A &lt;= 0 inserts zero (or blank)
Compression           A/B   sel     Select elements in B corresponding to ones in A
                                    In ivy: abs(A) gives count, A &lt;= 0 inserts zero
Index of              A⍳B   iota    The location (index) of B in A; 1+⌈/⍳⍴A if not found
                                    In ivy: origin-1 if not found (i.e. 0 if one-indexed)
Matrix divide         A⌹B           Solution to system of linear equations Ax = B
Rotation              A⌽B   rot     The elements of B are rotated A positions left
Rotation              A⊖B   flip    The elements of B are rotated A positions along the first axis
Logarithm             A⍟B   log     Logarithm of B to base A
Dyadic format         A⍕B   text    Format B into a character matrix according to A
                                    A is the textual format (see format special command);
                                    otherwise result depends on length of A:
                                    1 gives decimal count, 2 gives width and decimal count,
                                    3 gives width, decimal count, and style (&#39;d&#39;, &#39;e&#39;, &#39;f&#39;, etc.).
General transpose     A⍉B           The axes of B are ordered by A
Combinations          A!B   !       Number of combinations of B taken A at a time
Less than             A&lt;B   &lt;       Comparison: 1 if true, 0 if false
Less than or equal    A≤B   &lt;=      Comparison: 1 if true, 0 if false
Equal                 A=B   ==      Comparison: 1 if true, 0 if false
Greater than or equal A≥B   &gt;=      Comparison: 1 if true, 0 if false
Greater than          A&gt;B   &gt;       Comparison: 1 if true, 0 if false
Not equal             A≠B   !=      Comparison: 1 if true, 0 if false
Or                    A∨B   or      Logic: 0 if A and B are 0; 1 otherwise
And                   A∧B   and     Logic: 1 if A and B are 1; 0 otherwise
Nor                   A⍱B   nor     Logic: 1 if both A and B are 0; otherwise 0
Nand                  A⍲B   nand    Logic: 0 if both A and B are 1; otherwise 1
Xor                         xor     Logic: 1 if A != B; otherwise 0
Bitwise and                 &amp;       Bitwise A and B (integer only)
Bitwise or                  |       Bitwise A or B (integer only)
Bitwise xor                 ^       Bitwise A exclusive or B (integer only)
Left shift                  &lt;&lt;      A shifted left B bits (integer only)
Right Shift                 &gt;&gt;      A shifted right B bits (integer only)
</pre>
<p>
Operators and axis indicator
</p>
<pre>Name                APL  Ivy  APL Example  Ivy Example  Meaning (of example)
Reduce (last axis)  /    /    +/B          +/B          Sum across B
Reduce (first axis) ⌿         +⌿B                       Sum down B
Scan (last axis)    \    \    +\B          +\B          Running sum across B
Scan (first axis)   ⍀         +⍀B                       Running sum down B
Inner product       .    .    A+.×B        A +.* B      Matrix product of A and B
Outer product       ∘.   o.   A∘.×B        A o.* B      Outer product of A and B
                                                    (lower case o; may need preceding space)
</pre>
<p>
Type-converting operations
</p>
<pre>Name              APL   Ivy     Meaning
Code                    code B  The integer Unicode value of char B
Char                    char B  The character with integer Unicode value B
Float                   float B The floating-point representation of B
</pre>
<h3 id="hdr-Pre_defined_constants">Pre-defined constants</h3>
<p>
The constants e (base of natural logarithms) and pi (π) are pre-defined to high
precision, about 3000 decimal digits truncated according to the floating point
precision setting.
</p>
<h3 id="hdr-Character_data">Character data</h3>
<p>
Strings are vectors of &#34;chars&#34;, which are Unicode code points (not bytes).
Syntactically, string literals are very similar to those in Go, with back-quoted
raw strings and double-quoted interpreted strings. Unlike Go, single-quoted strings
are equivalent to double-quoted, a nod to APL syntax. A string with a single char
is just a singleton char value; all others are vectors. Thus &ldquo;, &#34;&#34;, and &rdquo; are
empty vectors, ` + "`" + `a` + "`" + `, &#34;a&#34;, and &#39;a&#39; are equivalent representations of a single char,
and ` + "`" + `ab` + "`" + `, ` + "`" + `a` + "`" + ` ` + "`" + `b` + "`" + `, &#34;ab&#34;, &#34;a&#34; &#34;b&#34;, &#39;ab&#39;, and &#39;a&#39; &#39;b&#39; are equivalent representations
of a two-char vector.
</p>
<p>
Unlike in Go, a string in ivy comprises code points, not bytes; as such it can
contain only valid Unicode values. Thus in ivy &#34;\x80&#34; is illegal, although it is
a legal one-byte string in Go.
</p>
<p>
Strings can be printed. If a vector contains only chars, it is printed without
spaces between them.
</p>
<p>
Chars have restricted operations. Printing, comparison, indexing and so on are
legal but arithmetic is not, and chars cannot be converted automatically into other
singleton values (ints, floats, and so on). The unary operators char and code
enable transcoding between integer and char values.
</p>
<h3 id="hdr-User_defined_operators">User-defined operators</h3>
<p>
Users can define unary and binary operators, which then behave just like
built-in operators. Both a unary and a binary operator may be defined for the
same name.
</p>
<p>
The syntax of a definition is the &#39;op&#39; keyword, the operator and formal
arguments, an equals sign, and then the body. The names of the operator and its
arguments must be identifiers.  For unary operators, write &#34;op name arg&#34;; for
binary write &#34;op leftarg name rightarg&#34;. The final expression in the body is the
return value. Operators may have recursive definitions, but since there are
no conditional or looping constructs (yet), such operators are problematic
when executed.
</p>
<p>
The body may be a single line (possibly containing semicolons) on the same line
as the &#39;op&#39;, or it can be multiple lines. For a multiline entry, there is a
newline after the &#39;=&#39; and the definition ends at the first blank line (ignoring
spaces).
</p>
<p>
Conditional execution is done with the &#34;:&#34; binary conditional return operator,
which is valid only within the code for a user-defined operator. The left
operand must be a scalar. If it is non-zero, the right operand is returned as
the value of the function. Otherwise, execution continues normally. The &#34;:&#34;
operator has a lower precedence than any other operator; in effect it breaks
the line into two separate expressions.
</p>
<p>
Example: average of a vector (unary):
</p>
<pre>op avg x = (+/x)/rho x
avg iota 11
result: 6
</pre>
<p>
Example: n largest entries in a vector (binary):
</p>
<pre>op n largest x = n take x[down x]
3 largest 7 1 3 24 1 5 12 5 51
result: 51 24 12
</pre>
<p>
Example: multiline operator definition (binary):
</p>
<pre>op a sum b =
	a = a+b
	a

iota 3 sum 4
result: 1 2 3 4 5 6 7
</pre>
<p>
Example: primes less than N (unary):
</p>
<pre>op primes N = (not T in T o.* T) sel T = 1 drop iota N
primes 50
result: 2 3 5 7 11 13 17 19 23 29 31 37 41 43 47
</pre>
<p>
Example: greatest common divisor (binary):
</p>
<pre>op a gcd b =
	a == b: a
	a &gt; b: b gcd a-b
	a gcd b-a

1562 gcd !11
result: 22
</pre>
<p>
To declare an operator but not define it, omit the equals sign and what follows.
</p>
<pre>op foo x
op bar x = foo x
op foo x = -x
bar 3
result: -3
op foo x = /x
bar 3
result: 1/3
</pre>
<p>
Within a user-defined operator, identifiers are local to the invocation unless
they are undefined in the operator but defined globally, in which case they refer to
the global variable. A mechanism to declare locals may come later.
</p>
<h3 id="hdr-Special_commands">Special commands</h3>
<p>
Ivy accepts a number of special commands, introduced by a right paren
at the beginning of the line. Most report the current value if a new value
is not specified. For these commands, numbers are always read and printed
base 10 and must be non-negative on input.
</p>
<pre>) help
	Describe the special commands. Run )help &lt;topic&gt; to learn more
	about a topic, )help &lt;op&gt; to learn more about an operator.
) base 0
	Set the number base for input and output. The commands ibase and
	obase control setting of the base for input and output alone,
	respectively.  Base 0 allows C-style input: decimal, with 037 being
	octal and 0x10 being hexadecimal. If the base is greater than 10,
	any identifier formed from valid numerals in the base system, such
	as abe for base 16, is taken to be a number. TODO: To output
	large integers and rationals, base must be one of 0 2 8 10 16.
	Floats are always printed base 10.
) cpu
	Print the duration of the last interactive calculation.
) debug name 0|1
	Toggle or set the named debugging flag. With no argument, lists
	the settings.
) demo
	Run a line-by-line interactive demo. Requires a Go installation.
) format &#34;&#34;
	Set the format for printing values. If empty, the output is printed
	using the output base. If non-empty, the format determines the
	base used in printing. The format is in the style of golang.org/pkg/fmt.
	For floating-point formats, flags and width are ignored.
) get &#34;save.ivy&#34;
	Read input from the named file; return to interactive execution
	afterwards. If no file is specified, read from &#34;save.ivy&#34;.
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
	Set the origin for indexing a vector or matrix.
) prec 256
	Set the precision (mantissa length) for floating-point values.
	The value is in bits. The exponent always has 32 bits.
) prompt &#34;&#34;
	Set the interactive prompt.
) save &#34;save.ivy&#34;
	Write definitions of user-defined operators and variables to the
	named file, as ivy textual source. If no file is specified, save to
	&#34;save.ivy&#34;.
	(Unimplemented on mobile.)
) seed 0
	Set the seed for the ? operator.
</pre>
</body></html>
`
