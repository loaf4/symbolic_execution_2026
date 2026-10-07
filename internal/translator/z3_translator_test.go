package translator

import (
	"errors"
	"testing"

	"github.com/ebukreev/go-z3/z3"
	"symbolic-execution-course/internal/symbolic"
)

// Сокращения для построения выражений в тестах

func intVar(name string) *symbolic.SymbolicVariable {
	return symbolic.NewSymbolicVariable(name, symbolic.IntType)
}

func boolVar(name string) *symbolic.SymbolicVariable {
	return symbolic.NewSymbolicVariable(name, symbolic.BoolType)
}

func num(v int64) *symbolic.IntConstant {
	return symbolic.NewIntConstant(v)
}

func bin(l symbolic.SymbolicExpression, op symbolic.BinaryOperator, r symbolic.SymbolicExpression) *symbolic.BinaryOperation {
	return symbolic.NewBinaryOperation(l, r, op)
}

func logic(op symbolic.LogicalOperator, operands ...symbolic.SymbolicExpression) *symbolic.LogicalOperation {
	return symbolic.NewLogicalOperation(operands, op)
}

// translateBool транслирует выражение и проверяет, что получилась булева формула
func translateBool(t *testing.T, zt *Z3Translator, expr symbolic.SymbolicExpression) z3.Bool {
	t.Helper()
	res, err := zt.TranslateExpression(expr)
	if err != nil {
		t.Fatalf("translate %s: %v", expr, err)
	}
	b, ok := res.(z3.Bool)
	if !ok {
		t.Fatalf("translate %s: got %T, want z3.Bool", expr, res)
	}
	return b
}

// isSat проверяет выполнимость конъюнкции выражений
func isSat(t *testing.T, zt *Z3Translator, exprs ...symbolic.SymbolicExpression) bool {
	t.Helper()
	solver := z3.NewSolver(zt.ctx)
	for _, expr := range exprs {
		solver.Assert(translateBool(t, zt, expr))
	}
	sat, err := solver.Check()
	if err != nil {
		t.Fatalf("solver check: %v", err)
	}
	return sat
}

// isValid проверяет, что выражение истинно при любых значениях переменных
func isValid(t *testing.T, zt *Z3Translator, expr symbolic.SymbolicExpression) bool {
	t.Helper()
	return !isSat(t, zt, logic(symbolic.NOT, expr))
}

func TestTranslateResultTypes(t *testing.T) {
	zt := NewZ3Translator()
	defer zt.Close()

	x, p := intVar("x"), boolVar("p")
	tests := []struct {
		expr    symbolic.SymbolicExpression
		wantInt bool
	}{
		{x, true},
		{num(42), true},
		{bin(x, symbolic.ADD, num(1)), true},
		{symbolic.NewUnaryOperation(x, symbolic.NEG), true},
		{symbolic.NewConditionalExpression(p, x, num(0)), true},
		{p, false},
		{symbolic.NewBoolConstant(true), false},
		{bin(x, symbolic.LT, num(1)), false},
		{logic(symbolic.NOT, p), false},
		{symbolic.NewConditionalExpression(p, p, symbolic.NewBoolConstant(false)), false},
	}
	for _, tt := range tests {
		res, err := zt.TranslateExpression(tt.expr)
		if err != nil {
			t.Errorf("%s: %v", tt.expr, err)
			continue
		}
		_, isInt := res.(z3.Int)
		_, isBool := res.(z3.Bool)
		if isInt != tt.wantInt || isBool == tt.wantInt {
			t.Errorf("%s: got %T", tt.expr, res)
		}
	}
}

func TestVariableCache(t *testing.T) {
	zt := NewZ3Translator()
	defer zt.Close()

	// Две разные переменные с одним именем должны быть одной переменной Z3:
	// x > 0 && x < 0 невыполнимо
	if isSat(t, zt, bin(intVar("x"), symbolic.GT, num(0)), bin(intVar("x"), symbolic.LT, num(0))) {
		t.Error("x > 0 && x < 0 must be unsatisfiable")
	}
	if len(zt.vars) != 1 {
		t.Errorf("cache has %d variables, want 1", len(zt.vars))
	}

	zt.Reset()
	if len(zt.vars) != 0 {
		t.Errorf("cache has %d variables after Reset, want 0", len(zt.vars))
	}
}

func TestSatisfiability(t *testing.T) {
	zt := NewZ3Translator()
	defer zt.Close()

	x, y := intVar("x"), intVar("y")
	p, q := boolVar("p"), boolVar("q")

	tests := []struct {
		name    string
		expr    symbolic.SymbolicExpression
		wantSat bool
	}{
		{"x + y > 5", bin(bin(x, symbolic.ADD, y), symbolic.GT, num(5)), true},
		{"x > 0 && y < 10", logic(symbolic.AND, bin(x, symbolic.GT, num(0)), bin(y, symbolic.LT, num(10))), true},
		{"x < y && y < x", logic(symbolic.AND, bin(x, symbolic.LT, y), bin(y, symbolic.LT, x)), false},
		{"x - x != 0", bin(bin(x, symbolic.SUB, x), symbolic.NE, num(0)), false},
		{"x * 2 == 7", bin(bin(x, symbolic.MUL, num(2)), symbolic.EQ, num(7)), false},
		{"x * 2 == 8", bin(bin(x, symbolic.MUL, num(2)), symbolic.EQ, num(8)), true},
		{"x >= 3 && x <= 3 && x != 3", logic(symbolic.AND,
			bin(x, symbolic.GE, num(3)), bin(x, symbolic.LE, num(3)), bin(x, symbolic.NE, num(3))), false},
		{"p && !p", logic(symbolic.AND, p, logic(symbolic.NOT, p)), false},
		{"p || !p", logic(symbolic.OR, p, logic(symbolic.NOT, p)), true},
		{"p && (p => q) && !q", logic(symbolic.AND, p, logic(symbolic.IMPLIES, p, q), logic(symbolic.NOT, q)), false},
		{"p != p", bin(p, symbolic.NE, p), false},
		{"p == q", bin(p, symbolic.EQ, q), true},
		{"false", symbolic.NewBoolConstant(false), false},
		{"single operand AND", logic(symbolic.AND, p), true},
	}
	for _, tt := range tests {
		if got := isSat(t, zt, tt.expr); got != tt.wantSat {
			t.Errorf("%s: sat = %v, want %v", tt.name, got, tt.wantSat)
		}
	}
}

func TestValidity(t *testing.T) {
	zt := NewZ3Translator()
	defer zt.Close()

	x, y := intVar("x"), intVar("y")
	p, q := boolVar("p"), boolVar("q")
	zero := num(0)
	neg := func(e symbolic.SymbolicExpression) symbolic.SymbolicExpression {
		return symbolic.NewUnaryOperation(e, symbolic.NEG)
	}
	abs := symbolic.NewConditionalExpression(bin(x, symbolic.LT, zero), neg(x), x)

	tests := []struct {
		name string
		expr symbolic.SymbolicExpression
	}{
		{"x + y == y + x", bin(bin(x, symbolic.ADD, y), symbolic.EQ, bin(y, symbolic.ADD, x))},
		{"-(-x) == x", bin(neg(neg(x)), symbolic.EQ, x)},
		{"x + (-x) == 0", bin(bin(x, symbolic.ADD, neg(x)), symbolic.EQ, zero)},
		{"abs(x) >= 0", bin(abs, symbolic.GE, zero)},
		{"De Morgan", bin(
			logic(symbolic.NOT, logic(symbolic.AND, p, q)),
			symbolic.EQ,
			logic(symbolic.OR, logic(symbolic.NOT, p), logic(symbolic.NOT, q)))},
		{"(p => q) == (!p || q)", bin(
			logic(symbolic.IMPLIES, p, q),
			symbolic.EQ,
			logic(symbolic.OR, logic(symbolic.NOT, p), q))},
		// Свойства деления и остатка из спецификации Go
		{"y != 0 => x == (x / y) * y + x % y", logic(symbolic.IMPLIES,
			bin(y, symbolic.NE, zero),
			bin(x, symbolic.EQ, bin(bin(bin(x, symbolic.DIV, y), symbolic.MUL, y), symbolic.ADD, bin(x, symbolic.MOD, y))))},
		{"x >= 0 && y != 0 => x % y >= 0", logic(symbolic.IMPLIES,
			logic(symbolic.AND, bin(x, symbolic.GE, zero), bin(y, symbolic.NE, zero)),
			bin(bin(x, symbolic.MOD, y), symbolic.GE, zero))},
		{"x <= 0 && y != 0 => x % y <= 0", logic(symbolic.IMPLIES,
			logic(symbolic.AND, bin(x, symbolic.LE, zero), bin(y, symbolic.NE, zero)),
			bin(bin(x, symbolic.MOD, y), symbolic.LE, zero))},
		{"y != 0 => (-x) / y == -(x / y)", logic(symbolic.IMPLIES,
			bin(y, symbolic.NE, zero),
			bin(bin(neg(x), symbolic.DIV, y), symbolic.EQ, neg(bin(x, symbolic.DIV, y))))},
	}
	for _, tt := range tests {
		if !isValid(t, zt, tt.expr) {
			t.Errorf("%s must be valid", tt.name)
		}
	}
}

// Деление и остаток должны совпадать с Go на конкретных значениях, включая отрицательные
func TestDivModMatchGo(t *testing.T) {
	zt := NewZ3Translator()
	defer zt.Close()

	values := []int64{-9, -7, -6, -1, 0, 1, 6, 7, 9}
	divisors := []int64{-7, -3, -2, -1, 1, 2, 3, 7}
	for _, l := range values {
		for _, r := range divisors {
			div := bin(bin(num(l), symbolic.DIV, num(r)), symbolic.EQ, num(l/r))
			if !isValid(t, zt, div) {
				t.Errorf("%d / %d must be %d", l, r, l/r)
			}
			mod := bin(bin(num(l), symbolic.MOD, num(r)), symbolic.EQ, num(l%r))
			if !isValid(t, zt, mod) {
				t.Errorf("%d %% %d must be %d", l, r, l%r)
			}
		}
	}
}

func TestModelValues(t *testing.T) {
	zt := NewZ3Translator()
	defer zt.Close()

	x, y := intVar("x"), intVar("y")
	// x + y == 10 && x - y == 4  =>  x = 7, y = 3
	solver := z3.NewSolver(zt.ctx)
	solver.Assert(translateBool(t, zt, bin(bin(x, symbolic.ADD, y), symbolic.EQ, num(10))))
	solver.Assert(translateBool(t, zt, bin(bin(x, symbolic.SUB, y), symbolic.EQ, num(4))))
	sat, err := solver.Check()
	if err != nil || !sat {
		t.Fatalf("sat = %v, err = %v", sat, err)
	}

	model := solver.Model()
	for name, want := range map[string]int64{"x": 7, "y": 3} {
		res, err := zt.TranslateExpression(intVar(name))
		if err != nil {
			t.Fatal(err)
		}
		got, isLiteral, ok := model.Eval(res.(z3.Int), true).(z3.Int).AsInt64()
		if !isLiteral || !ok || got != want {
			t.Errorf("%s = %d (literal %v, ok %v), want %d", name, got, isLiteral, ok, want)
		}
	}
}

func TestTranslationErrors(t *testing.T) {
	zt := NewZ3Translator()
	defer zt.Close()

	var te *TranslationError

	if _, err := zt.TranslateExpression(nil); !errors.As(err, &te) {
		t.Errorf("nil expression: err = %v, want TranslationError", err)
	}

	arr := symbolic.NewSymbolicVariable("arr", symbolic.ArrayType)
	if _, err := zt.TranslateExpression(arr); !errors.As(err, &te) {
		t.Errorf("array variable: err = %v, want TranslationError", err)
	}

	// Одно имя с разными типами
	if _, err := zt.TranslateExpression(intVar("v")); err != nil {
		t.Fatal(err)
	}
	if _, err := zt.TranslateExpression(boolVar("v")); !errors.As(err, &te) {
		t.Errorf("redeclared variable: err = %v, want TranslationError", err)
	}

	// Выражение, собранное в обход конструктора, с несовместимыми типами
	bad := &symbolic.BinaryOperation{Left: intVar("x"), Right: boolVar("p"), Operator: symbolic.ADD}
	if _, err := zt.TranslateExpression(bad); !errors.As(err, &te) {
		t.Errorf("ill-typed expression: err = %v, want TranslationError", err)
	}

	// После ошибки транслятор остаётся рабочим
	if _, err := zt.TranslateExpression(bin(intVar("x"), symbolic.GT, num(0))); err != nil {
		t.Errorf("translation after error: %v", err)
	}
}
