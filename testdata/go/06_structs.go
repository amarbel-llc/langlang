package main

type Empty struct{}

type Point struct {
	X int
	Y int
}

type Person struct {
	Name    string
	Age     int
	Address *Address
}

type Address struct {
	Street string
	City   string
	State  string
}

type Embedded struct {
	Point
	*Person
	Label string
}

type Tagged struct {
	Name string `json:"name" xml:"name,attr"`
	Age  int    `json:"age,omitempty"`
}

type WithArrays struct {
	Scores  [5]int
	Names   []string
	Lookup  map[string]int
	Channel chan int
}
