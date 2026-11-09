// Package translator содержит реализацию транслятора в Z3
package translator

import (
	"fmt"

	"github.com/ebukreev/go-z3/z3"
	"symbolic-execution-course/internal/symbolic"
)

// Z3Translator транслирует символьные выражения в Z3 формулы
type Z3Translator struct {
	ctx    *z3.Context
	config *z3.Config
	vars   map[string]z3.Value // Кэш переменных
}

// NewZ3Translator создаёт новый экземпляр Z3 транслятора
func NewZ3Translator() *Z3Translator {
	config := z3.NewContextConfig()
	ctx := z3.NewContext(config)

	return &Z3Translator{
		ctx:    ctx,
		config: config,
		vars:   make(map[string]z3.Value),
	}
}

// GetContext возвращает Z3 контекст
func (zt *Z3Translator) GetContext() interface{} {
	return zt.ctx
}

// Reset сбрасывает состояние транслятора
func (zt *Z3Translator) Reset() {
	zt.vars = make(map[string]z3.Value)
}

// Close освобождает ресурсы
func (zt *Z3Translator) Close() {
	// Z3 контекст закрывается автоматически
}

// TranslateExpression транслирует символьное выражение в Z3
func (zt *Z3Translator) TranslateExpression(expr symbolic.SymbolicExpression) (result interface{}, err error) {
	if expr == nil {
		return nil, NewTranslationError("nil expression", nil)
	}

	defer func() {
		if r := recover(); r != nil {
			te, ok := r.(*TranslationError)
			if !ok {
				panic(r)
			}
			result, err = nil, te
		}
	}()
	return expr.Accept(zt), nil
}

// VisitVariable транслирует символьную переменную в Z3
func (zt *Z3Translator) VisitVariable(expr *symbolic.SymbolicVariable) interface{} {
	// if var already exist, return it from cache
	if v, exists := zt.vars[expr.Name]; exists {
		if _, err := zt.castToZ3Type(v, expr.ExprType); err != nil {
			panic(NewTranslationError(
				fmt.Sprintf("variable %q is already declared with a different type", expr.Name), expr))
		}
		return v
	}

	v := zt.createZ3Variable(expr.Name, expr.ExprType)
	if v == nil {
		panic(NewTranslationError(
			fmt.Sprintf("unsupported type %s of variable %q", expr.ExprType, expr.Name), expr))
	}
	zt.vars[expr.Name] = v
	return v
}

// VisitIntConstant транслирует целочисленную константу в Z3
func (zt *Z3Translator) VisitIntConstant(expr *symbolic.IntConstant) interface{} {
	return zt.intLit(expr.Value)
}

// VisitBoolConstant транслирует булеву константу в Z3
func (zt *Z3Translator) VisitBoolConstant(expr *symbolic.BoolConstant) interface{} {
	return zt.ctx.FromBool(expr.Value)
}

// VisitFloatConstant транслирует вещественную константу в Z3
func (zt *Z3Translator) VisitFloatConstant(expr *symbolic.FloatConstant) interface{} {
	return zt.ctx.FromFloat64(expr.Value, zt.floatSort())
}

// VisitBinaryOperation транслирует бинарную операцию в Z3
func (zt *Z3Translator) VisitBinaryOperation(expr *symbolic.BinaryOperation) interface{} {
	left := expr.Left.Accept(zt)
	right := expr.Right.Accept(zt)

	if expr.Left.Type() == symbolic.FloatType {
		return zt.translateFloatOperation(expr, zt.asFloat(left, expr), zt.asFloat(right, expr))
	}

	// equal is both for bool and int
	if expr.Operator == symbolic.EQ || expr.Operator == symbolic.NE {
		if expr.Left.Type() == symbolic.BoolType {
			l, r := zt.asBool(left, expr), zt.asBool(right, expr)
			if expr.Operator == symbolic.EQ {
				return l.Eq(r)
			}
			return l.NE(r)
		}
		l, r := zt.asInt(left, expr), zt.asInt(right, expr)
		if expr.Operator == symbolic.EQ {
			return l.Eq(r)
		}
		return l.NE(r)
	}

	l, r := zt.asInt(left, expr), zt.asInt(right, expr)
	switch expr.Operator {
	case symbolic.ADD:
		return l.Add(r)
	case symbolic.SUB:
		return l.Sub(r)
	case symbolic.MUL:
		return l.Mul(r)
	case symbolic.DIV:
		return zt.truncDiv(l, r)
	case symbolic.MOD:
		// x % y == x - y*(x / y)
		return l.Sub(r.Mul(zt.truncDiv(l, r)))
	case symbolic.LT:
		return l.LT(r)
	case symbolic.LE:
		return l.LE(r)
	case symbolic.GT:
		return l.GT(r)
	case symbolic.GE:
		return l.GE(r)
	}

	lb, rb := l.ToBV(64), r.ToBV(64)
	switch expr.Operator {
	case symbolic.BIT_AND:
		return lb.And(rb).SToInt()
	case symbolic.BIT_OR:
		return lb.Or(rb).SToInt()
	case symbolic.BIT_XOR:
		return lb.Xor(rb).SToInt()
	case symbolic.BIT_AND_NOT:
		return lb.And(rb.Not()).SToInt()
	case symbolic.SHL:
		return lb.Lsh(rb).SToInt()
	case symbolic.SHR:
		return lb.SRsh(rb).SToInt()
	}
	panic(NewTranslationError(fmt.Sprintf("unsupported binary operator %s", expr.Operator), expr))
}

func (zt *Z3Translator) translateFloatOperation(expr *symbolic.BinaryOperation, l, r z3.Float) interface{} {
	switch expr.Operator {
	case symbolic.ADD:
		return l.Add(r)
	case symbolic.SUB:
		return l.Sub(r)
	case symbolic.MUL:
		return l.Mul(r)
	case symbolic.DIV:
		return l.Div(r)
	case symbolic.EQ:
		return l.IEEEEq(r)
	case symbolic.NE:
		return l.IEEEEq(r).Not()
	case symbolic.LT:
		return l.LT(r)
	case symbolic.LE:
		return l.LE(r)
	case symbolic.GT:
		return l.GT(r)
	case symbolic.GE:
		return l.GE(r)
	}
	panic(NewTranslationError(fmt.Sprintf("unsupported float operator %s", expr.Operator), expr))
}

// VisitLogicalOperation транслирует логическую операцию в Z3
func (zt *Z3Translator) VisitLogicalOperation(expr *symbolic.LogicalOperation) interface{} {
	operands := make([]z3.Bool, len(expr.Operands))
	for i, operand := range expr.Operands {
		operands[i] = zt.asBool(operand.Accept(zt), expr)
	}

	switch expr.Operator {
	case symbolic.AND:
		if len(operands) > 0 {
			return operands[0].And(operands[1:]...)
		}
	case symbolic.OR:
		if len(operands) > 0 {
			return operands[0].Or(operands[1:]...)
		}
	case symbolic.NOT:
		if len(operands) == 1 {
			return operands[0].Not()
		}
	case symbolic.IMPLIES:
		if len(operands) == 2 {
			return operands[0].Implies(operands[1])
		}
	default:
		panic(NewTranslationError(fmt.Sprintf("unsupported logical operator %s", expr.Operator), expr))
	}
	panic(NewTranslationError(
		fmt.Sprintf("wrong number of operands (%d) for logical operator %s", len(operands), expr.Operator), expr))
}

// VisitUnaryOperation транслирует унарную операцию в Z3
func (zt *Z3Translator) VisitUnaryOperation(expr *symbolic.UnaryOperation) interface{} {
	if expr.Operand.Type() == symbolic.FloatType {
		operand := zt.asFloat(expr.Operand.Accept(zt), expr)
		if expr.Operator == symbolic.NEG {
			return operand.Neg()
		}
		panic(NewTranslationError(fmt.Sprintf("unsupported float unary operator %s", expr.Operator), expr))
	}

	operand := zt.asInt(expr.Operand.Accept(zt), expr)
	switch expr.Operator {
	case symbolic.NEG:
		return operand.Neg()
	case symbolic.BIT_NOT:
		return operand.Neg().Sub(zt.intLit(1))
	}
	panic(NewTranslationError(fmt.Sprintf("unsupported unary operator %s", expr.Operator), expr))
}

// VisitConditionalExpression транслирует условное выражение в Z3 (if-then-else)
func (zt *Z3Translator) VisitConditionalExpression(expr *symbolic.ConditionalExpression) interface{} {
	cond := zt.asBool(expr.Condition.Accept(zt), expr)
	thenVal, err := zt.castToZ3Type(expr.Then.Accept(zt), expr.Type())
	if err != nil {
		panic(NewTranslationError(err.Error(), expr))
	}
	elseVal, err := zt.castToZ3Type(expr.Else.Accept(zt), expr.Type())
	if err != nil {
		panic(NewTranslationError(err.Error(), expr))
	}
	return cond.IfThenElse(thenVal, elseVal)
}

// Вспомогательные методы

// createZ3Variable создаёт Z3 переменную соответствующего типа
func (zt *Z3Translator) createZ3Variable(name string, exprType symbolic.ExpressionType) z3.Value {
	switch exprType {
	case symbolic.IntType:
		return zt.ctx.IntConst(name)
	case symbolic.BoolType:
		return zt.ctx.BoolConst(name)
	case symbolic.FloatType:
		return zt.ctx.Const(name, zt.floatSort())
	}
	return nil
}

// castToZ3Type приводит значение к нужному Z3 типу
func (zt *Z3Translator) castToZ3Type(value interface{}, targetType symbolic.ExpressionType) (z3.Value, error) {
	switch targetType {
	case symbolic.IntType:
		if v, ok := value.(z3.Int); ok {
			return v, nil
		}
	case symbolic.BoolType:
		if v, ok := value.(z3.Bool); ok {
			return v, nil
		}
	case symbolic.FloatType:
		if v, ok := value.(z3.Float); ok {
			return v, nil
		}
	default:
		return nil, fmt.Errorf("unsupported target type %s", targetType)
	}
	return nil, fmt.Errorf("expected Z3 value of type %s, got %T", targetType, value)
}

// asInt приводит результат трансляции к z3.Int; expr используется для сообщения об ошибке
func (zt *Z3Translator) asInt(value interface{}, expr symbolic.SymbolicExpression) z3.Int {
	v, err := zt.castToZ3Type(value, symbolic.IntType)
	if err != nil {
		panic(NewTranslationError(err.Error(), expr))
	}
	return v.(z3.Int)
}

// asBool приводит результат трансляции к z3.Bool; expr используется для сообщения об ошибке
func (zt *Z3Translator) asBool(value interface{}, expr symbolic.SymbolicExpression) z3.Bool {
	v, err := zt.castToZ3Type(value, symbolic.BoolType)
	if err != nil {
		panic(NewTranslationError(err.Error(), expr))
	}
	return v.(z3.Bool)
}

// asFloat приводит результат трансляции к z3.Float; expr используется для сообщения об ошибке
func (zt *Z3Translator) asFloat(value interface{}, expr symbolic.SymbolicExpression) z3.Float {
	v, err := zt.castToZ3Type(value, symbolic.FloatType)
	if err != nil {
		panic(NewTranslationError(err.Error(), expr))
	}
	return v.(z3.Float)
}

func (zt *Z3Translator) floatSort() z3.Sort {
	return zt.ctx.FloatSort(11, 53)
}

// intLit создаёт целочисленную константу Z3
func (zt *Z3Translator) intLit(value int64) z3.Int {
	return zt.ctx.FromInt(value, zt.ctx.IntSort()).(z3.Int)
}

func (zt *Z3Translator) truncDiv(l, r z3.Int) z3.Int {
	zero := zt.intLit(0)
	one := zt.intLit(1)
	q := l.Div(r)
	exact := l.Mod(r).Eq(zero)
	adjusted := r.GT(zero).IfThenElse(q.Add(one), q.Sub(one)).(z3.Int)
	return l.GE(zero).Or(exact).IfThenElse(q, adjusted).(z3.Int)
}
