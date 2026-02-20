package main

func f() {
	x := 10

	if x > 0 {
		x = 1
	}

	if x > 0 {
		x = 1
	} else {
		x = -1
	}

	if x > 0 {
		x = 1
	} else if x == 0 {
		x = 0
	} else {
		x = -1
	}

	if v := compute(); v > 0 {
		x = v
	}
}

func compute() int { return 42 }
