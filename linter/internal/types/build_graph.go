package types

import (
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/linter/internal/common"
	"github.com/google/go-jsonnet/parser"
)

const maxKnownCount = 5

func (g *typeGraph) getExprPlaceholder(node ast.Node) placeholderID {
	if g.exprPlaceholder[node] == noType {
		fmt.Fprintf(os.Stderr, "------------------------------------------------------------------\n")
		spew.Dump(node)
		panic("Bug - placeholder for a dependent node cannot be noType")
		// It will be possible in later stages, after some simplifications
		// but for now (i.e. during generation) it means that something was not initialized
		// at the appropriate time.
	}
	return g.exprPlaceholder[node]
}

func prepareTPWithPlaceholder(node ast.Node, g *typeGraph, p placeholderID) {
	if node == nil {
		panic("Node cannot be nil")
	}
	switch node := node.(type) {
	case *ast.Local:
		bindPlaceholders := make([]placeholderID, len(node.Binds))
		for i := range node.Binds {
			bindPlaceholders[i] = g.newPlaceholder()
			g.exprPlaceholder[node.Binds[i].Body] = bindPlaceholders[i]
		}
		for i := range node.Binds {
			// TODO(sbarzowski) what about func? is desugaring allowing us to avoid that
			prepareTPWithPlaceholder(node.Binds[i].Body, g, bindPlaceholders[i])
		}
		prepareTP(node.Body, g)
	case *ast.DesugaredObject:
		localPlaceholders := make([]placeholderID, len(node.Locals))
		for i := range node.Locals {
			localPlaceholders[i] = g.newPlaceholder()
			g.exprPlaceholder[node.Locals[i].Body] = localPlaceholders[i]
		}
		for i := range node.Locals {
			// TODO(sbarzowski) what about func? is desugaring allowing us to avoid that
			prepareTPWithPlaceholder(node.Locals[i].Body, g, localPlaceholders[i])
		}
		for i := range node.Fields {
			prepareTP(node.Fields[i].Name, g)
			prepareTP(node.Fields[i].Body, g)
		}
	default:
		for _, child := range parser.Children(node) {
			if child == nil {
				panic("Bug - child cannot be nil")
			}
			prepareTP(child, g)
		}
	}
	*(g.placeholder(p)) = calcTP(node, g)
}

func prepareTP(node ast.Node, g *typeGraph) {
	if node == nil {
		panic("Node cannot be nil")
	}
	p := g.newPlaceholder()
	g.exprPlaceholder[node] = p
	prepareTPWithPlaceholder(node, g, p)
}

func calcTP(node ast.Node, g *typeGraph) typePlaceholder {
	switch node := node.(type) {
	case *ast.Array:
		knownCount := len(node.Elements)
		if knownCount > maxKnownCount {
			knownCount = maxKnownCount
		}

		desc := &arrayDesc{
			allContain:      make([]placeholderID, 0, len(node.Elements)-knownCount),
			elementContains: make([][]placeholderID, knownCount, maxKnownCount),
		}

		for i, el := range node.Elements {
			if i < knownCount {
				desc.elementContains[i] = []placeholderID{g.getExprPlaceholder(el)}
			} else {
				desc.allContain = append(desc.allContain, g.getExprPlaceholder(el))
			}
		}

		return concreteTP(TypeDesc{ArrayDesc: desc})
	case *ast.Binary:
		// complicated
		return tpRef(anyType)
	case *ast.Unary:
		switch node.Op {
		case ast.UopNot:
			return tpRef(boolType)
		case ast.UopBitwiseNot, ast.UopPlus, ast.UopMinus:
			return tpRef(numberType)
		default:
			panic(fmt.Sprintf("Unrecognized unary operator %v", node.Op))
		}
	case *ast.Conditional:
		return tpSum(g.getExprPlaceholder(node.BranchTrue), g.getExprPlaceholder(node.BranchFalse))
	case *ast.Var:
		v := g.varAt[node]
		if v == nil {
			panic("Could not find variable")
		}
		switch v.VariableKind {
		case common.VarStdlib:
			return concreteTP(TypeDesc{ObjectDesc: &objectDesc{
				allFieldsKnown: false,
				allContain:     []placeholderID{anyType},
			}})
		case common.VarParam:
			return tpRef(anyType)

		case common.VarDollarObject:
			panic("Not implemented yet")
			// TODO(sbarzowski) do we really want to treat it differently from regular Var? Perhaps it would be better to just assign it when the object appears
		case common.VarRegular:

			return tpRef(g.getExprPlaceholder(v.BindNode))
		}

	case *ast.DesugaredObject:
		// TODO
		obj := &objectDesc{
			allFieldsKnown: true,
			fieldContains:  make(map[string][]placeholderID),
		}
		for _, field := range node.Fields {
			// TODO(sbarzowski) what about plussuper, how does it change things here?
			switch fieldName := field.Name.(type) {
			case *ast.LiteralString:
				obj.fieldContains[fieldName.Value] = append(obj.fieldContains[fieldName.Value], g.getExprPlaceholder(field.Body))
			default:
				obj.allContain = append(obj.allContain, g.getExprPlaceholder(field.Body))
				obj.allFieldsKnown = false
			}
		}
		return concreteTP(TypeDesc{ObjectDesc: obj})
	case *ast.Error:
		return concreteTP(voidTypeDesc())
	case *ast.Index:
		switch index := node.Index.(type) {
		case *ast.LiteralString:
			return tpIndex(knownObjectIndex(g.getExprPlaceholder(node.Target), index.Value))
		case *ast.LiteralNumber:
			valFloat := index.Value
			if valFloat >= 0 && valFloat < maxKnownCount && valFloat == float64(int64(valFloat)) {
				return tpIndex(arrayIndex(g.getExprPlaceholder(node.Target), int(valFloat)))
			}
		}
		return tpIndex(unknownIndexSpec(g.getExprPlaceholder(node.Target)))
	case *ast.Import:
		// complicated
		return tpRef(anyType)
	case *ast.ImportStr:
		// complicated
		return tpRef(stringType)
	case *ast.LiteralBoolean:
		return tpRef(boolType)
	case *ast.LiteralNull:
		return tpRef(nullType)

	case *ast.LiteralNumber:
		return tpRef(numberType)

	case *ast.LiteralString:
		return tpRef(stringType)

	case *ast.Local:
		// TODO(sbarzowski) perhaps it should return the id and any creation of the new placeholders would happend in this function
		// then we would be able to avoid unnecessary indirection
		return tpRef(g.getExprPlaceholder(node.Body))
	case *ast.Self:
		// no recursion yet
		return tpRef(anyObjectType)
	case *ast.SuperIndex:
		return tpRef(anyObjectType)
	case *ast.InSuper:
		return tpRef(boolType)
	case *ast.Function:
		// TODO(sbarzowski) more fancy description of functions...
		return concreteTP(TypeDesc{FunctionDesc: &functionDesc{
			minArity:       len(node.Parameters.Required),
			maxArity:       len(node.Parameters.Required) + len(node.Parameters.Optional),
			params:         &node.Parameters,
			resultContains: []placeholderID{g.getExprPlaceholder(node.Body)},
		}})
	case *ast.Apply:
		return tpIndex(functionCallIndex(g.getExprPlaceholder(node.Target)))
	}
	panic(fmt.Sprintf("Unexpected %#v", node))
}
