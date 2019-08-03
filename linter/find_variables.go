package linter

import (
	"fmt"

	"github.com/davecgh/go-spew/spew"

	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/parser"
)

type vScope map[ast.Identifier]*variable

func addVar(name ast.Identifier, node ast.Node, info *LintingInfo, scope vScope, param bool) {
	v := &variable{
		name:     name,
		declNode: node,
		uses:     nil,
		param:    param,
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

func findVariablesInFunc(node *ast.Function, info *LintingInfo, scope vScope) {
	for _, param := range node.Parameters.Required {
		addVar(param, node, info, scope, true)
	}
	for _, param := range node.Parameters.Optional {
		addVar(param.Name, node, info, scope, true)
	}
	for _, param := range node.Parameters.Optional {
		findVariables(param.DefaultArg, info, scope)
	}
	findVariables(node.Body, info, scope)
}

func findVariablesInLocal(node *ast.Local, info *LintingInfo, scope vScope) {
	fmt.Println("FOOOO")
	for _, bind := range node.Binds {
		fmt.Printf("Bar %s\n", bind.Variable)
		addVar(bind.Variable, node, info, scope, false)
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

func findVariables(node ast.Node, info *LintingInfo, scope vScope) {
	switch node := node.(type) {
	case *ast.Function:
		newScope := cloneScope(scope)
		findVariablesInFunc(node, info, newScope)
	case *ast.Local:
		newScope := cloneScope(scope)
		findVariablesInLocal(node, info, newScope)
	case *ast.Var:
		if v, ok := scope[node.Id]; ok {
			fmt.Printf("USE %s\n", node.Id)
			v.uses = append(v.uses, node)
			spew.Dump(v)
		} else {
			panic("Undeclared variable " + string(node.Id) + " - it should be caught earlier")
		}

	default:
		for _, child := range parser.Children(node) {
			findVariables(child, info, scope)
		}
	}
}
