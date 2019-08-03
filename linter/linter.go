// Package linter analyses Jsonnet code for code "smells".
package linter

import (
	"io"

	"github.com/davecgh/go-spew/spew"

	jsonnet "github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/parser"
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

type variable struct {
	name     ast.Identifier
	declNode ast.Node
	uses     []ast.Node
	param    bool // TODO enum
}

// LintingInfo holds additional information about the program
// which was gathered during linting. The data should only be added to it.
// It is global, i.e. it holds the same data regardless of scope we're
// currently analyzing.
type LintingInfo struct {
	variables []*variable
}

// Lint analyses a node and reports any issues it encounters to an error writer.
func Lint(node ast.Node, e *ErrorWriter) {
	lintingInfo := LintingInfo{
		variables: nil,
	}
	std := variable{
		name:     "std",
		declNode: nil,
		uses:     nil,
		param:    false,
	}
	findVariables(node, &lintingInfo, vScope{"std": &std})
	for _, v := range lintingInfo.variables {
		spew.Dump(v.uses)
		if len(v.uses) == 0 && !v.param {
			e.writeError(parser.MakeStaticError("Unused variable: "+string(v.name), *v.declNode.Loc()))
		}
	}
	et := make(exprTypes)
	ec := ErrCollector{}
	prepareTypesWithGraph(node, et)
	check(node, et, &ec)
	for _, err := range ec.errs {
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
