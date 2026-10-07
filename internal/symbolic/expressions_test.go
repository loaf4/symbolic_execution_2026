package symbolic

import "testing"

// mustPanic проверяет, что f завершается паникой
func mustPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s: expected panic", name)
		}
	}()
	f()
}

func TestBinaryOperationTypeAndString(t *testing.T) {
	x := NewSymbolicVariable("x", IntType)
	y := NewSymbolicVariable("y", IntType)
	p := NewSymbolicVariable("p", BoolType)

	tests := []struct {
		expr     SymbolicExpression
		wantType ExpressionType
		wantStr  string
	}{
		{NewBinaryOperation(x, y, ADD), IntType, "(x + y)"},
		{NewBinaryOperation(x, y, SUB), IntType, "(x - y)"},
		{NewBinaryOperation(x, y, MUL), IntType, "(x * y)"},
		{NewBinaryOperation(x, y, DIV), IntType, "(x / y)"},
		{NewBinaryOperation(x, y, MOD), IntType, "(x % y)"},
		{NewBinaryOperation(x, y, EQ), BoolType, "(x == y)"},
		{NewBinaryOperation(x, y, NE), BoolType, "(x != y)"},
		{NewBinaryOperation(x, y, LT), BoolType, "(x < y)"},
		{NewBinaryOperation(x, y, LE), BoolType, "(x <= y)"},
		{NewBinaryOperation(x, y, GT), BoolType, "(x > y)"},
		{NewBinaryOperation(x, y, GE), BoolType, "(x >= y)"},
		{NewBinaryOperation(p, NewBoolConstant(true), EQ), BoolType, "(p == true)"},
		{
			NewBinaryOperation(NewBinaryOperation(x, y, ADD), NewIntConstant(5), GT),
			BoolType, "((x + y) > 5)",
		},
	}
	for _, tt := range tests {
		if got := tt.expr.Type(); got != tt.wantType {
			t.Errorf("%s: type = %s, want %s", tt.wantStr, got, tt.wantType)
		}
		if got := tt.expr.String(); got != tt.wantStr {
			t.Errorf("string = %q, want %q", got, tt.wantStr)
		}
	}
}

func TestBinaryOperationTypeErrors(t *testing.T) {
	x := NewSymbolicVariable("x", IntType)
	p := NewSymbolicVariable("p", BoolType)
	arr := NewSymbolicVariable("arr", ArrayType)

	mustPanic(t, "int + bool", func() { NewBinaryOperation(x, p, ADD) })
	mustPanic(t, "bool + bool", func() { NewBinaryOperation(p, p, ADD) })
	mustPanic(t, "bool < bool", func() { NewBinaryOperation(p, p, LT) })
	mustPanic(t, "int == bool", func() { NewBinaryOperation(x, p, EQ) })
	mustPanic(t, "array == array", func() { NewBinaryOperation(arr, arr, EQ) })
	mustPanic(t, "nil operand", func() { NewBinaryOperation(x, nil, ADD) })
	mustPanic(t, "unknown operator", func() { NewBinaryOperation(x, x, BinaryOperator(100)) })
}

func TestLogicalOperationString(t *testing.T) {
	p := NewSymbolicVariable("p", BoolType)
	q := NewSymbolicVariable("q", BoolType)
	r := NewSymbolicVariable("r", BoolType)

	tests := []struct {
		expr *LogicalOperation
		want string
	}{
		{NewLogicalOperation([]SymbolicExpression{p, q, r}, AND), "(p && q && r)"},
		{NewLogicalOperation([]SymbolicExpression{p, q}, OR), "(p || q)"},
		{NewLogicalOperation([]SymbolicExpression{p}, NOT), "!p"},
		{NewLogicalOperation([]SymbolicExpression{p, q}, IMPLIES), "(p => q)"},
		{
			NewLogicalOperation([]SymbolicExpression{
				NewLogicalOperation([]SymbolicExpression{p, q}, AND),
			}, NOT),
			"!(p && q)",
		},
	}
	for _, tt := range tests {
		if got := tt.expr.String(); got != tt.want {
			t.Errorf("string = %q, want %q", got, tt.want)
		}
		if got := tt.expr.Type(); got != BoolType {
			t.Errorf("%s: type = %s, want bool", tt.want, got)
		}
	}
}

func TestLogicalOperationErrors(t *testing.T) {
	x := NewSymbolicVariable("x", IntType)
	p := NewSymbolicVariable("p", BoolType)

	mustPanic(t, "int operand", func() { NewLogicalOperation([]SymbolicExpression{p, x}, AND) })
	mustPanic(t, "empty AND", func() { NewLogicalOperation(nil, AND) })
	mustPanic(t, "NOT with 2 operands", func() { NewLogicalOperation([]SymbolicExpression{p, p}, NOT) })
	mustPanic(t, "IMPLIES with 1 operand", func() { NewLogicalOperation([]SymbolicExpression{p}, IMPLIES) })
	mustPanic(t, "nil operand", func() { NewLogicalOperation([]SymbolicExpression{nil}, NOT) })
}

func TestLogicalOperationCopiesOperands(t *testing.T) {
	p := NewSymbolicVariable("p", BoolType)
	q := NewSymbolicVariable("q", BoolType)
	operands := []SymbolicExpression{p, q}
	and := NewLogicalOperation(operands, AND)
	operands[0] = q
	if got := and.String(); got != "(p && q)" {
		t.Errorf("string = %q after modifying source slice, want (p && q)", got)
	}
}

func TestUnaryAndConditional(t *testing.T) {
	x := NewSymbolicVariable("x", IntType)
	p := NewSymbolicVariable("p", BoolType)

	neg := NewUnaryOperation(x, NEG)
	if neg.Type() != IntType || neg.String() != "(-x)" {
		t.Errorf("neg: type %s, string %q", neg.Type(), neg.String())
	}
	mustPanic(t, "-bool", func() { NewUnaryOperation(p, NEG) })

	// abs(x) = x < 0 ? -x : x
	abs := NewConditionalExpression(NewBinaryOperation(x, NewIntConstant(0), LT), neg, x)
	if abs.Type() != IntType || abs.String() != "((x < 0) ? (-x) : x)" {
		t.Errorf("abs: type %s, string %q", abs.Type(), abs.String())
	}
	boolCond := NewConditionalExpression(p, NewBoolConstant(true), p)
	if boolCond.Type() != BoolType {
		t.Errorf("bool conditional: type %s", boolCond.Type())
	}
	mustPanic(t, "int condition", func() { NewConditionalExpression(x, x, x) })
	mustPanic(t, "different branch types", func() { NewConditionalExpression(p, x, p) })
}

func TestCollectVariables(t *testing.T) {
	x := NewSymbolicVariable("x", IntType)
	y := NewSymbolicVariable("y", IntType)
	p := NewSymbolicVariable("p", BoolType)

	// p ? (x + y) : (-x) > 0 && p
	expr := NewLogicalOperation([]SymbolicExpression{
		NewBinaryOperation(
			NewConditionalExpression(p, NewBinaryOperation(x, y, ADD), NewUnaryOperation(x, NEG)),
			NewIntConstant(0), GT),
		p,
	}, AND)

	vars := CollectVariables(expr)
	want := []string{"p", "x", "y"}
	if len(vars) != len(want) {
		t.Fatalf("got %d variables, want %d", len(vars), len(want))
	}
	for i, name := range want {
		if vars[i].Name != name {
			t.Errorf("variable %d = %s, want %s", i, vars[i].Name, name)
		}
	}
}
