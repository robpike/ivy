// echo GENERATED; DO NOT EDIT
package parse

const specialHelpMessage = `
) help
	Print this list of special commands.
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
) maxdigits 10000
	To avoid overwhelming amounts of output, if an integer has more
	than this many digits, print it using the defined floating-point
	format. If maxdigits is 0, integers are always printed as integers.
) maxexp 1000000000
	The maximum allowed exponent in the ** operator.
	The exponent must fit in 63 bits; the default is 1e9.
) op X
	Show the definition of the user-defined operator X.
	Inside the definition, numbers are always shown base
	10, ignoring the ibase and obase.
) origin 1
	Set the origin for indexing a vector or matrix.
) prec 256
	Set the precision (mantissa length) for floating-point values.
	The value is in bits. The exponent always has 32 bits.
) prompt ""
	Set the interactive prompt.
) seed 0
	Set the seed for the ? operator.
`
