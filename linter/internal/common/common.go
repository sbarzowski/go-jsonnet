package common

import (
	"github.com/google/go-jsonnet/ast"
)

type VariableKind int

const (
	VarRegular VariableKind = iota
	VarParam
	VarStdlib
	VarDollarObject
)

type Variable struct {
	Name         ast.Identifier
	BindNode     ast.Node
	Occurences   []ast.Node
	VariableKind VariableKind
	LocRange     ast.LocationRange
}
