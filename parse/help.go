// echo GENERATED; DO NOT EDIT

package parse

const specialHelpMessage = `
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
) cpu
	Print the duration of the last interactive calculation.
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
`
