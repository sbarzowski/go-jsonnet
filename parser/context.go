/*
Copyright 2017 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package parser

import (
	"fmt"

	"github.com/google/go-jsonnet/ast"
)

const anonymous = "anonymous"

func functionContext(funcName string) *string {
	r := "function <" + funcName + ">"
	return &r
}

func objectContext(objName string) *string {
	r := "object <" + objName + ">"
	return &r
}

func arrayContext() *string {
	r := "array_element"
	return &r
}

func directChildren(node ast.Node) []ast.Node {
	switch node := node.(type) {
	case *ast.Binary:
		return nil
	case *ast.Conditional:
		return []ast.Node{node.Cond, node.BranchTrue, node.BranchFalse}
	case *ast.Dollar:
		return nil
	case *ast.Error:
		return nil
	case *ast.Function:
		return nil
	case *ast.Import:
		return nil
	case *ast.Index:
		return []ast.Node{node.Target, node.Index}
	case *ast.Slice:
		return []ast.Node{node.Target, node.BeginIndex, node.EndIndex, node.Step}
	case *ast.Local:
		return []ast.Node{node.Body}
	case *ast.LiteralBoolean:
		return nil
	case *ast.LiteralNull:
		return nil
	case *ast.LiteralNumber:
		return nil
	case *ast.LiteralString:
		return nil
	case *ast.Object:
		return nil
	case *ast.ObjectComp:
		return nil
	case *ast.Self:
		return nil
	case *ast.SuperIndex:
		return []ast.Node{node.Index}
	case *ast.InSuper:
		return []ast.Node{node.Index}
	case *ast.Unary:
		return nil
	}
	panic(fmt.Sprintf("Unknown node %#v", node))
}

func thunkChildren(node ast.Node) []ast.Node {
	switch node := node.(type) {
	case *ast.Binary:
		return []ast.Node{node.Left, node.Right}
	case *ast.Conditional:
		return nil
	case *ast.Dollar:
		return nil
	case *ast.Error:
		return []ast.Node{node.Expr}
	case *ast.Function:
		// TODO(sbarzowski) what to do with default args
		return nil
	case *ast.Import:
		return nil
	case *ast.Index:
		return nil
	case *ast.Slice:
		return nil
	case *ast.Local:
		// TODO(sbarzowski) complicated
	case *ast.LiteralBoolean:
		return nil
	case *ast.LiteralNull:
		return nil
	case *ast.LiteralNumber:
		return nil
	case *ast.LiteralString:
		return nil
	case *ast.Object:
		// TODO(sbarzowski) complicated
		return nil
	case *ast.ObjectComp:
		// TODO(sbarzowski) complicated
		return nil
	case *ast.Self:
		return nil
	case *ast.SuperIndex:
		return nil
	case *ast.InSuper:
		return nil
	case *ast.Unary:
		return []ast.Node{node.Expr}
	}
	panic(fmt.Sprintf("Unknown node %#v", node))
}

// func specialChildren(node ast.Node) []ast.Node {

// }

func addContext(node ast.Node, context *string, bind string) {
	node.SetContext(context)

	switch node := node.(type) {
	case *ast.Function:
		funContext := functionContext(bind)
		addContext(node.Body, funContext, anonymous)
		for i := range node.Parameters.Named {
			// TODO(sbarzowski) what should the context of a default argument be?
			addContext(node.Parameters.Named[i].DefaultArg, context, anonymous)
		}
	case *ast.Object:
		objContext := objectContext(bind)
		for i := range node.Fields {
			field := &node.Fields[i]
			if field.Expr1 != nil {
				// This actually is evaluated outside of object
				addContext(field.Expr1, context, anonymous)
			}
			if field.Expr2 != nil {
				addContext(field.Expr2, objContext, anonymous)
			}
			if field.Expr3 != nil {
				addContext(field.Expr3, objContext, anonymous)
			}
			if field.MethodSugar {
				for i := range field.Params.Named {
					addContext(field.Params.Named[i].DefaultArg, context, anonymous)
				}
			}
		}

	case *ast.Array:
		arrContext := arrayContext()
		for i := range node.Elements {
			addContext(node.Elements[i], arrContext, anonymous)
		}
	case *ast.ArrayComp:
	case *ast.ObjectComp:

	case *ast.Apply:
	case *ast.Local:

	default:

	}
}
