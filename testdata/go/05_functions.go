package main

func noArgs() {}

func oneArg(x int) {}

func twoArgs(x int, y int) {}

func grouped(x, y int) {}

func singleReturn(x int) int {
	return x + 1
}

func namedReturn(x int) (result int) {
	result = x * 2
	return
}

func multiReturn(x, y int) (int, int) {
	return y, x
}

func variadic(xs ...int) int {
	sum := 0
	for _, v := range xs {
		sum += v
	}
	return sum
}

func higherOrder(f func(int) int, x int) int {
	return f(x)
}
