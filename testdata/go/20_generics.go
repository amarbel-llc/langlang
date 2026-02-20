package main

type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~float32 | ~float64
}

func Sum[T Number](xs []T) T {
	var total T
	for _, v := range xs {
		total += v
	}
	return total
}

func Map[T any, U any](xs []T, f func(T) U) []U {
	result := make([]U, len(xs))
	for i, v := range xs {
		result[i] = f(v)
	}
	return result
}

type Pair[A, B any] struct {
	First  A
	Second B
}

func NewPair[A, B any](a A, b B) Pair[A, B] {
	return Pair[A, B]{First: a, Second: b}
}

type Stack[T any] struct {
	items []T
}

func (s *Stack[T]) Push(v T) {
	s.items = append(s.items, v)
}

func (s *Stack[T]) Pop() (T, bool) {
	if len(s.items) == 0 {
		var zero T
		return zero, false
	}
	v := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return v, true
}
