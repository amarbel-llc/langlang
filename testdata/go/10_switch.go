package main

func f(x int) string {
	switch x {
	case 0:
		return "zero"
	case 1:
		return "one"
	case 2, 3:
		return "few"
	default:
		return "many"
	}
}

func g(x int) string {
	switch {
	case x < 0:
		return "negative"
	case x == 0:
		return "zero"
	default:
		return "positive"
	}
}

func h(x int) string {
	switch v := x * 2; {
	case v < 0:
		return "negative"
	default:
		return "non-negative"
	}
}

func withFallthrough(x int) int {
	result := 0
	switch x {
	case 1:
		result = 10
		fallthrough
	case 2:
		result += 20
	}
	return result
}
