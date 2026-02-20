package main

import "fmt"

func describe(x interface{}) string {
	switch v := x.(type) {
	case int:
		return fmt.Sprintf("int: %d", v)
	case string:
		return fmt.Sprintf("string: %s", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return "unknown"
	}
}

func typeCheck(x interface{}) {
	switch x.(type) {
	case int, int64:
		fmt.Println("integer")
	case string:
		fmt.Println("string")
	}
}
