package clones

import "fmt"

// SumPositive adds the positive values and reports how many there were.
func SumPositive(values []int) (int, int) {
	total := 0
	count := 0
	for _, value := range values {
		if value > 0 {
			total += value
			count++
		}
	}
	fmt.Printf("%d positive values\n", count)
	return total, count
}

// SumNegative is SumPositive with the names and the comparison changed.
func SumNegative(numbers []int) (int, int) {
	sum := 0
	seen := 0
	for _, number := range numbers {
		if number < 0 {
			sum += number
			seen++
		}
	}
	fmt.Printf("%d negative values\n", seen)
	return sum, seen
}
