package linter

import (
	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/linter/internal/common"
	"github.com/google/go-jsonnet/parser"
)

type vScope map[ast.Identifier]*common.Variable

func addVar(name ast.Identifier, loc ast.LocationRange, bindNode ast.Node, info *VariableInfo, scope vScope, varKind common.VariableKind) {
	v := &common.Variable{
		Name:         name,
		BindNode:     bindNode,
		Occurences:   nil,
		VariableKind: varKind,
		LocRange:     loc,
	}
	info.variables = append(info.variables, v)
	scope[name] = v
}

func cloneScope(oldScope vScope) vScope {
	new := make(vScope)
	for k, v := range oldScope {
		new[k] = v
	}
	return new
}

func findVariablesInFunc(node *ast.Function, info *VariableInfo, scope vScope) {
	// TODO(sbarzowski) right location range
	for _, param := range node.Parameters.Required {
		addVar(param, ast.LocationRange{}, nil, info, scope, common.VarParam)
	}
	for _, param := range node.Parameters.Optional {
		addVar(param.Name, ast.LocationRange{}, nil, info, scope, common.VarParam)
	}
	for _, param := range node.Parameters.Optional {
		findVariables(param.DefaultArg, info, scope)
	}
	findVariables(node.Body, info, scope)
}

func findVariablesInLocal(node *ast.Local, info *VariableInfo, scope vScope) {
	for _, bind := range node.Binds {
		addVar(bind.Variable, bind.LocRange, bind.Body, info, scope, common.VarRegular)
	}
	for _, bind := range node.Binds {
		if bind.Fun != nil {
			newScope := cloneScope(scope)
			findVariablesInFunc(bind.Fun, info, newScope)
		} else {
			findVariables(bind.Body, info, scope)
		}
	}
	findVariables(node.Body, info, scope)
}

func findVariablesInObject(node *ast.DesugaredObject, info *VariableInfo, scopeOutside vScope) {
	scopeInside := cloneScope(scopeOutside)
	// if scopeInside["$"] == nil {
	// 	addVar("$", node, node, info, scopeInside, common.VarDollarObject)
	// }
	for _, local := range node.Locals {
		addVar(local.Variable, local.LocRange, local.Body, info, scopeInside, common.VarRegular)
	}
	for _, local := range node.Locals {
		findVariables(local.Body, info, scopeInside)
	}
	for _, field := range node.Fields {
		findVariables(field.Body, info, scopeInside)
		findVariables(field.Name, info, scopeOutside)
	}
}

func findVariables(node ast.Node, info *VariableInfo, scope vScope) {
	switch node := node.(type) {
	case *ast.Function:
		newScope := cloneScope(scope)
		findVariablesInFunc(node, info, newScope)
	case *ast.Local:
		newScope := cloneScope(scope)
		findVariablesInLocal(node, info, newScope)
	case *ast.DesugaredObject:
		newScope := cloneScope(scope)
		findVariablesInObject(node, info, newScope)
	case *ast.Var:
		if v, ok := scope[node.Id]; ok {
			v.Occurences = append(v.Occurences, node)
		} else {
			panic("Undeclared variable " + string(node.Id) + " - it should be caught earlier")
		}

	default:
		for _, child := range parser.Children(node) {
			findVariables(child, info, scope)
		}
	}
}
