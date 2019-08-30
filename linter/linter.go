// Package linter analyses Jsonnet code for code "smells".
package linter

import (
	"io"

	jsonnet "github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/parser"

	"github.com/google/go-jsonnet/linter/internal/common"
	"github.com/google/go-jsonnet/linter/internal/traversal"
	"github.com/google/go-jsonnet/linter/internal/types"
	"github.com/google/go-jsonnet/linter/internal/utils"
)

// TODO(sbarzowski) refactoring idea - put ast.Node and its path in a struct as it gets
// passed around together constantly

// ErrorWriter encapsulates a writer and an error state indicating when at least
// one error has been written to the writer.
type ErrorWriter struct {
	ErrorsFound bool
	Writer      io.Writer
	Formatter   jsonnet.ErrorFormatter
}

func (e *ErrorWriter) writeError(err parser.StaticError) {
	e.ErrorsFound = true
	e.Writer.Write([]byte(e.Formatter.Format(err) + "\n"))
}

// VariableInfo holds information about a variables from one file
type VariableInfo struct {
	variables []*common.Variable
	varAt     map[ast.Node]*common.Variable // Variable information at every use site
}

// Lint analyses a node and reports any issues it encounters to an error writer.
func lint(vm *jsonnet.VM, node ast.Node, currentPath string, errWriter *ErrorWriter) {
	roots := make(map[string]ast.Node)
	getImports(vm, node, roots, currentPath, errWriter)

	variablesInFile := make(map[string]VariableInfo)

	std := common.Variable{
		Name:         "std",
		Occurences:   nil,
		VariableKind: common.VarStdlib,
	}

	findVariables := func(node ast.Node, currentPath string) VariableInfo {
		variableInfo := VariableInfo{
			variables: nil,
			varAt:     make(map[ast.Node]*common.Variable),
		}
		variableInfo.variables = append(variableInfo.variables, &std)
		findVariables(node, &variableInfo, vScope{"std": &std})
		for _, v := range variableInfo.variables {
			for _, u := range v.Occurences {
				variableInfo.varAt[u] = v
			}
		}
		return variableInfo
	}

	variableInfo := findVariables(node, currentPath)
	for importedPath, rootNode := range roots {
		variablesInFile[importedPath] = findVariables(rootNode, importedPath)
	}

	for _, v := range variableInfo.variables {
		if len(v.Occurences) == 0 && v.VariableKind == common.VarRegular && v.Name != "$" {
			// TODO(sbarzowski) re-enable
			errWriter.writeError(parser.MakeStaticError("Unused variable: "+string(v.Name), v.LocRange))
		}
	}
	et := make(types.ExprTypes)
	ec := utils.ErrCollector{}

	g := types.NewTypeGraph(variableInfo.varAt)

	for importedPath, rootNode := range roots {
		g.AddToGraph(rootNode, variablesInFile[importedPath].varAt)
	}

	g.PrepareTypes(node, et, variableInfo.varAt)
	types.Check(node, et, &ec)

	traversal.Traverse(node, &ec)

	for _, err := range ec.Errs {
		errWriter.writeError(err)
	}
}

func getImports(vm *jsonnet.VM, node ast.Node, roots map[string]ast.Node, currentPath string, errWriter *ErrorWriter) {
	// TODO(sbarzowski) consider providing some way to disable warnings about nonexistent imports
	// At least for 3rd party code.
	// Perhaps there may be some valid use cases for conditional imports where one of the imported
	// files doesn't exist.
	switch node := node.(type) {
	case *ast.Import:
		p := node.File.Value
		contents, foundAt, err := vm.ImportAST(currentPath, p)
		if err != nil {
			errWriter.writeError(parser.MakeStaticError(err.Error(), *node.Loc()))
		} else {
			roots[foundAt] = contents
		}
	case *ast.ImportStr:
		p := node.File.Value
		_, err := vm.ResolveImport(currentPath, p)
		if err != nil {
			errWriter.writeError(parser.MakeStaticError(err.Error(), *node.Loc()))
		}
	default:
		for _, c := range parser.Children(node) {
			getImports(vm, c, roots, currentPath, errWriter)
		}
	}
}

type Linter struct {
	vm *jsonnet.VM
	// TODO(sbarzowski) implement observer in VM, so that linter cache is automatically flushed when
	// the sources are invalidated. This is unnecessary if we implement one-go linter.
	errWriter *ErrorWriter

	// TODO(sbarzowski) allow more files
	filename string
	code     string
}

func (linter *Linter) flushCache() {

}

func (linter *Linter) AddFile(filename, code string) {
	// TODO(sbarzowski) create addImported - a variant which imports it instead of getting code as string
	// This allows avoiding processing it twice if it's both imported and accessed directly
	// This is more important for linter than for interpretation, since for linter every file that
	// the user created may be directly linted. And normally for execution there are only some set entry points.

	// TODO(sbarzowski) do this properly
	linter.filename = filename
	linter.code = code
}

func (linter *Linter) Check() bool {
	node, err := jsonnet.SnippetToAST(linter.filename, linter.code)
	if err != nil {
		linter.errWriter.writeError(err.(parser.StaticError)) // ugly but true
		return true
	}
	lint(linter.vm, node, linter.filename, linter.errWriter)
	// TODO(sbarzowski) There is something fishy about this usage errWriter, since it may be reused multiple times
	return linter.errWriter.ErrorsFound
}

func NewLinter(vm *jsonnet.VM, errWriter *ErrorWriter) *Linter {
	return &Linter{
		vm:        vm,
		errWriter: errWriter,
	}
}

// func RunLint(vm *jsonnet.VM, filename string, code string, errWriter *ErrorWriter) bool {
// 	node, err := jsonnet.SnippetToAST(filename, code)
// 	if err != nil {
// 		errWriter.writeError(err.(parser.StaticError)) // ugly but true
// 		return true
// 	}
// 	lint(node, errWriter)
// 	return errWriter.ErrorsFound
// }
