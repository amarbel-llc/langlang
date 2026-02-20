package main

func f() {
	// arithmetic
	a := 1 + 2
	b := a * 3
	c := b / 2
	d := c % 5
	e := a - d
	_, _, _, _, _ = a, b, c, d, e

	// bitwise
	x := 0xFF
	y := x & 0x0F
	z := x | 0xF0
	w := x ^ 0xAA
	s := x << 2
	r := x >> 1
	t := x &^ 0x0F
	_, _, _, _, _, _ = y, z, w, s, r, t

	// comparison
	_ = 1 == 1
	_ = 1 != 2
	_ = 1 < 2
	_ = 2 > 1
	_ = 1 <= 1
	_ = 1 >= 1

	// logical
	_ = true && false
	_ = true || false
	_ = !true

	// unary
	n := 42
	_ = -n
	_ = +n
	_ = ^n

	// assignment operators
	n += 1
	n -= 1
	n *= 2
	n /= 2
	n %= 3
	n &= 0xFF
	n |= 0x01
	n ^= 0xAA
	n <<= 1
	n >>= 1
	n &^= 0x0F

	// inc/dec
	n++
	n--
}
