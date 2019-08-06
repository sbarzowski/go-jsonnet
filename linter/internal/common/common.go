package common

import (
	"github.com/google/go-jsonnet/ast"
)

type Variable struct {
	Name       ast.Identifier
	DeclNode   ast.Node
	BindNode   ast.Node
	Occurences []ast.Node
	Param      bool // TODO enum
	Stdlib     bool
}
