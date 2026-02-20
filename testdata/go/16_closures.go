package main

func makeAdder(base int) func(int) int {
	return func(x int) int {
		return base + x
	}
}

func apply(f func(int, int) int, a, b int) int {
	return f(a, b)
}

func f() {
	add5 := makeAdder(5)
	_ = add5(3)

	result := apply(func(a, b int) int { return a + b }, 10, 20)
	_ = result

	// immediately invoked
	v := func() int { return 42 }()
	_ = v

	// closure capturing variables
	count := 0
	inc := func() {
		count++
	}
	inc()
	inc()
}
