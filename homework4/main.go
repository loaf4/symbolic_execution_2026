package main

import (
	"fmt"

	"symbolic-execution-course/internal/memory"
	"symbolic-execution-course/internal/symbolic"
)

func main() {
	var mem = memory.NewSymbolicMemory()
	var array = mem.Allocate(symbolic.ArrayType)

	mem.AssignToArray(array, symbolic.NewIntConstant(5), symbolic.NewIntConstant(10))

	var fromArray = mem.GetFromArray(array, symbolic.NewIntConstant(5), symbolic.IntType)
	fmt.Println(fromArray)

	var anotherFromArray = mem.GetFromArray(array, symbolic.NewIntConstant(10), symbolic.IntType)
	fmt.Println(anotherFromArray)
}
