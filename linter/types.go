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

func calcType(node ast.Node, typeOf exprTypes) *TypeDesc {
	switch node := node.(type) {
	case *ast.Array:
		return &TypeDesc{Array: true}
	case *ast.Binary:
		// complicated
		return AnyType()
	case *ast.Unary:
		// complicated
		return AnyType()
	case *ast.Conditional:
		// complicated
		return AnyType()
	case *ast.Var:
		// need to get expr of var
		// We may not know the type of the Var yet, for now, let's assume Any in such case
		return AnyType()
	case *ast.DesugaredObject:
		// TODO
		return &TypeDesc{Object: true}
	case *ast.Error:
		return VoidType()
	case *ast.Index:
		// indexType := typeOf[node.Index]
		// TODO
		return AnyType()
	case *ast.Import:
		// complicated
		return AnyType()
	case *ast.LiteralBoolean:
		return &TypeDesc{Bool: true}
	case *ast.LiteralNull:
		return &TypeDesc{Null: true}

	case *ast.LiteralNumber:
		return &TypeDesc{Number: true}

	case *ast.LiteralString:
		return &TypeDesc{String: true}

	case *ast.Local:
		return typeOf[node.Body]
	case *ast.Self:
		// no recursion yet
		return &TypeDesc{Object: true}
	case *ast.SuperIndex:
		return &TypeDesc{Object: true}
	case *ast.InSuper:
		return &TypeDesc{Bool: true}
	case *ast.Function:
		// TODO(sbarzowski) more fancy description of functions...
		return &TypeDesc{Function: true}
	case *ast.Apply:
		// Can't do anything, before we have a better description of function types
		return AnyType()
	}
	panic(fmt.Sprintf("Unexpected %t", node))
}

func prepareTypes(node ast.Node, typeOf exprTypes) {
	prepareSubexprTypes(node, typeOf)
	typeOf[node] = calcType(node, typeOf)
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
		// spew.Dump(indexType)
		// spew.Dump(targetType)
		if !targetType.Array && !targetType.Object && !targetType.String {
			ec.staticErr("Indexed value is neither an array nor an object nor a string", node.Loc())
		} else if !targetType.Object {
			// It's not an object, so it must be an array or a string
			var assumedType string
			if targetType.Array && targetType.String {
				assumedType = "an array or a string"
			} else if targetType.Array {
				assumedType = "an array"
			} else {
				assumedType = "a string"
			}
			if !indexType.Number {
				ec.staticErr("Indexed value is assumed to be "+assumedType+", but index is not a number", node.Loc())
			}
		} else if !targetType.Array {
			// It's not an array so it must be an object
			if !indexType.String {
				ec.staticErr("Indexed value is assumed to be an object, but index is not a string", node.Loc())
			}
		} else if !indexType.Number && !indexType.String {
			// We don't know what the target is, but we sure cannot index it with that
			ec.staticErr("Index is neither a number (for indexing arrays and string) nor a string (for indexing objects)", node.Loc())
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
