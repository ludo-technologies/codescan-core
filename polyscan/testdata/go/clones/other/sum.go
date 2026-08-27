package other

import "fmt"

// SumPositive is an exact copy, apart from this comment and the spacing.
func SumPositive(values []int) (int, int) {
	total := 0
	count := 0

	for _, value := range values {
		if value > 0 { // inline comment
			total += value
			count++
		}
	}
	fmt.Printf("%d positive values\n", count)
	return total, count
}
