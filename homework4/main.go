package main

import (
	"fmt"

	"symbolic-execution-course/internal/memory"
	"symbolic-execution-course/internal/symbolic"
)

func main() {
	var mem = memory.NewSymbolicMemory()
	var array = mem.Allocate(symbolic.ArrayType)

	mem.AssignToArray(array, 5, symbolic.NewIntConstant(10))

	var fromArray = mem.GetFromArray(array, 5, symbolic.IntType)
	fmt.Println(fromArray)

	var anotherFromArray = mem.GetFromArray(array, 10, symbolic.IntType)
	fmt.Println(anotherFromArray)
}
