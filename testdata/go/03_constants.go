package main

const x = 1

const (
	a = 1
	b = 2
	c = "hello"
)

const (
	_  = iota
	KB = 1 << (10 * iota)
	MB
	GB
	TB
)

const MaxSize int = 1024
const Pi float64 = 3.14159265358979323846
