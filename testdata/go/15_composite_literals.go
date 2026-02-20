package main

type Point struct {
	X, Y int
}

func f() {
	// struct literal
	p := Point{X: 1, Y: 2}
	_ = p

	// struct literal positional
	q := Point{3, 4}
	_ = q

	// slice literal
	xs := []int{1, 2, 3, 4, 5}
	_ = xs

	// array literal
	arr := [3]string{"a", "b", "c"}
	_ = arr

	// ellipsis array
	auto := [...]int{10, 20, 30}
	_ = auto

	// map literal
	m := map[string]int{
		"one":   1,
		"two":   2,
		"three": 3,
	}
	_ = m

	// nested literal
	points := []Point{
		{1, 2},
		{3, 4},
		{5, 6},
	}
	_ = points

	// map of slices
	ms := map[string][]int{
		"primes": {2, 3, 5, 7},
		"evens":  {2, 4, 6, 8},
	}
	_ = ms

	// empty literals
	empty1 := Point{}
	empty2 := []int{}
	empty3 := map[string]int{}
	_, _, _ = empty1, empty2, empty3
}
