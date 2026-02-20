package main

func loops() {
	// infinite loop
	for {
		break
	}

	// condition only
	x := 0
	for x < 10 {
		x++
	}

	// classic three-part
	for i := 0; i < 10; i++ {
		_ = i
	}

	// range over slice
	xs := []int{1, 2, 3}
	for i, v := range xs {
		_, _ = i, v
	}

	// range index only
	for i := range xs {
		_ = i
	}

	// range discard
	for range xs {
	}

	// range over map
	m := map[string]int{"a": 1, "b": 2}
	for k, v := range m {
		_, _ = k, v
	}

	// range over string
	for i, r := range "hello" {
		_, _ = i, r
	}

	// range over channel
	ch := make(chan int)
	for v := range ch {
		_ = v
	}
}
