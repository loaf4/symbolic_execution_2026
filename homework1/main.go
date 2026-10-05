// Демонстрационная программа для тестирования SSA построения
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"symbolic-execution-course/internal/ssa"
)

func main() {
	file := flag.String("file", "", "path to Go source file (default: homework1/examples/test_functions.go)")
	funcName := flag.String("func", "", "function to print (default: all functions)")
	flag.Parse()

	fmt.Println("=== SSA Builder Demo ===")

	// Пример исходного кода для анализа
	source := `
package main

func testFunction(x int) int {
	if x > 0 {
		return x * 2
	} else {
		return x * -1
	}
}
`

	// Создаём builder для SSA
	builder := ssa.NewBuilder()

	// Строим SSA из исходного кода
	graph, err := builder.ParseAndBuildSSA(source, "testFunction")
	if err != nil {
		log.Fatalf("SSA build failed: %v", err)
	}
	fmt.Printf("CFG built for function with %d blocks\n\n", len(graph.Blocks))
	ssa.PrintFunction(os.Stdout, graph)

	// from examples
	path := *file
	if path == "" {
		path = findExamples()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("failed to read %s: %v", path, err)
	}

	pkg, err := ssa.NewBuilder().BuildPackage(string(data))
	if err != nil {
		log.Fatalf("SSA build failed for %s: %v", path, err)
	}

	names := ssa.FunctionNames(pkg)
	if *funcName != "" {
		names = []string{*funcName}
	}
	for _, name := range names {
		fn := pkg.Func(name)
		if fn == nil {
			log.Fatalf("function %q not found in %s", name, path)
		}
		fmt.Printf("\n==================== %s ====================\n", name)
		ssa.PrintFunction(os.Stdout, fn)
	}
}

func findExamples() string {
	for _, p := range []string{"homework1/examples/test_functions.go", "examples/test_functions.go"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "homework1/examples/test_functions.go"
}
