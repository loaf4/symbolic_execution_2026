// Package ssa предоставляет функции для построения SSA представления
package ssa

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"sort"
	"strings"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// Builder отвечает за построение SSA из исходного кода Go
type Builder struct {
	fset *token.FileSet
}

// NewBuilder создаёт новый экземпляр Builder
func NewBuilder() *Builder {
	return &Builder{
		fset: token.NewFileSet(),
	}
}

func (b *Builder) FileSet() *token.FileSet {
	return b.fset
}

func (b *Builder) BuildPackage(source string) (*ssa.Package, error) {
	file, err := parser.ParseFile(b.fset, "input.go", source, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	pkg := types.NewPackage(file.Name.Name, file.Name.Name)
	conf := &types.Config{Importer: importer.Default()}
	mode := ssa.SanityCheckFunctions | ssa.InstantiateGenerics
	ssaPkg, _, err := ssautil.BuildPackage(conf, b.fset, pkg, []*ast.File{file}, mode)
	if err != nil {
		return nil, fmt.Errorf("SSA build error: %w", err)
	}
	return ssaPkg, nil
}

// returns function SSA
func (b *Builder) ParseAndBuildSSA(source string, funcName string) (*ssa.Function, error) {
	ssaPkg, err := b.BuildPackage(source)
	if err != nil {
		return nil, err
	}

	fn := ssaPkg.Func(funcName)
	if fn == nil {
		return nil, fmt.Errorf("function %q not found in package %s", funcName, ssaPkg.Pkg.Name())
	}
	return fn, nil
}

func FunctionNames(pkg *ssa.Package) []string {
	var names []string
	for name, member := range pkg.Members {
		if _, ok := member.(*ssa.Function); ok && name != "init" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func PrintFunction(w io.Writer, fn *ssa.Function) {
	fmt.Fprintf(w, "Function: %s\n", fn.Name())
	fmt.Fprintf(w, "Signature: %s\n", fn.Signature)
	if len(fn.Params) > 0 {
		params := make([]string, len(fn.Params))
		for i, p := range fn.Params {
			params[i] = fmt.Sprintf("%s %s", p.Name(), p.Type())
		}
		fmt.Fprintf(w, "Params: %s\n", strings.Join(params, ", "))
	}
	fmt.Fprintf(w, "Basic blocks: %d\n", len(fn.Blocks))

	for _, block := range fn.Blocks {
		fmt.Fprintf(w, "\n  Block %d (%s)\n", block.Index, block.Comment)
		fmt.Fprintf(w, "    preds: %s\n", blockList(block.Preds))
		fmt.Fprintf(w, "    succs: %s\n", blockList(block.Succs))
		if block.Idom() != nil {
			fmt.Fprintf(w, "    idom:  %d\n", block.Idom().Index)
		}
		fmt.Fprintf(w, "    instructions (%d):\n", len(block.Instrs))
		for _, instr := range block.Instrs {
			fmt.Fprintf(w, "      %-12s %s\n", instrKind(instr), formatInstr(instr))
		}
	}

	for _, anon := range fn.AnonFuncs {
		fmt.Fprintln(w)
		PrintFunction(w, anon)
	}
}

func blockList(blocks []*ssa.BasicBlock) string {
	if len(blocks) == 0 {
		return "-"
	}
	idx := make([]string, len(blocks))
	for i, b := range blocks {
		idx[i] = fmt.Sprint(b.Index)
	}
	return strings.Join(idx, ", ")
}

func formatInstr(instr ssa.Instruction) string {
	if v, ok := instr.(ssa.Value); ok && v.Name() != "" {
		return fmt.Sprintf("%s = %s : %s", v.Name(), instr.String(), v.Type())
	}
	return instr.String()
}

func instrKind(instr ssa.Instruction) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", instr), "*ssa.")
}
