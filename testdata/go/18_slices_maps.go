package main

func f() {
	// slice operations
	s := make([]int, 5, 10)
	s = append(s, 1, 2, 3)
	_ = len(s)
	_ = cap(s)

	// slice expressions
	sub := s[1:3]
	_ = sub
	sub2 := s[:3]
	_ = sub2
	sub3 := s[2:]
	_ = sub3
	full := s[:]
	_ = full

	// three-index slice
	limited := s[1:3:5]
	_ = limited

	// copy
	dst := make([]int, len(s))
	copy(dst, s)

	// map operations
	m := make(map[string]int)
	m["one"] = 1
	m["two"] = 2
	v, ok := m["one"]
	_, _ = v, ok
	delete(m, "two")
	_ = len(m)
}
