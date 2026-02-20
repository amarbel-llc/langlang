package main

func f() {
	x := 42
	p := &x
	_ = *p

	*p = 100

	// pointer to struct
	type Node struct {
		Value int
		Next  *Node
	}

	n := &Node{Value: 1}
	n.Next = &Node{Value: 2}
	_ = n.Next.Value
}

func modify(p *int) {
	*p = *p * 2
}

func newInt(v int) *int {
	return &v
}
