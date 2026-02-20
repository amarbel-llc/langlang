package main

// type alias
type Byte = byte

// defined type
type MyInt int

type StringSlice []string

type Callback func(string) error

type IntChannel chan int

type BiDirChannel chan int
type SendOnly chan<- int
type RecvOnly <-chan int

type Matrix [4][4]float64

type (
	Handler  func(req Request) Response
	Request  struct{ URL string }
	Response struct{ Code int }
)

type PointerToSlice *[]int
type SliceOfPointers []*int
type MapOfSlices map[string][]int
