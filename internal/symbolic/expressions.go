// Package symbolic содержит конкретные реализации символьных выражений
package symbolic

import (
	"fmt"
	"strings"
)

// SymbolicExpression - базовый интерфейс для всех символьных выражений
type SymbolicExpression interface {
	// Type возвращает тип выражения
	Type() ExpressionType

	// String возвращает строковое представление выражения
	String() string

	// Accept принимает visitor для обхода дерева выражений
	Accept(visitor Visitor) interface{}
}

// SymbolicVariable представляет символьную переменную
type SymbolicVariable struct {
	Name     string
	ExprType ExpressionType
}

// NewSymbolicVariable создаёт новую символьную переменную
func NewSymbolicVariable(name string, exprType ExpressionType) *SymbolicVariable {
	return &SymbolicVariable{
		Name:     name,
		ExprType: exprType,
	}
}

// Type возвращает тип переменной
func (sv *SymbolicVariable) Type() ExpressionType {
	return sv.ExprType
}

// String возвращает строковое представление переменной
func (sv *SymbolicVariable) String() string {
	return sv.Name
}

// Accept реализует Visitor pattern
func (sv *SymbolicVariable) Accept(visitor Visitor) interface{} {
	return visitor.VisitVariable(sv)
}

// IntConstant представляет целочисленную константу
type IntConstant struct {
	Value int64
}

// NewIntConstant создаёт новую целочисленную константу
func NewIntConstant(value int64) *IntConstant {
	return &IntConstant{Value: value}
}

// Type возвращает тип константы
func (ic *IntConstant) Type() ExpressionType {
	return IntType
}

// String возвращает строковое представление константы
func (ic *IntConstant) String() string {
	return fmt.Sprintf("%d", ic.Value)
}

// Accept реализует Visitor pattern
func (ic *IntConstant) Accept(visitor Visitor) interface{} {
	return visitor.VisitIntConstant(ic)
}

// BoolConstant представляет булеву константу
type BoolConstant struct {
	Value bool
}

// NewBoolConstant создаёт новую булеву константу
func NewBoolConstant(value bool) *BoolConstant {
	return &BoolConstant{Value: value}
}

// Type возвращает тип константы
func (bc *BoolConstant) Type() ExpressionType {
	return BoolType
}

// String возвращает строковое представление константы
func (bc *BoolConstant) String() string {
	return fmt.Sprintf("%t", bc.Value)
}

// Accept реализует Visitor pattern
func (bc *BoolConstant) Accept(visitor Visitor) interface{} {
	return visitor.VisitBoolConstant(bc)
}

// BinaryOperation представляет бинарную операцию
type BinaryOperation struct {
	Left     SymbolicExpression
	Right    SymbolicExpression
	Operator BinaryOperator
}

// NewBinaryOperation создаёт новую бинарную операцию
func NewBinaryOperation(left, right SymbolicExpression, op BinaryOperator) *BinaryOperation {
	if left == nil || right == nil {
		panic(fmt.Sprintf("binary operation %s: nil operand", op))
	}
	lt, rt := left.Type(), right.Type()
	switch {
	case op.IsArithmetic(), op.IsOrdering():
		if lt != IntType || rt != IntType {
			panic(fmt.Sprintf("binary operation %s: expected int operands, got %s and %s", op, lt, rt))
		}
	case op == EQ || op == NE:
		if lt != rt || (lt != IntType && lt != BoolType) {
			panic(fmt.Sprintf("binary operation %s: incompatible operand types %s and %s", op, lt, rt))
		}
	default:
		panic(fmt.Sprintf("unknown binary operator %d", int(op)))
	}
	return &BinaryOperation{Left: left, Right: right, Operator: op}
}

// Type возвращает результирующий тип операции
func (bo *BinaryOperation) Type() ExpressionType {
	// int + int = int, int < int = bool
	if bo.Operator.IsArithmetic() {
		return IntType
	}
	return BoolType
}

// String возвращает строковое представление операции
func (bo *BinaryOperation) String() string {
	return fmt.Sprintf("(%s %s %s)", bo.Left, bo.Operator, bo.Right)
}

// Accept реализует Visitor pattern
func (bo *BinaryOperation) Accept(visitor Visitor) interface{} {
	return visitor.VisitBinaryOperation(bo)
}

// LogicalOperation представляет логическую операцию
type LogicalOperation struct {
	Operands []SymbolicExpression
	Operator LogicalOperator
}

// NewLogicalOperation создаёт новую логическую операцию
func NewLogicalOperation(operands []SymbolicExpression, op LogicalOperator) *LogicalOperation {
	switch op {
	case AND, OR:
		if len(operands) == 0 {
			panic(fmt.Sprintf("logical operation %s: expected at least 1 operand", op))
		}
	case NOT:
		if len(operands) != 1 {
			panic(fmt.Sprintf("logical operation %s: expected 1 operand, got %d", op, len(operands)))
		}
	case IMPLIES:
		if len(operands) != 2 {
			panic(fmt.Sprintf("logical operation %s: expected 2 operands, got %d", op, len(operands)))
		}
	default:
		panic(fmt.Sprintf("unknown logical operator %d", int(op)))
	}
	for i, operand := range operands {
		if operand == nil {
			panic(fmt.Sprintf("logical operation %s: operand %d is nil", op, i))
		}
		if operand.Type() != BoolType {
			panic(fmt.Sprintf("logical operation %s: operand %d has type %s, expected bool", op, i, operand.Type()))
		}
	}
	return &LogicalOperation{Operands: append([]SymbolicExpression(nil), operands...), Operator: op}
}

// Type возвращает тип логической операции (всегда bool)
func (lo *LogicalOperation) Type() ExpressionType {
	return BoolType
}

// String возвращает строковое представление логической операции
func (lo *LogicalOperation) String() string {
	// Для NOT: "!operand"
	if lo.Operator == NOT {
		return "!" + lo.Operands[0].String()
	}
	parts := make([]string, len(lo.Operands))
	for i, operand := range lo.Operands {
		parts[i] = operand.String()
	}
	return "(" + strings.Join(parts, " "+lo.Operator.String()+" ") + ")"
}

// Accept реализует Visitor pattern
func (lo *LogicalOperation) Accept(visitor Visitor) interface{} {
	return visitor.VisitLogicalOperation(lo)
}

// Операторы для бинарных выражений
type BinaryOperator int

const (
	// Арифметические операторы
	ADD BinaryOperator = iota
	SUB
	MUL
	DIV
	MOD

	// Операторы сравнения
	EQ // равно
	NE // не равно
	LT // меньше
	LE // меньше или равно
	GT // больше
	GE // больше или равно
)

func (op BinaryOperator) IsArithmetic() bool {
	return op >= ADD && op <= MOD
}

func (op BinaryOperator) IsOrdering() bool {
	return op >= LT && op <= GE
}

func (op BinaryOperator) IsComparison() bool {
	return op >= EQ && op <= GE
}

// String возвращает строковое представление оператора
func (op BinaryOperator) String() string {
	switch op {
	case ADD:
		return "+"
	case SUB:
		return "-"
	case MUL:
		return "*"
	case DIV:
		return "/"
	case MOD:
		return "%"
	case EQ:
		return "=="
	case NE:
		return "!="
	case LT:
		return "<"
	case LE:
		return "<="
	case GT:
		return ">"
	case GE:
		return ">="
	default:
		return "unknown"
	}
}

// Логические операторы
type LogicalOperator int

const (
	AND LogicalOperator = iota
	OR
	NOT
	IMPLIES
)

// String возвращает строковое представление логического оператора
func (op LogicalOperator) String() string {
	switch op {
	case AND:
		return "&&"
	case OR:
		return "||"
	case NOT:
		return "!"
	case IMPLIES:
		return "=>"
	default:
		return "unknown"
	}
}

type UnaryOperation struct {
	Operand  SymbolicExpression
	Operator UnaryOperator
}

func NewUnaryOperation(operand SymbolicExpression, op UnaryOperator) *UnaryOperation {
	if operand == nil {
		panic(fmt.Sprintf("unary operation %s: nil operand", op))
	}
	if op != NEG {
		panic(fmt.Sprintf("unknown unary operator %d", int(op)))
	}
	if operand.Type() != IntType {
		panic(fmt.Sprintf("unary operation %s: expected int operand, got %s", op, operand.Type()))
	}
	return &UnaryOperation{Operand: operand, Operator: op}
}

func (uo *UnaryOperation) Type() ExpressionType {
	return IntType
}

func (uo *UnaryOperation) String() string {
	return fmt.Sprintf("(%s%s)", uo.Operator, uo.Operand)
}

func (uo *UnaryOperation) Accept(visitor Visitor) interface{} {
	return visitor.VisitUnaryOperation(uo)
}

type UnaryOperator int

const (
	NEG UnaryOperator = iota // арифметическое отрицание
)

func (op UnaryOperator) String() string {
	switch op {
	case NEG:
		return "-"
	default:
		return "unknown"
	}
}

type ConditionalExpression struct {
	Condition SymbolicExpression
	Then      SymbolicExpression
	Else      SymbolicExpression
}

func NewConditionalExpression(condition, thenExpr, elseExpr SymbolicExpression) *ConditionalExpression {
	if condition == nil || thenExpr == nil || elseExpr == nil {
		panic("conditional expression: nil operand")
	}
	if condition.Type() != BoolType {
		panic(fmt.Sprintf("conditional expression: condition has type %s, expected bool", condition.Type()))
	}
	if thenExpr.Type() != elseExpr.Type() {
		panic(fmt.Sprintf("conditional expression: branch types differ: %s and %s", thenExpr.Type(), elseExpr.Type()))
	}
	return &ConditionalExpression{Condition: condition, Then: thenExpr, Else: elseExpr}
}

func (ce *ConditionalExpression) Type() ExpressionType {
	return ce.Then.Type()
}

func (ce *ConditionalExpression) String() string {
	return fmt.Sprintf("(%s ? %s : %s)", ce.Condition, ce.Then, ce.Else)
}

func (ce *ConditionalExpression) Accept(visitor Visitor) interface{} {
	return visitor.VisitConditionalExpression(ce)
}

type Ref struct {
	// TODO: Выбрать и написать внутреннее представление символьной ссылки
}

func (ref *Ref) Type() ExpressionType {
	panic("не реализовано")
}

func (ref *Ref) String() string {
	panic("не реализовано")
}

func (ref *Ref) Accept(visitor Visitor) interface{} {
	panic("не реализовано")
}

// TODO: Добавьте дополнительные типы выражений по необходимости:
// - ArrayAccess (доступ к элементам массива: arr[index])
// - FunctionCall (вызовы функций: f(x, y))
