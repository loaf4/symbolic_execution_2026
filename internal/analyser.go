package internal

import (
	"container/heap"
	"fmt"

	"github.com/ebukreev/go-z3/z3"
	"golang.org/x/tools/go/ssa"
	ssabuilder "symbolic-execution-course/internal/ssa"
	"symbolic-execution-course/internal/symbolic"
	"symbolic-execution-course/internal/translator"
)

const maxSteps = 10000
const solverTimeoutMs = 5000

type Analyser struct {
	Package      *ssa.Package
	StatesQueue  PriorityQueue
	PathSelector PathSelector
	Results      []Interpreter
	Z3Translator *translator.Z3Translator
}

func Analyse(source string, functionName string) []Interpreter {
	builder := ssabuilder.NewBuilder()
	function, err := builder.ParseAndBuildSSA(source, functionName)
	if err != nil {
		fmt.Println("analysis failed:", err)
		return nil
	}

	analyser := &Analyser{
		Package:      function.Pkg,
		PathSelector: &DfsPathSelector{},
		Z3Translator: translator.NewZ3Translator(),
	}
	ctx := analyser.Z3Translator.GetContext().(*z3.Context)
	ctx.Config().SetUint("timeout", solverTimeoutMs)

	heap.Init(&analyser.StatesQueue)
	analyser.addState(analyser.initialState(function))

	steps := 0
	for analyser.StatesQueue.Len() > 0 && steps < maxSteps {
		state := heap.Pop(&analyser.StatesQueue).(*Item).value
		frame := state.frame()
		instr := frame.Block.Instrs[frame.InstrIndex]

		newStates := state.interpretDynamically(instr)
		steps++

		for _, newState := range newStates {
			if newState.Status != "" {
				// Путь закончился, сохраняем результат
				analyser.Results = append(analyser.Results, newState)
			} else {
				analyser.addState(newState)
			}
		}
	}

	for analyser.StatesQueue.Len() > 0 {
		state := heap.Pop(&analyser.StatesQueue).(*Item).value
		state.Status = StatusOutOfBudget
		analyser.Results = append(analyser.Results, state)
	}

	return analyser.Results
}

func (analyser *Analyser) initialState(function *ssa.Function) Interpreter {
	frame := CallStackFrame{
		Function:    function,
		LocalMemory: make(map[string]symbolic.SymbolicExpression),
		Block:       function.Blocks[0],
		InstrIndex:  0,
		BlockVisits: map[int]int{0: 1},
	}
	for _, param := range function.Params {
		paramType, ok := convertType(param.Type())
		if ok {
			frame.LocalMemory[param.Name()] = symbolic.NewSymbolicVariable(param.Name(), paramType)
		}
	}

	return Interpreter{
		CallStack:     []CallStackFrame{frame},
		Analyser:      analyser,
		PathCondition: symbolic.NewBoolConstant(true),
	}
}

func (analyser *Analyser) addState(state Interpreter) {
	item := &Item{
		value:    state,
		priority: analyser.PathSelector.CalculatePriority(state),
	}
	heap.Push(&analyser.StatesQueue, item)
}

func (analyser *Analyser) isSatisfiable(pathCondition symbolic.SymbolicExpression) (bool, error) {
	z3Expr, err := analyser.Z3Translator.TranslateExpression(pathCondition)
	if err != nil {
		return false, err
	}
	ctx := analyser.Z3Translator.GetContext().(*z3.Context)
	solver := z3.NewSolver(ctx)
	solver.Assert(z3Expr.(z3.Bool))
	return solver.Check()
}
