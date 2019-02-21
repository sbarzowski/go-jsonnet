package linter

import (
	"fmt"

	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/parser"
)

// Errors are not typed - they can fit any type
// Capabilities, sets, elements...
type TypeInfo interface {
	// A concise, human readable description of the type
	TypeName() string
}

type TypeDesc struct {
	Bool     bool
	Number   bool
	String   bool
	Null     bool
	Function bool // TODO(sbarzowski) better rep
	Object   bool // TODO(sbarzowski) better rep
	Array    bool // TODO(sbarzowski) better rep
}

type exprTypes map[ast.Node]*TypeDesc

func prepareSubexprTypes(node ast.Node, typeOf exprTypes) {
	for _, child := range parser.Children(node) {
		prepareTypes(child, typeOf)
	}
}

func AnyType() *TypeDesc {
	return &TypeDesc{
		Bool:     true,
		Number:   true,
		String:   true,
		Null:     true,
		Function: true,
		Object:   true,
		Array:    true,
	}
}

func VoidType() *TypeDesc {
	return &TypeDesc{}
}

func prepareTypes(node ast.Node, typeOf exprTypes) {
	prepareSubexprTypes(node, typeOf)
	switch node := node.(type) {
	case *ast.Array:
		typeOf[node] = &TypeDesc{Array: true}
	case *ast.Binary:
		// complicated
		typeOf[node] = AnyType()
	case *ast.Unary:
		// complicated
		typeOf[node] = AnyType()
	case *ast.Conditional:
		// complicated
		typeOf[node] = AnyType()
	case *ast.Var:
		// need to get expr of var
		// We may not know the type of the Var yet, for now, let's assume Any in such case
	case *ast.DesugaredObject:
		// TODO
		typeOf[node] = &TypeDesc{Object: true}
	case *ast.Error:
		typeOf[node] = VoidType()
	case *ast.Index:
		// indexType := typeOf[node.Index]
		// TODO
		typeOf[node] = AnyType()
	case *ast.Import:
		// complicated
		typeOf[node] = AnyType()
	case *ast.LiteralBoolean:
		typeOf[node] = &TypeDesc{Bool: true}
	case *ast.LiteralNull:
		typeOf[node] = &TypeDesc{Null: true}

	case *ast.LiteralNumber:
		typeOf[node] = &TypeDesc{Number: true}

	case *ast.LiteralString:
		typeOf[node] = &TypeDesc{String: true}

	case *ast.Local:
		typeOf[node] = typeOf[node.Body]
	case *ast.Self:
		// no recursion yet
		typeOf[node] = &TypeDesc{Object: true}
	case *ast.SuperIndex:
		typeOf[node] = &TypeDesc{Object: true}
	case *ast.InSuper:
		typeOf[node] = &TypeDesc{Bool: true}
	case *ast.Function:
		// TODO(sbarzowski) more fancy description of functions...
		typeOf[node] = &TypeDesc{Function: true}
	case *ast.Apply:
		// Can't do anything, before we have a better description of function types
		typeOf[node] = AnyType()
	default:
		typeOf[node] = AnyType()
		panic(fmt.Sprintf("Unexpected %t", node))

	}
}

func checkSubexpr(node ast.Node, typeOf exprTypes, ec *ErrCollector) {
	for _, child := range parser.Children(node) {
		check(child, typeOf, ec)
	}
}

type ErrCollector struct {
	errs []parser.StaticError
}

func (ec *ErrCollector) collect(err parser.StaticError) {
	ec.errs = append(ec.errs, err)
}

func (ec *ErrCollector) staticErr(msg string, loc *ast.LocationRange) {
	ec.collect(parser.MakeStaticError(msg, *loc))
}

func check(node ast.Node, typeOf exprTypes, ec *ErrCollector) {
	checkSubexpr(node, typeOf, ec)
	switch node := node.(type) {
	case *ast.Apply:
		if !typeOf[node.Target].Function {
			ec.staticErr("Called value is not a function", node.Loc())
		}
	case *ast.Index:
		targetType := typeOf[node.Target]
		indexType := typeOf[node.Index]
		if !targetType.Array && !targetType.Object {
			ec.staticErr("Indexed value is neither an array nor an object", node.Loc())
		} else if !targetType.Array {
			if !indexType.Number {
				ec.staticErr("Indexed value is assumed to be an array, but index is not an integer", node.Loc())
			}
		} else if !targetType.Object {
			if !indexType.String {
				ec.staticErr("Indexed value is assumed to be an object, but index is not a string", node.Loc())
			}
		} else {
			ec.staticErr("Index is neither an integer (for indexing arrays) nor a string (for indexing objects)", node.Loc())
		}
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
