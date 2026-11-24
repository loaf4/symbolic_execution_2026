package memory

import (
	"fmt"

	"symbolic-execution-course/internal/symbolic"
)

type Memory interface {
	Allocate(tpe symbolic.ExpressionType) *symbolic.Ref

	AssignField(ref *symbolic.Ref, fieldIdx int, value symbolic.SymbolicExpression)

	GetFieldValue(ref *symbolic.Ref, fieldIdx int, tpe symbolic.ExpressionType) symbolic.SymbolicExpression

	AssignToArray(ref *symbolic.Ref, index symbolic.SymbolicExpression, value symbolic.SymbolicExpression)

	GetFromArray(ref *symbolic.Ref, index symbolic.SymbolicExpression, tpe symbolic.ExpressionType) symbolic.SymbolicExpression
}

type SymbolicMemory struct {
	nextAddress int64
	arrays      map[string]symbolic.SymbolicExpression
}

func NewSymbolicMemory() SymbolicMemory {
	return SymbolicMemory{
		nextAddress: 1, // 0 зарезервирован под nil
		arrays:      make(map[string]symbolic.SymbolicExpression),
	}
}

func (mem *SymbolicMemory) Allocate(tpe symbolic.ExpressionType) *symbolic.Ref {
	ref := symbolic.NewRef(symbolic.NewIntConstant(mem.nextAddress))
	mem.nextAddress++
	return ref
}

func (mem *SymbolicMemory) AssignField(ref *symbolic.Ref, fieldIdx int, value symbolic.SymbolicExpression) {
	name := fmt.Sprintf("field%d_%s", fieldIdx, value.Type())
	mem.write(name, ref, value)
}

func (mem *SymbolicMemory) GetFieldValue(ref *symbolic.Ref, fieldIdx int, tpe symbolic.ExpressionType) symbolic.SymbolicExpression {
	name := fmt.Sprintf("field%d_%s", fieldIdx, tpe)
	return mem.read(name, ref, tpe)
}

func (mem *SymbolicMemory) AssignToArray(ref *symbolic.Ref, index symbolic.SymbolicExpression, value symbolic.SymbolicExpression) {
	name := fmt.Sprintf("arrays_%s", value.Type())
	arrays := mem.getArrayOfArrays(name, value.Type())

	// inner = arrays[addr]; inner[index] = value; arrays[addr] = inner
	inner := symbolic.NewArraySelect(arrays, ref.Address)
	newInner := symbolic.NewArrayStore(inner, index, value)
	mem.arrays[name] = symbolic.NewArrayStore(arrays, ref.Address, newInner)
}

func (mem *SymbolicMemory) GetFromArray(ref *symbolic.Ref, index symbolic.SymbolicExpression, tpe symbolic.ExpressionType) symbolic.SymbolicExpression {
	name := fmt.Sprintf("arrays_%s", tpe)
	arrays := mem.getArrayOfArrays(name, tpe)

	// arrays[addr][index]
	inner := symbolic.NewArraySelect(arrays, ref.Address)
	value := symbolic.NewArraySelect(inner, index)
	if tpe == symbolic.RefType {
		return symbolic.NewRef(value)
	}
	return value
}

func (mem *SymbolicMemory) getArray(name string, elemType symbolic.ExpressionType) symbolic.SymbolicExpression {
	array, ok := mem.arrays[name]
	if !ok {
		array = symbolic.NewSymbolicArray(name, elemType)
		mem.arrays[name] = array
	}
	return array
}

func (mem *SymbolicMemory) getArrayOfArrays(name string, elemType symbolic.ExpressionType) symbolic.SymbolicExpression {
	array, ok := mem.arrays[name]
	if !ok {
		array = symbolic.NewArrayOfArrays(name, elemType)
		mem.arrays[name] = array
	}
	return array
}

func (mem *SymbolicMemory) write(name string, ref *symbolic.Ref, value symbolic.SymbolicExpression) {
	array := mem.getArray(name, value.Type())
	mem.arrays[name] = symbolic.NewArrayStore(array, ref.Address, value)
}

func (mem *SymbolicMemory) read(name string, ref *symbolic.Ref, tpe symbolic.ExpressionType) symbolic.SymbolicExpression {
	array := mem.getArray(name, tpe)
	value := symbolic.NewArraySelect(array, ref.Address)

	if tpe == symbolic.RefType {
		return symbolic.NewRef(value)
	}
	return value
}
