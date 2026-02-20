package main

import "fmt"

func safeDiv(a, b int) (result int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered: %v", r)
		}
	}()
	return a / b, nil
}

func withDefer() {
	defer fmt.Println("done")
	fmt.Println("working")
}

func multiDefer() {
	for i := 0; i < 3; i++ {
		defer fmt.Println(i)
	}
}
