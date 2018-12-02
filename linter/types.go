package linter

import (
	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/parser"
)

// Errors are not typed - they can fit any type
// Capabilities, sets, elements...
type TypeInfo interface {
	// A concise, human readable description of the type
	TypeName() string
}

// perhaps "Unknown would be better"
type AnyType struct {
}

func (*AnyType) TypeName() string {
	// should it be capitalized? What's the best convention for typename?
	return "Any"
}

type NullType struct {
}

func (*NullType) TypeName() string {
	return "Null"
}

type NumberType struct {
}

func (*NumberType) TypeName() string {
	return "Number"
}

type BooleanType struct {
}

func (*BooleanType) TypeName() string {
	return "Boolean"
}

type StringType struct {
}

func (*StringType) TypeName() string {
	return "String"
}

type ArrayType struct {
}

func (*ArrayType) TypeName() string {
	return "Array"
}

type KnownObjectType struct {
}

func (*KnownObjectType) TypeName() string {
	return "Object (known)"
}

type UnknownObjectType struct {
}

func (*UnknownObjectType) TypeName() string {
	return "Object (unknown)"
}

type exprTypes map[ast.Node]TypeInfo

type FunctionType struct {
}

func (*FunctionType) TypeName() string {
	return "Function"
}

func prepareSubexprTypes(node ast.Node, typeOf exprTypes) {
	for _, child := range parser.Children(node) {
		prepareTypes(child, typeOf)
	}
}

func prepareTypes(node ast.Node, typeOf exprTypes) {
	prepareSubexprTypes(node, typeOf)
	switch node := node.(type) {
	case *ast.Array:
		typeOf[node] = &ArrayType{}
	case *ast.Binary:
		// complicated
		typeOf[node] = &AnyType{}
	case *ast.Unary:
		// complicated
		typeOf[node] = &AnyType{}
	case *ast.Conditional:
		// complicated
		typeOf[node] = &AnyType{}
	case *ast.Var:
		// need to get expr of var
		// We may not know the type of the Var yet, for now, let's assume Any in such case
	case *ast.DesugaredObject:
		// TODO
		typeOf[node] = &AnyType{}
	case *ast.Error:
		typeOf[node] = &AnyType{}
	case *ast.Index:
		indexType := typeOf[node.Index]
		switch typeOf[node.Target].(type) {
		case *AnyType:
			// This is wrong, because we also want to allow Any...
			if _, ok := indexType.(*StringType); ok {
				// do nothing
			} else if _, ok := indexType.(*NumberType); ok {
				// do nothing
			}
		case *KnownObjectType:
			if _, ok := indexType.(*AnyType); ok {
				// check if it's a good index if the string is known
			} else if _, ok := indexType.(*StringType); ok {
				// check if it's a good index if the string is known
			} else {
				// ERROR
			}
		case *UnknownObjectType:
			if _, ok := indexType.(*AnyType); ok {
				// do nothing
			} else if _, ok := indexType.(*StringType); !ok {
				// ERROR
			}
		case *ArrayType:
			if _, ok := indexType.(*AnyType); ok {
				// do nothing
			} else if _, ok := indexType.(*NumberType); !ok {
				// ERROR
			}
		}
	case *ast.Import:
		// complicated
		typeOf[node] = &AnyType{}
	case *ast.LiteralBoolean:
		typeOf[node] = &BooleanType{}
	case *ast.LiteralNull:
		typeOf[node] = &NullType{}

	case *ast.LiteralNumber:
		typeOf[node] = &NumberType{}

	case *ast.LiteralString:
		typeOf[node] = &StringType{}

	case *ast.Local:
		typeOf[node] = typeOf[node.Body]
	case *ast.Self:
		// no recursion yet
		typeOf[node] = &UnknownObjectType{}
	case *ast.SuperIndex:
		typeOf[node] = &UnknownObjectType{}
	case *ast.InSuper:
		typeOf[node] = &BooleanType{}
	case *ast.Function:
		// TODO(sbarzowski) more fancy description of functions...
		typeOf[node] = &FunctionType{}
	case *ast.Apply:
		// Can't do anything, before we have a better description of function types
		typeOf[node] = &AnyType{}
	default:
		typeOf[node] = &AnyType{}

	}
}

// Open issues:
// What about recursion?
// What about polymorphic functions

// Ideas:
// Dispatch description
// Type predicates
//
// Primary goal - checking the correct use of the API
//
// Progressive narrowing of types of expressions
// Saving relationships of types of expressions
// Handling of knowledge about types/exprs
