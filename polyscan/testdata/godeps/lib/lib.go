package lib

import (
	"fmt"

	"example.com/godeps/model"
)

// Count returns the number of records and prints it.
func Count(records ...model.Record) int {
	fmt.Println(len(records))
	return len(records)
}
