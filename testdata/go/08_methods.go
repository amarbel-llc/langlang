package main

type Counter struct {
	n int
}

func (c Counter) Value() int {
	return c.n
}

func (c *Counter) Increment() {
	c.n++
}

func (c *Counter) Add(delta int) {
	c.n += delta
}

func (c *Counter) Reset() {
	c.n = 0
}
