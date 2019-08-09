// Package linter analyses Jsonnet code for code "smells".
package linter

import (
	"io"

	jsonnet "github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/parser"

	"github.com/google/go-jsonnet/linter/internal/common"
	"github.com/google/go-jsonnet/linter/internal/types"
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
func Lint(node ast.Node, e *ErrorWriter) {
	lintingInfo := LintingInfo{
		variables: nil,
		varAt:     make(map[ast.Node]*common.Variable),
	}
	std := common.Variable{
		Name:       "std",
		DeclNode:   nil,
		Occurences: nil,
		Param:      false,
		Stdlib:     true,
	}
	lintingInfo.variables = append(lintingInfo.variables, &std)
	findVariables(node, &lintingInfo, vScope{"std": &std})
	for _, v := range lintingInfo.variables {
		for _, u := range v.Occurences {
			lintingInfo.varAt[u] = v
		}
	}
	for _, v := range lintingInfo.variables {
		if len(v.Occurences) == 0 && !v.Param && !v.Stdlib && v.Name != "$" {
			e.writeError(parser.MakeStaticError("Unused variable: "+string(v.Name), *v.DeclNode.Loc()))
		}
	}
	et := make(types.ExprTypes)
	ec := types.ErrCollector{}

	types.PrepareTypes(node, et, lintingInfo.varAt)
	types.Check(node, et, &ec)
	for _, err := range ec.Errs {
		e.writeError(err)
	}
}

func RunLint(filename string, code string, errWriter *ErrorWriter) bool {
	node, err := jsonnet.SnippetToAST(filename, code)
	if err != nil {
		errWriter.writeError(err.(parser.StaticError)) // ugly but true
		return true
	}
	Lint(node, errWriter)
	return errWriter.ErrorsFound
}
