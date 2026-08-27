package clones

import "testing"

// TestSumPositive is another copy of SumPositive. Test files are excluded
// from clone detection, so it must not be reported.
func TestSumPositive(t *testing.T) {
	values := []int{1, -2, 3}
	total := 0
	count := 0
	for _, value := range values {
		if value > 0 {
			total += value
			count++
		}
	}
	if total != 4 || count != 2 {
		t.Fatalf("got %d, %d", total, count)
	}
}
