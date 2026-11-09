package memory

import (
	"testing"

	"github.com/ebukreev/go-z3/z3"
	"symbolic-execution-course/internal/symbolic"
	"symbolic-execution-course/internal/translator"
)

func isSat(t *testing.T, zt *translator.Z3Translator, conditions ...symbolic.SymbolicExpression) (bool, *z3.Model) {
	solver := z3.NewSolver(zt.GetContext().(*z3.Context))
	for _, cond := range conditions {
		z3Cond, err := zt.TranslateExpression(cond)
		if err != nil {
			t.Fatalf("translation failed: %v", err)
		}
		solver.Assert(z3Cond.(z3.Bool))
	}
	sat, err := solver.Check()
	if err != nil {
		t.Fatalf("solver failed: %v", err)
	}
	if !sat {
		return false, nil
	}
	return true, solver.Model()
}

func eq(a, b symbolic.SymbolicExpression) symbolic.SymbolicExpression {
	return symbolic.NewBinaryOperation(a, b, symbolic.EQ)
}

func ne(a, b symbolic.SymbolicExpression) symbolic.SymbolicExpression {
	return symbolic.NewBinaryOperation(a, b, symbolic.NE)
}

func TestFieldReadAfterWrite(t *testing.T) {
	zt := translator.NewZ3Translator()
	mem := NewSymbolicMemory()

	p := mem.Allocate(symbolic.IntType)
	mem.AssignField(p, 1, symbolic.NewIntConstant(25))
	age := mem.GetFieldValue(p, 1, symbolic.IntType)

	if sat, _ := isSat(t, zt, ne(age, symbolic.NewIntConstant(25))); sat {
		t.Errorf("expected field value 25, got %s", age)
	}
}

func TestDifferentObjects(t *testing.T) {
	zt := translator.NewZ3Translator()
	mem := NewSymbolicMemory()

	a := mem.Allocate(symbolic.IntType)
	b := mem.Allocate(symbolic.IntType)
	mem.AssignField(a, 0, symbolic.NewIntConstant(1))
	mem.AssignField(b, 0, symbolic.NewIntConstant(2))

	if sat, _ := isSat(t, zt, ne(mem.GetFieldValue(a, 0, symbolic.IntType), symbolic.NewIntConstant(1))); sat {
		t.Error("write to b changed a")
	}
	if sat, _ := isSat(t, zt, ne(mem.GetFieldValue(b, 0, symbolic.IntType), symbolic.NewIntConstant(2))); sat {
		t.Error("expected b.f == 2")
	}
}

func TestArray(t *testing.T) {
	zt := translator.NewZ3Translator()
	mem := NewSymbolicMemory()

	array := mem.Allocate(symbolic.ArrayType)
	mem.AssignToArray(array, 5, symbolic.NewIntConstant(10))

	fromArray := mem.GetFromArray(array, 5, symbolic.IntType)
	if sat, _ := isSat(t, zt, ne(fromArray, symbolic.NewIntConstant(10))); sat {
		t.Errorf("expected array[5] == 10, got %s", fromArray)
	}

	another := mem.GetFromArray(array, 10, symbolic.IntType)
	if sat, _ := isSat(t, zt, eq(another, symbolic.NewIntConstant(10))); !sat {
		t.Error("array[10] can be 10")
	}
	if sat, _ := isSat(t, zt, ne(another, symbolic.NewIntConstant(10))); !sat {
		t.Error("array[10] can be not 10")
	}
}

func TestRefField(t *testing.T) {
	zt := translator.NewZ3Translator()
	mem := NewSymbolicMemory()

	emp := mem.Allocate(symbolic.IntType)
	p := mem.Allocate(symbolic.IntType)
	mem.AssignField(emp, 0, p)

	person := mem.GetFieldValue(emp, 0, symbolic.RefType).(*symbolic.Ref)
	mem.AssignField(person, 1, symbolic.NewIntConstant(30))

	age := mem.GetFieldValue(p, 1, symbolic.IntType)
	if sat, _ := isSat(t, zt, ne(age, symbolic.NewIntConstant(30))); sat {
		t.Errorf("expected p.age == 30, got %s", age)
	}
}

func TestAliasing(t *testing.T) {
	zt := translator.NewZ3Translator()
	mem := NewSymbolicMemory()

	foo1 := symbolic.NewSymbolicRef("foo1")
	foo2 := symbolic.NewSymbolicRef("foo2")
	mem.AssignField(foo2, 0, symbolic.NewIntConstant(5))
	mem.AssignField(foo1, 0, symbolic.NewIntConstant(2))
	foo2a := mem.GetFieldValue(foo2, 0, symbolic.IntType)
	cond := eq(foo2a, symbolic.NewIntConstant(2))

	sat, model := isSat(t, zt, cond)
	if !sat {
		t.Fatal("expected foo2.a == 2 to be reachable")
	}
	z3Foo1, _ := zt.TranslateExpression(foo1)
	z3Foo2, _ := zt.TranslateExpression(foo2)
	addr1 := model.Eval(z3Foo1.(z3.Value), true).String()
	addr2 := model.Eval(z3Foo2.(z3.Value), true).String()
	if addr1 != addr2 {
		t.Errorf("expected foo1 == foo2 in model, got %s and %s", addr1, addr2)
	}

	if sat, _ := isSat(t, zt, cond, ne(foo1, foo2)); sat {
		t.Error("foo2.a == 2 must be impossible when foo1 != foo2")
	}

	if sat, _ := isSat(t, zt, ne(foo2a, symbolic.NewIntConstant(2)), ne(foo1, foo2)); !sat {
		t.Error("expected foo2.a != 2 to be reachable when foo1 != foo2")
	}
}
