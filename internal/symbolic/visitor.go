package symbolic

// Visitor интерфейс для обхода символьных выражений (Visitor Pattern)
type Visitor interface {
	VisitVariable(expr *SymbolicVariable) interface{}
	VisitIntConstant(expr *IntConstant) interface{}
	VisitBoolConstant(expr *BoolConstant) interface{}
	VisitFloatConstant(expr *FloatConstant) interface{}
	VisitBinaryOperation(expr *BinaryOperation) interface{}
	VisitLogicalOperation(expr *LogicalOperation) interface{}
	VisitUnaryOperation(expr *UnaryOperation) interface{}
	VisitConditionalExpression(expr *ConditionalExpression) interface{}
	// TODO: Добавьте методы для других типов выражений по мере необходимости
}

type VariableCollector struct {
	seen map[string]bool
	vars []*SymbolicVariable
}

func CollectVariables(expr SymbolicExpression) []*SymbolicVariable {
	collector := &VariableCollector{seen: make(map[string]bool)}
	expr.Accept(collector)
	return collector.vars
}

func (vc *VariableCollector) VisitVariable(expr *SymbolicVariable) interface{} {
	if !vc.seen[expr.Name] {
		vc.seen[expr.Name] = true
		vc.vars = append(vc.vars, expr)
	}
	return nil
}

func (vc *VariableCollector) VisitIntConstant(expr *IntConstant) interface{} {
	return nil
}

func (vc *VariableCollector) VisitBoolConstant(expr *BoolConstant) interface{} {
	return nil
}

func (vc *VariableCollector) VisitFloatConstant(expr *FloatConstant) interface{} {
	return nil
}

func (vc *VariableCollector) VisitBinaryOperation(expr *BinaryOperation) interface{} {
	expr.Left.Accept(vc)
	expr.Right.Accept(vc)
	return nil
}

func (vc *VariableCollector) VisitLogicalOperation(expr *LogicalOperation) interface{} {
	for _, operand := range expr.Operands {
		operand.Accept(vc)
	}
	return nil
}

func (vc *VariableCollector) VisitUnaryOperation(expr *UnaryOperation) interface{} {
	expr.Operand.Accept(vc)
	return nil
}

func (vc *VariableCollector) VisitConditionalExpression(expr *ConditionalExpression) interface{} {
	expr.Condition.Accept(vc)
	expr.Then.Accept(vc)
	expr.Else.Accept(vc)
	return nil
}
