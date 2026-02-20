package main

func f() {
	// decimal integers
	_ = 0
	_ = 42
	_ = 1_000_000

	// hex
	_ = 0xFF
	_ = 0XAB
	_ = 0xff_ff

	// octal
	_ = 0o77
	_ = 0O17
	_ = 0777

	// binary
	_ = 0b1010
	_ = 0B1111_0000

	// float
	_ = 3.14
	_ = .5
	_ = 1.
	_ = 1e10
	_ = 1.5e-3
	_ = 0x1p10
	_ = 0x1.Fp+0

	// imaginary
	_ = 1i
	_ = 3.14i
	_ = 1e10i
	_ = .5i

	// rune
	_ = 'a'
	_ = '\n'
	_ = '\t'
	_ = '\\'
	_ = '\''
	_ = '\x41'
	_ = '\u0041'
	_ = '\U00000041'
	_ = '\007'

	// string
	_ = "hello world"
	_ = "tab\there"
	_ = "newline\nhere"
	_ = "quote\"inside"
	_ = `raw string literal`
	_ = `multi
line
raw`
	_ = ""
	_ = ``
}
