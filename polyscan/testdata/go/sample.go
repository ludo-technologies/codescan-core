package sample

import "fmt"

// Simple has no decision points.
func Simple() int {
	return 1
}

// Branches: if, else if, && and || give 1 + 1 + 1 + 1 = 4 decision points.
func Branches(a, b, c bool) string {
	if a && b {
		return "both"
	} else if b || c {
		return "either"
	}
	return "none"
}

type Server struct{}

// Handle: for, range for, expression switch with two cases and a default,
// type switch with two cases, select with one case and a default.
func (s *Server) Handle(items []int, v interface{}, ch chan int) (n int) {
	for i := 0; i < 10; i++ {
		n += i
	}
	for range items {
		n++
	}
	switch n {
	case 1:
		n = 10
	case 2, 3:
		n = 20
	default:
		n = 0
	}
	switch v.(type) {
	case int:
		n++
	case string:
		n--
	default:
	}
	select {
	case <-ch:
		n++
	default:
	}
	return n
}

// Closure: the function literal's if counts toward Closure itself.
func Closure(xs []int) func() {
	return func() {
		for _, x := range xs {
			if x > 0 {
				fmt.Println(x)
			}
		}
	}
}

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(item T) {
	s.items = append(s.items, item)
}

func (s Stack[T]) Len() int {
	return len(s.items)
}
