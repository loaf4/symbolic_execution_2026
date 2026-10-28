package internal

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/ssa"
	"symbolic-execution-course/internal/memory"
	"symbolic-execution-course/internal/symbolic"
)

const maxBlockVisits = 20

const (
	StatusReturned    = "returned"
	StatusPanic       = "panic"
	StatusLoopLimit   = "loop limit reached"
	StatusUnsupported = "unsupported instruction"
	StatusUnknown     = "solver returned unknown"
	StatusOutOfBudget = "analysis budget exhausted"
)

type Interpreter struct {
	CallStack     []CallStackFrame
	Analyser      *Analyser
	PathCondition symbolic.SymbolicExpression
	Heap          memory.Memory
	Status        string
	Message       string
}

type CallStackFrame struct {
	Function    *ssa.Function
	LocalMemory map[string]symbolic.SymbolicExpression
	ReturnValue symbolic.SymbolicExpression
	Block       *ssa.BasicBlock
	InstrIndex  int
	BlockVisits map[int]int
}

func (interpreter *Interpreter) interpretDynamically(element ssa.Instruction) []Interpreter {
	frame := interpreter.frame()

	switch instr := element.(type) {
	case *ssa.BinOp:
		left := interpreter.resolveExpression(instr.X)
		right := interpreter.resolveExpression(instr.Y)
		if left == nil || right == nil {
			return interpreter.unsupported(element)
		}
		if (instr.Op == token.QUO || instr.Op == token.REM) && left.Type() == symbolic.IntType {
			return interpreter.interpretDivision(instr, left, right)
		}
		result := makeBinaryExpression(instr.Op, left, right)
		if result == nil {
			return interpreter.unsupported(element)
		}
		frame.LocalMemory[instr.Name()] = result
		frame.InstrIndex++
		return []Interpreter{*interpreter}

	case *ssa.UnOp:
		operand := interpreter.resolveExpression(instr.X)
		if operand == nil {
			return interpreter.unsupported(element)
		}
		result := makeUnaryExpression(instr.Op, operand)
		if result == nil {
			return interpreter.unsupported(element)
		}
		frame.LocalMemory[instr.Name()] = result
		frame.InstrIndex++
		return []Interpreter{*interpreter}

	case *ssa.Phi:
		frame.InstrIndex++
		return []Interpreter{*interpreter}

	case *ssa.MakeInterface:
		frame.InstrIndex++
		return []Interpreter{*interpreter}

	case *ssa.Jump:
		interpreter.goToBlock(frame.Block.Succs[0])
		return []Interpreter{*interpreter}

	case *ssa.If:
		cond := interpreter.resolveExpression(instr.Cond)
		if cond == nil {
			return interpreter.unsupported(element)
		}
		if c, ok := cond.(*symbolic.BoolConstant); ok {
			if c.Value {
				interpreter.goToBlock(frame.Block.Succs[0])
			} else {
				interpreter.goToBlock(frame.Block.Succs[1])
			}
			return []Interpreter{*interpreter}
		}
		notCond := symbolic.NewLogicalOperation([]symbolic.SymbolicExpression{cond}, symbolic.NOT)

		var result []Interpreter
		thenState, ok := interpreter.addCondition(cond)
		if ok {
			if thenState.Status == "" {
				thenState.goToBlock(frame.Block.Succs[0])
			}
			result = append(result, thenState)
		}
		elseState, ok := interpreter.addCondition(notCond)
		if ok {
			if elseState.Status == "" {
				elseState.goToBlock(frame.Block.Succs[1])
			}
			result = append(result, elseState)
		}
		return result

	case *ssa.Return:
		if len(instr.Results) > 1 {
			return interpreter.unsupported(element)
		}
		if len(instr.Results) == 1 {
			value := interpreter.resolveExpression(instr.Results[0])
			if value == nil {
				return interpreter.unsupported(element)
			}
			frame.ReturnValue = value
		}
		interpreter.Status = StatusReturned
		return []Interpreter{*interpreter}

	case *ssa.Panic:
		interpreter.Status = StatusPanic
		interpreter.Message = panicMessage(instr)
		return []Interpreter{*interpreter}
	}

	return interpreter.unsupported(element)
}

func (interpreter *Interpreter) resolveExpression(value ssa.Value) symbolic.SymbolicExpression {
	switch v := value.(type) {
	case *ssa.Const:
		return constToExpression(v)
	default:
		expr, ok := interpreter.frame().LocalMemory[value.Name()]
		if !ok {
			return nil
		}
		return expr
	}
}

func (interpreter Interpreter) String() string {
	result := "status: " + interpreter.Status
	if interpreter.Message != "" {
		result += " (" + interpreter.Message + ")"
	}
	result += "\n  path condition: " + interpreter.PathCondition.String()
	if interpreter.Status == StatusReturned && len(interpreter.CallStack) > 0 {
		returnValue := interpreter.CallStack[len(interpreter.CallStack)-1].ReturnValue
		if returnValue != nil {
			result += "\n  return value: " + returnValue.String()
		}
	}
	return result
}

func (interpreter *Interpreter) frame() *CallStackFrame {
	return &interpreter.CallStack[len(interpreter.CallStack)-1]
}

func (interpreter *Interpreter) copyState() Interpreter {
	newState := *interpreter
	newState.CallStack = make([]CallStackFrame, len(interpreter.CallStack))
	for i, frame := range interpreter.CallStack {
		newFrame := frame
		newFrame.LocalMemory = make(map[string]symbolic.SymbolicExpression)
		for name, value := range frame.LocalMemory {
			newFrame.LocalMemory[name] = value
		}
		newFrame.BlockVisits = make(map[int]int)
		for block, count := range frame.BlockVisits {
			newFrame.BlockVisits[block] = count
		}
		newState.CallStack[i] = newFrame
	}
	return newState
}

func (interpreter *Interpreter) addCondition(cond symbolic.SymbolicExpression) (Interpreter, bool) {
	newState := interpreter.copyState()
	if constant, ok := interpreter.PathCondition.(*symbolic.BoolConstant); ok && constant.Value {
		newState.PathCondition = cond
	} else {
		newState.PathCondition = symbolic.NewLogicalOperation(
			[]symbolic.SymbolicExpression{interpreter.PathCondition, cond}, symbolic.AND)
	}

	sat, err := interpreter.Analyser.isSatisfiable(newState.PathCondition)
	if err != nil {
		newState.Status = StatusUnknown
		newState.Message = err.Error()
		return newState, true
	}
	return newState, sat
}

func (interpreter *Interpreter) goToBlock(target *ssa.BasicBlock) {
	frame := interpreter.frame()

	frame.BlockVisits[target.Index]++
	if frame.BlockVisits[target.Index] > maxBlockVisits {
		interpreter.Status = StatusLoopLimit
		interpreter.Message = fmt.Sprintf("block %d visited more than %d times", target.Index, maxBlockVisits)
		return
	}

	newValues := make(map[string]symbolic.SymbolicExpression)
	for _, instr := range target.Instrs {
		phi, ok := instr.(*ssa.Phi)
		if !ok {
			break
		}
		var value symbolic.SymbolicExpression
		for i, pred := range target.Preds {
			if pred == frame.Block {
				value = interpreter.resolveExpression(phi.Edges[i])
			}
		}
		if value == nil {
			interpreter.Status = StatusUnsupported
			interpreter.Message = phi.String()
			return
		}
		newValues[phi.Name()] = value
	}
	for name, value := range newValues {
		frame.LocalMemory[name] = value
	}

	frame.Block = target
	frame.InstrIndex = 0
}

func (interpreter *Interpreter) interpretDivision(instr *ssa.BinOp, left, right symbolic.SymbolicExpression) []Interpreter {
	if c, ok := right.(*symbolic.IntConstant); ok && c.Value != 0 {
		interpreter.frame().LocalMemory[instr.Name()] = makeBinaryExpression(instr.Op, left, right)
		interpreter.frame().InstrIndex++
		return []Interpreter{*interpreter}
	}

	zero := symbolic.NewIntConstant(0)
	var result []Interpreter

	okState, ok := interpreter.addCondition(symbolic.NewBinaryOperation(right, zero, symbolic.NE))
	if ok {
		if okState.Status == "" {
			okState.frame().LocalMemory[instr.Name()] = makeBinaryExpression(instr.Op, left, right)
			okState.frame().InstrIndex++
		}
		result = append(result, okState)
	}

	panicState, ok := interpreter.addCondition(symbolic.NewBinaryOperation(right, zero, symbolic.EQ))
	if ok {
		if panicState.Status == "" {
			panicState.Status = StatusPanic
			panicState.Message = "integer divide by zero"
		}
		result = append(result, panicState)
	}
	return result
}

func (interpreter *Interpreter) unsupported(instr ssa.Instruction) []Interpreter {
	interpreter.Status = StatusUnsupported
	interpreter.Message = instr.String()
	return []Interpreter{*interpreter}
}

func makeBinaryExpression(op token.Token, left, right symbolic.SymbolicExpression) symbolic.SymbolicExpression {
	var operator symbolic.BinaryOperator
	switch op {
	case token.ADD:
		operator = symbolic.ADD
	case token.SUB:
		operator = symbolic.SUB
	case token.MUL:
		operator = symbolic.MUL
	case token.QUO:
		operator = symbolic.DIV
	case token.REM:
		operator = symbolic.MOD
	case token.EQL:
		operator = symbolic.EQ
	case token.NEQ:
		operator = symbolic.NE
	case token.LSS:
		operator = symbolic.LT
	case token.LEQ:
		operator = symbolic.LE
	case token.GTR:
		operator = symbolic.GT
	case token.GEQ:
		operator = symbolic.GE
	case token.AND:
		operator = symbolic.BIT_AND
	case token.OR:
		operator = symbolic.BIT_OR
	case token.XOR:
		operator = symbolic.BIT_XOR
	case token.AND_NOT:
		operator = symbolic.BIT_AND_NOT
	case token.SHL:
		operator = symbolic.SHL
	case token.SHR:
		operator = symbolic.SHR
	default:
		return nil
	}

	folded := foldBinary(left, right, operator)
	if folded != nil {
		return folded
	}
	return symbolic.NewBinaryOperation(left, right, operator)
}

func makeUnaryExpression(op token.Token, operand symbolic.SymbolicExpression) symbolic.SymbolicExpression {
	switch op {
	case token.SUB:
		if c, ok := operand.(*symbolic.IntConstant); ok {
			return symbolic.NewIntConstant(-c.Value)
		}
		if c, ok := operand.(*symbolic.FloatConstant); ok {
			return symbolic.NewFloatConstant(-c.Value)
		}
		return symbolic.NewUnaryOperation(operand, symbolic.NEG)
	case token.XOR:
		if c, ok := operand.(*symbolic.IntConstant); ok {
			return symbolic.NewIntConstant(^c.Value)
		}
		return symbolic.NewUnaryOperation(operand, symbolic.BIT_NOT)
	case token.NOT:
		if c, ok := operand.(*symbolic.BoolConstant); ok {
			return symbolic.NewBoolConstant(!c.Value)
		}
		return symbolic.NewLogicalOperation([]symbolic.SymbolicExpression{operand}, symbolic.NOT)
	}
	return nil
}

func foldBinary(left, right symbolic.SymbolicExpression, op symbolic.BinaryOperator) symbolic.SymbolicExpression {
	leftInt, leftIsInt := left.(*symbolic.IntConstant)
	rightInt, rightIsInt := right.(*symbolic.IntConstant)
	if leftIsInt && rightIsInt {
		a, b := leftInt.Value, rightInt.Value
		switch op {
		case symbolic.ADD:
			return symbolic.NewIntConstant(a + b)
		case symbolic.SUB:
			return symbolic.NewIntConstant(a - b)
		case symbolic.MUL:
			return symbolic.NewIntConstant(a * b)
		case symbolic.DIV:
			if b != 0 {
				return symbolic.NewIntConstant(a / b)
			}
		case symbolic.MOD:
			if b != 0 {
				return symbolic.NewIntConstant(a % b)
			}
		case symbolic.EQ:
			return symbolic.NewBoolConstant(a == b)
		case symbolic.NE:
			return symbolic.NewBoolConstant(a != b)
		case symbolic.LT:
			return symbolic.NewBoolConstant(a < b)
		case symbolic.LE:
			return symbolic.NewBoolConstant(a <= b)
		case symbolic.GT:
			return symbolic.NewBoolConstant(a > b)
		case symbolic.GE:
			return symbolic.NewBoolConstant(a >= b)
		case symbolic.BIT_AND:
			return symbolic.NewIntConstant(a & b)
		case symbolic.BIT_OR:
			return symbolic.NewIntConstant(a | b)
		case symbolic.BIT_XOR:
			return symbolic.NewIntConstant(a ^ b)
		case symbolic.BIT_AND_NOT:
			return symbolic.NewIntConstant(a &^ b)
		case symbolic.SHL:
			if b >= 0 {
				return symbolic.NewIntConstant(a << b)
			}
		case symbolic.SHR:
			if b >= 0 {
				return symbolic.NewIntConstant(a >> b)
			}
		}
		return nil
	}

	leftFloat, leftIsFloat := left.(*symbolic.FloatConstant)
	rightFloat, rightIsFloat := right.(*symbolic.FloatConstant)
	if leftIsFloat && rightIsFloat {
		a, b := leftFloat.Value, rightFloat.Value
		switch op {
		case symbolic.ADD:
			return symbolic.NewFloatConstant(a + b)
		case symbolic.SUB:
			return symbolic.NewFloatConstant(a - b)
		case symbolic.MUL:
			return symbolic.NewFloatConstant(a * b)
		case symbolic.DIV:
			return symbolic.NewFloatConstant(a / b)
		case symbolic.EQ:
			return symbolic.NewBoolConstant(a == b)
		case symbolic.NE:
			return symbolic.NewBoolConstant(a != b)
		case symbolic.LT:
			return symbolic.NewBoolConstant(a < b)
		case symbolic.LE:
			return symbolic.NewBoolConstant(a <= b)
		case symbolic.GT:
			return symbolic.NewBoolConstant(a > b)
		case symbolic.GE:
			return symbolic.NewBoolConstant(a >= b)
		}
		return nil
	}

	leftBool, leftIsBool := left.(*symbolic.BoolConstant)
	rightBool, rightIsBool := right.(*symbolic.BoolConstant)
	if leftIsBool && rightIsBool {
		switch op {
		case symbolic.EQ:
			return symbolic.NewBoolConstant(leftBool.Value == rightBool.Value)
		case symbolic.NE:
			return symbolic.NewBoolConstant(leftBool.Value != rightBool.Value)
		}
	}
	return nil
}

func convertType(t types.Type) (symbolic.ExpressionType, bool) {
	basic, ok := t.Underlying().(*types.Basic)
	if !ok {
		return 0, false
	}
	info := basic.Info()
	if info&types.IsBoolean != 0 {
		return symbolic.BoolType, true
	}
	if info&types.IsInteger != 0 {
		return symbolic.IntType, true
	}
	if info&types.IsFloat != 0 {
		return symbolic.FloatType, true
	}
	return 0, false
}

func constToExpression(c *ssa.Const) symbolic.SymbolicExpression {
	exprType, ok := convertType(c.Type())
	if !ok {
		return nil
	}
	switch exprType {
	case symbolic.BoolType:
		if c.Value == nil {
			return symbolic.NewBoolConstant(false)
		}
		return symbolic.NewBoolConstant(constant.BoolVal(c.Value))
	case symbolic.IntType:
		if c.Value == nil {
			return symbolic.NewIntConstant(0)
		}
		return symbolic.NewIntConstant(c.Int64())
	case symbolic.FloatType:
		if c.Value == nil {
			return symbolic.NewFloatConstant(0)
		}
		return symbolic.NewFloatConstant(c.Float64())
	}
	return nil
}

func panicMessage(instr *ssa.Panic) string {
	if makeInterface, ok := instr.X.(*ssa.MakeInterface); ok {
		if c, ok := makeInterface.X.(*ssa.Const); ok && c.Value != nil {
			return c.Value.String()
		}
	}
	return "panic"
}
