// Демонстрационная программа для тестирования символьных выражений
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/ebukreev/go-z3/z3"
	"symbolic-execution-course/internal/symbolic"
	"symbolic-execution-course/internal/translator"
)

func main() {
	fmt.Println("=== Symbolic Expressions Demo ===")

	// Создаём простые символьные выражения
	x := symbolic.NewSymbolicVariable("x", symbolic.IntType)
	y := symbolic.NewSymbolicVariable("y", symbolic.IntType)
	five := symbolic.NewIntConstant(5)

	// Создаём выражение: x + y > 5
	sum := symbolic.NewBinaryOperation(x, y, symbolic.ADD)
	condition := symbolic.NewBinaryOperation(sum, five, symbolic.GT)

	fmt.Printf("Expression: %s\n", condition.String())
	fmt.Printf("Expression type: %s\n", condition.Type().String())

	// Создаём Z3 транслятор
	translator := translator.NewZ3Translator()
	defer translator.Close()

	// Транслируем в Z3
	z3Expr, err := translator.TranslateExpression(condition)
	if err != nil {
		log.Fatalf("Translation failed: %v", err)
	}

	fmt.Printf("Z3 expression created: %T: %v\n", z3Expr, z3Expr)

	// Создаём более сложное выражение: (x > 0) && (y < 10)
	zero := symbolic.NewIntConstant(0)
	ten := symbolic.NewIntConstant(10)

	cond1 := symbolic.NewBinaryOperation(x, zero, symbolic.GT)
	cond2 := symbolic.NewBinaryOperation(y, ten, symbolic.LT)

	andExpr := symbolic.NewLogicalOperation([]symbolic.SymbolicExpression{cond1, cond2}, symbolic.AND)

	fmt.Printf("Complex expression: %s\n", andExpr.String())

	// Транслируем сложное выражение
	z3AndExpr, err := translator.TranslateExpression(andExpr)
	if err != nil {
		log.Fatalf("Complex expression translation failed: %v", err)
	}

	fmt.Printf("Complex Z3 expression created: %T: %v\n", z3AndExpr, z3AndExpr)

	// Решаем выражения, соответствующие функциям из examples/test_functions.go
	fmt.Println("\n=== Solving with Z3 ===")

	z := symbolic.NewSymbolicVariable("z", symbolic.IntType)
	score := symbolic.NewSymbolicVariable("score", symbolic.IntType)
	and := func(operands ...symbolic.SymbolicExpression) symbolic.SymbolicExpression {
		return symbolic.NewLogicalOperation(operands, symbolic.AND)
	}
	or := func(operands ...symbolic.SymbolicExpression) symbolic.SymbolicExpression {
		return symbolic.NewLogicalOperation(operands, symbolic.OR)
	}
	not := func(operand symbolic.SymbolicExpression) symbolic.SymbolicExpression {
		return symbolic.NewLogicalOperation([]symbolic.SymbolicExpression{operand}, symbolic.NOT)
	}
	bin := symbolic.NewBinaryOperation
	num := symbolic.NewIntConstant

	// abs(x) = x < 0 ? -x : x
	abs := symbolic.NewConditionalExpression(
		bin(x, zero, symbolic.LT), symbolic.NewUnaryOperation(x, symbolic.NEG), x)

	examples := []struct {
		name string
		expr symbolic.SymbolicExpression
	}{
		// isValid: x > 0 && y > 0 && x < 100
		{"isValid", and(
			bin(x, zero, symbolic.GT), bin(y, zero, symbolic.GT), bin(x, num(100), symbolic.LT))},
		// complexCondition: (x > 0 || y > 0) && (z > x+y)
		{"complexCondition", and(
			or(bin(x, zero, symbolic.GT), bin(y, zero, symbolic.GT)),
			bin(z, bin(x, y, symbolic.ADD), symbolic.GT))},
		// modOperations: x%2 == 0 && y%3 == 1
		{"modOperations", and(
			bin(bin(x, num(2), symbolic.MOD), zero, symbolic.EQ),
			bin(bin(y, num(3), symbolic.MOD), num(1), symbolic.EQ))},
		// polynomial: 3*x*x + 2*x + 1 == 34
		{"polynomial == 34", bin(
			bin(bin(bin(bin(num(3), x, symbolic.MUL), x, symbolic.MUL), bin(num(2), x, symbolic.MUL), symbolic.ADD),
				num(1), symbolic.ADD),
			num(34), symbolic.EQ)},
		// classify: условие пути до return "B"
		{"classify returns B", and(
			not(bin(score, num(90), symbolic.GE)), bin(score, num(80), symbolic.GE))},
		// abs: abs(x) == 5 при отрицательном x
		{"abs(x) == 5, x < 0", and(bin(abs, five, symbolic.EQ), bin(x, zero, symbolic.LT))},
		// abs: результат не может быть отрицательным
		{"abs(x) < 0", bin(abs, zero, symbolic.LT)},
	}
	for _, example := range examples {
		fmt.Printf("\n%s: %s\n", example.name, example.expr)
		solve(translator, example.expr)
	}
}

func solve(zt *translator.Z3Translator, expr symbolic.SymbolicExpression) {
	z3Expr, err := zt.TranslateExpression(expr)
	if err != nil {
		log.Fatalf("Translation failed: %v", err)
	}
	formula, ok := z3Expr.(z3.Bool)
	if !ok {
		log.Fatalf("Expected boolean formula, got %T", z3Expr)
	}
	fmt.Printf("  Z3: %v\n", formula)

	solver := z3.NewSolver(zt.GetContext().(*z3.Context))
	solver.Assert(formula)
	sat, err := solver.Check()
	if err != nil {
		log.Fatalf("Solver failed: %v", err)
	}
	if !sat {
		fmt.Println("  Result: UNSAT")
		return
	}

	// get var values from model
	model := solver.Model()
	var values []string
	for _, v := range symbolic.CollectVariables(expr) {
		z3Var, err := zt.TranslateExpression(v)
		if err != nil {
			log.Fatalf("Translation failed: %v", err)
		}
		values = append(values, fmt.Sprintf("%s = %v", v.Name, model.Eval(z3Var.(z3.Value), true)))
	}
	fmt.Printf("  Result: SAT, %s\n", strings.Join(values, ", "))
}
