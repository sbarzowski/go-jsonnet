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

// LintingInfo holds additional information about the program
// which was gathered during linting. The data should only be added to it.
// It is global, i.e. it holds the same data regardless of scope we're
// currently analyzing.
type LintingInfo struct {
	variables []*common.Variable
	varAt     map[ast.Node]*common.Variable // Variable information at every use site
}

// Lint analyses a node and reports any issues it encounters to an error writer.
func lint(node ast.Node, e *ErrorWriter) {
	lintingInfo := LintingInfo{
		variables: nil,
		varAt:     make(map[ast.Node]*common.Variable),
	}
	std := common.Variable{
		Name:         "std",
		Occurences:   nil,
		VariableKind: common.VarStdlib,
	}
	lintingInfo.variables = append(lintingInfo.variables, &std)
	findVariables(node, &lintingInfo, vScope{"std": &std})
	for _, v := range lintingInfo.variables {
		for _, u := range v.Occurences {
			lintingInfo.varAt[u] = v
		}
	}
	for _, v := range lintingInfo.variables {
		if len(v.Occurences) == 0 && v.VariableKind == common.VarRegular && v.Name != "$" {
			// TODO(sbarzowski) re-enable
			e.writeError(parser.MakeStaticError("Unused variable: "+string(v.Name), v.LocRange))
		}
	}
	et := make(types.ExprTypes)
	ec := utils.ErrCollector{}

	types.PrepareTypes(node, et, lintingInfo.varAt)
	types.Check(node, et, &ec)

	traversal.Traverse(node, &ec)

	for _, err := range ec.Errs {
		e.writeError(err)
	}
}

func getImports(node ast.Node) {
	switch node.(type) {
	case *ast.Import:
	case *ast.ImportStr:
	default:
		for _, c := range parser.Children(node) {
			getImports(c)
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
	lint(node, linter.errWriter)
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
