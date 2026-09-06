package main

import (
	"fmt"

	"example.com/godeps/lib"
	"example.com/godeps/model"
)

func main() {
	fmt.Println(lib.Count(model.Record{}))
}
