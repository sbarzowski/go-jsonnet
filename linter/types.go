package linter

import (
	"fmt"

	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/parser"
)

// Even though Jsonnet doesn't have a concept of static types
// we can infer for each expression what values it can take.
// Of course we cannot do this accurately at all times, but even
// coarse grained information about "types" can help with some bugs.
// We are mostly interested in simple issues - like using a nonexistent
// field of an object or
//
// Main assumptions:
// * It has to work with existing programs well
// * It needs to be conservative - strong preference for false negatives over false positives
//
//
// First of all "type" processing split into two very distinct phases:
// 1) Finding possible values for each expression
// 2)

// Errors are not typed - they can fit any type
// Capabilities, sets, elements...

// TODO(sbarzowski) what should be exported and what shouldn't

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

type PlaceholderID int
type stronglyConnectedComponentID int

type TypePlaceholder struct {
	// Derived from AST
	concrete TypeDesc

	contains []PlaceholderID
}

type TypeGraph struct {
	_placeholders   []TypePlaceholder
	exprPlaceholder map[ast.Node]PlaceholderID

	topoOrder []PlaceholderID
	sccOf     []stronglyConnectedComponentID

	upperBound []TypeDesc
}

func (g *TypeGraph) placeholder(id PlaceholderID) *TypePlaceholder {
	return &g._placeholders[id]
}

func (g *TypeGraph) newPlaceholder() PlaceholderID {
	g._placeholders = append(g._placeholders, TypePlaceholder{})
	return PlaceholderID(len(g._placeholders) - 1)
}

func (g *TypeGraph) makeTopoOrder() {
	visited := make([]bool, len(g._placeholders))

	g.topoOrder = make([]PlaceholderID, 0, len(g._placeholders))

	var visit func(p PlaceholderID)
	visit = func(p PlaceholderID) {
		visited[p] = true
		for _, child := range g.placeholder(p).contains {
			if !visited[child] {
				visit(child)
			}
		}
		g.topoOrder = append(g.topoOrder, p)
	}

	for i := range g._placeholders {
		if !visited[i] {
			visit(PlaceholderID(i))
		}
	}
}

func (g *TypeGraph) findTypes() {
	dependentOn := make([][]PlaceholderID, len(g._placeholders))
	for i, p := range g._placeholders {
		for _, dependency := range p.contains {
			dependentOn[dependency] = append(dependentOn[dependency], PlaceholderID(i))
		}
	}

	visited := make([]bool, len(g._placeholders))
	g.sccOf = make([]stronglyConnectedComponentID, len(g._placeholders))

	stronglyConnectedComponent := make([]PlaceholderID, 0, 10)
	var sccID stronglyConnectedComponentID

	var visit func(p PlaceholderID)
	visit = func(p PlaceholderID) {
		visited[p] = true
		g.sccOf[p] = sccID
		stronglyConnectedComponent = append(stronglyConnectedComponent, p)
		for _, dependent := range dependentOn[p] {
			if !visited[dependent] {
				visit(dependent)
			}
		}
	}

	g.upperBound = make([]TypeDesc, len(g._placeholders))

	for _, p := range g.topoOrder {
		if !visited[p] {
			visit(p)
			g.resolveTypesInSCC(stronglyConnectedComponent)
			sccID++
		}
		// Clear without freeing the underlying memory
		stronglyConnectedComponent = stronglyConnectedComponent[:0]
	}
}

func (g *TypeGraph) resolveTypesInSCC(scc []PlaceholderID) {
	sccID := g.sccOf[scc[0]]

	common := *AnyType()

	for _, p := range scc {
		for _, contained := range g.placeholder(p).contains {
			if g.sccOf[contained] != sccID {
				common = *widen(&common, &g.upperBound[contained])
			}
		}
	}

	for _, p := range scc {
		g.upperBound[p] = common
	}
}

func concreteTP(t TypeDesc) TypePlaceholder {
	return TypePlaceholder{
		concrete: t,
		contains: nil,
	}
}

func tpSum(p1, p2 PlaceholderID) TypePlaceholder {
	return TypePlaceholder{
		concrete: *VoidType(),
		contains: []PlaceholderID{p1, p2},
	}
}

func tpRef(p PlaceholderID) TypePlaceholder {
	return TypePlaceholder{
		concrete: *VoidType(),
		contains: []PlaceholderID{p},
	}
}

type exprTypes map[ast.Node]*TypeDesc
type exprTP map[ast.Node]*TypePlaceholder

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

func widen(a *TypeDesc, b *TypeDesc) *TypeDesc {
	return &TypeDesc{
		Bool:     a.Bool || b.Bool,
		Number:   a.Number || b.Number,
		String:   a.String || b.String,
		Null:     a.Null || b.Null,
		Function: a.Function || b.Function,
		Object:   a.Object || b.Object,
		Array:    a.Array || b.Array,
	}
}

func prepareTP(node ast.Node, g *TypeGraph) {
	prepareSubexprTPs(node, g)
	placeholderID := g.newPlaceholder()
	*(g.placeholder(placeholderID)) = calcTP(node, g)
}

func prepareSubexprTPs(node ast.Node, g *TypeGraph) {
	for _, child := range parser.Children(node) {
		prepareTP(child, g)
	}
}

func calcTP(node ast.Node, g *TypeGraph) TypePlaceholder {
	switch node := node.(type) {
	case *ast.Array:
		return concreteTP(TypeDesc{Array: true})
	case *ast.Binary:
		// complicated
		return concreteTP(*AnyType())
	case *ast.Unary:
		// complicated
		switch node.Op {
		case ast.UopNot:
			return concreteTP(TypeDesc{Bool: true})
		case ast.UopBitwiseNot:
		case ast.UopPlus:
		case ast.UopMinus:
			return concreteTP(TypeDesc{Number: true})
		default:
			panic(fmt.Sprintf("Unrecognized unary operator %v", node.Op))
		}
	case *ast.Conditional:
		return tpSum(g.exprPlaceholder[node.BranchTrue], g.exprPlaceholder[node.BranchFalse])
	case *ast.Var:
		// need to get expr of var
		// We may not know the type of the Var yet, for now, let's assume Any in such case
		return concreteTP(*AnyType())
	case *ast.DesugaredObject:
		// TODO
		return concreteTP(TypeDesc{Object: true})
	case *ast.Error:
		return concreteTP(*VoidType())
	case *ast.Index:
		// indexType := typeOf[node.Index]
		// TODO
		return concreteTP(*AnyType())
	case *ast.Import:
		// complicated
		return concreteTP(*AnyType())
	case *ast.LiteralBoolean:
		return concreteTP(TypeDesc{Bool: true})
	case *ast.LiteralNull:
		return concreteTP(TypeDesc{Null: true})

	case *ast.LiteralNumber:
		return concreteTP(TypeDesc{Number: true})

	case *ast.LiteralString:
		return concreteTP(TypeDesc{String: true})

	case *ast.Local:
		// TODO(sbarzowski) perhaps it should return the id and any creation of the new placeholders would happend in this function
		// then we would be able to avoid unnecessary indirection
		return tpRef(g.exprPlaceholder[node.Body])
	case *ast.Self:
		// no recursion yet
		return concreteTP(TypeDesc{Object: true})
	case *ast.SuperIndex:
		return concreteTP(TypeDesc{Object: true})
	case *ast.InSuper:
		return concreteTP(TypeDesc{Bool: true})
	case *ast.Function:
		// TODO(sbarzowski) more fancy description of functions...
		return concreteTP(TypeDesc{Function: true})
	case *ast.Apply:
		// Can't do anything, before we have a better description of function types
		return concreteTP(*AnyType())
	}
	panic(fmt.Sprintf("Unexpected %t", node))
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
		switch node.Op {
		case ast.UopNot:
			return &TypeDesc{Bool: true}
		case ast.UopBitwiseNot:
		case ast.UopPlus:
		case ast.UopMinus:
			return &TypeDesc{Number: true}
		default:
			panic(fmt.Sprintf("Unrecognized unary operator %v", node.Op))
		}
	case *ast.Conditional:
		return widen(typeOf[node.BranchTrue], typeOf[node.BranchFalse])
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

func prepareTypesWithGraph(node ast.Node, typeOf exprTypes) {
	g := TypeGraph{}
	prepareTP(node, &g)
	g.makeTopoOrder()
	g.findTypes()
	for e, p := range g.exprPlaceholder {
		fmt.Println(e, p)
		// eh, here a copy would probably be better
		typeOf[e] = &g.upperBound[p]
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
	case *ast.Unary:
		// TODO(sbarzowski) this
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
