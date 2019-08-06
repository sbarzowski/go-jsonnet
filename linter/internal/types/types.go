package types

import (
	"fmt"
	"os"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/fatih/color"

	jsonnet "github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/linter/internal/common"
	"github.com/google/go-jsonnet/parser"
)

// Even though Jsonnet doesn't have a concept of static types
// we can infer for each expression what values it can take.
// Of course we cannot do this accurately at all times, but even
// coarse grained information about "types" can help with some bugs.
// We are mostly interested in simple issues - like using a nonexistent
// field of an object or treating an array like a function.
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

type TypeDesc struct {
	Bool     bool
	Number   bool
	String   bool
	Null     bool
	Function bool // TODO(sbarzowski) better rep
	Object   bool // TODO(sbarzowski) better rep
	Array    bool // TODO(sbarzowski) better rep
}

func Describe(t *TypeDesc) string {
	if t.Bool && t.Number && t.String && t.Null && t.Function && t.Object && t.Array {
		return "any"
	}
	if !t.Bool && !t.Number && !t.String && !t.Null && !t.Function && !t.Object && !t.Array {
		return "void"
	}
	parts := []string{}
	if t.Bool {
		parts = append(parts, "bool")
	}
	if t.Number {
		parts = append(parts, "number")
	}
	if t.String {
		parts = append(parts, "string")
	}
	if t.Null {
		parts = append(parts, "null")
	}
	if t.Function {
		parts = append(parts, "function")
	}
	if t.Object {
		parts = append(parts, "object")
	}
	if t.Array {
		parts = append(parts, "array")
	}
	return strings.Join(parts, " or ")
}

type placeholderID int
type stronglyConnectedComponentID int

// 0 value for placeholderID acting as "nil" for placeholders
var noType placeholderID

type typePlaceholder struct {
	// Derived from AST
	concrete TypeDesc

	contains []placeholderID

	indexes placeholderID
}

type typeGraph struct {
	_placeholders   []typePlaceholder
	exprPlaceholder map[ast.Node]placeholderID

	topoOrder []placeholderID
	sccOf     []stronglyConnectedComponentID

	upperBound []TypeDesc

	// Additional information about the program
	varAt map[ast.Node]*common.Variable
}

func (g *typeGraph) placeholder(id placeholderID) *typePlaceholder {
	return &g._placeholders[id]
}

func (g *typeGraph) newPlaceholder() placeholderID {
	g._placeholders = append(g._placeholders, typePlaceholder{})
	return placeholderID(len(g._placeholders) - 1)
}

func (g *typeGraph) makeTopoOrder() {
	visited := make([]bool, len(g._placeholders))

	g.topoOrder = make([]placeholderID, 0, len(g._placeholders))

	var visit func(p placeholderID)
	visit = func(p placeholderID) {
		visited[p] = true
		for _, child := range g.placeholder(p).contains {
			fmt.Printf("%d -> %d\n", p, child)
			if !visited[child] {
				visit(child)
			}
		}
		g.topoOrder = append(g.topoOrder, p)
	}

	for i := range g._placeholders {
		if !visited[i] {
			visit(placeholderID(i))
		}
	}
	spew.Dump(g.topoOrder)
}

func (g *typeGraph) findTypes() {
	dependentOn := make([][]placeholderID, len(g._placeholders))
	for i, p := range g._placeholders {
		for _, dependency := range p.contains {
			dependentOn[dependency] = append(dependentOn[dependency], placeholderID(i))
		}
	}
	spew.Dump(dependentOn)

	visited := make([]bool, len(g._placeholders))
	g.sccOf = make([]stronglyConnectedComponentID, len(g._placeholders))

	stronglyConnectedComponents := make([][]placeholderID, 0)
	var sccID stronglyConnectedComponentID

	var visit func(p placeholderID)
	visit = func(p placeholderID) {
		visited[p] = true
		g.sccOf[p] = sccID
		stronglyConnectedComponents[sccID] = append(stronglyConnectedComponents[sccID], p)
		for _, dependent := range dependentOn[p] {
			if !visited[dependent] {
				visit(dependent)
			}
		}
	}

	g.upperBound = make([]TypeDesc, len(g._placeholders))

	for i := len(g.topoOrder) - 1; i >= 0; i-- {
		p := g.topoOrder[i]
		if !visited[p] {
			stronglyConnectedComponents = append(stronglyConnectedComponents, make([]placeholderID, 0, 1))
			visit(p)
			sccID++
		}
	}

	for i := len(stronglyConnectedComponents) - 1; i >= 0; i-- {
		scc := stronglyConnectedComponents[i]
		g.resolveTypesInSCC(scc)
	}
}

func (g *typeGraph) resolveTypesInSCC(scc []placeholderID) {
	sccID := g.sccOf[scc[0]]

	fmt.Println("Strongly connected component")
	spew.Dump(scc)

	common := *voidType()

	for _, p := range scc {
		for _, contained := range g.placeholder(p).contains {
			if g.sccOf[contained] != sccID {
				common = *widen(&common, &g.upperBound[contained])
				fmt.Println("widening with:", contained, "result:", Describe(&common))
			}
		}
	}

	fmt.Println("common:", Describe(&common))

	for _, p := range scc {
		common = *widen(&common, &g.placeholder(p).concrete)
	}

	fmt.Println("final:", Describe(&common))

	for _, p := range scc {
		g.upperBound[p] = common
	}
}

func concreteTP(t TypeDesc) typePlaceholder {
	return typePlaceholder{
		concrete: t,
		contains: nil,
	}
}

func tpSum(p1, p2 placeholderID) typePlaceholder {
	return typePlaceholder{
		concrete: *voidType(),
		contains: []placeholderID{p1, p2},
	}
}

func tpRef(p placeholderID) typePlaceholder {
	return typePlaceholder{
		concrete: *voidType(),
		contains: []placeholderID{p},
	}
}

func anyType() *TypeDesc {
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

func voidType() *TypeDesc {
	return &TypeDesc{}
}

func widen(a *TypeDesc, b *TypeDesc) *TypeDesc {
	fmt.Println("Widening (", Describe(a), ") (", Describe(b), ")")
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

func prepareTPWithPlaceholder(node ast.Node, g *typeGraph, p placeholderID) {
	switch node := node.(type) {
	case *ast.Local:
		bindPlaceholders := make([]placeholderID, len(node.Binds))
		for i := range node.Binds {
			bindPlaceholders[i] = g.newPlaceholder()
			fmt.Println("placeholder for bind", bindPlaceholders[i])
			g.exprPlaceholder[node.Binds[i].Body] = bindPlaceholders[i]
			fmt.Println("exprPlaceholder len =", len(g.exprPlaceholder))
		}
		for i := range node.Binds {
			// TODO(sbarzowski) what about func? is desugaring allowing us to avoid that
			prepareTPWithPlaceholder(node.Binds[i].Body, g, bindPlaceholders[i])
		}
		prepareTP(node.Body, g)
	default:
		for _, child := range parser.Children(node) {
			prepareTP(child, g)
		}
	}
	*(g.placeholder(p)) = calcTP(node, g)
}

func prepareTP(node ast.Node, g *typeGraph) {
	p := g.newPlaceholder()
	g.exprPlaceholder[node] = p
	prepareTPWithPlaceholder(node, g, p)
}

func calcTP(node ast.Node, g *typeGraph) typePlaceholder {
	switch node := node.(type) {
	case *ast.Array:
		return concreteTP(TypeDesc{Array: true})
	case *ast.Binary:
		// complicated
		return concreteTP(*anyType())
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
		v := g.varAt[node]
		if v == nil {
			panic("Could not find variable")
		}
		if v.Stdlib {
			return concreteTP(TypeDesc{Object: true})
		} else {
			return tpRef(g.exprPlaceholder[v.BindNode])
		}
	case *ast.DesugaredObject:
		// TODO
		return concreteTP(TypeDesc{Object: true})
	case *ast.Error:
		return concreteTP(*voidType())
	case *ast.Index:
		// indexType := typeOf[node.Index]
		// TODO
		return typePlaceholder{
			concrete: *voidType(),
			contains: nil,
			indexes:  g.exprPlaceholder[node.Target],
		}
	case *ast.Import:
		// complicated
		return concreteTP(*anyType())
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
		return concreteTP(*anyType())
	}
	panic(fmt.Sprintf("Unexpected %t", node))
}

type ExprTypes map[ast.Node]TypeDesc

func PrepareTypes(node ast.Node, typeOf ExprTypes, varAt map[ast.Node]*common.Variable) {
	g := typeGraph{
		exprPlaceholder: make(map[ast.Node]placeholderID),
		varAt:           varAt,
	}
	// Create the "no-type" sentinel placeholder
	g.newPlaceholder()

	prepareTP(node, &g)
	g.makeTopoOrder()
	g.findTypes()
	spew.Dump(g.upperBound)
	for e, p := range g.exprPlaceholder {
		// TODO(sbarzowski) using errors for debugging, ugh
		lf := jsonnet.LinterFormatter()
		lf.SetColorFormatter(color.New(color.FgRed).Fprintf)
		fmt.Fprintf(os.Stderr, lf.Format(parser.StaticError{
			Loc: *e.Loc(),
			Msg: fmt.Sprintf("placeholder %d is %s", p, Describe(&g.upperBound[p])),
		}))

		// TODO(sbarzowski) here we'll need to handle additional
		typeOf[e] = g.upperBound[p]
	}
}

type ErrCollector struct {
	Errs []parser.StaticError
}

func (ec *ErrCollector) collect(err parser.StaticError) {
	ec.Errs = append(ec.Errs, err)
}

func (ec *ErrCollector) staticErr(msg string, loc *ast.LocationRange) {
	ec.collect(parser.MakeStaticError(msg, *loc))
}

func checkSubexpr(node ast.Node, typeOf ExprTypes, ec *ErrCollector) {
	for _, child := range parser.Children(node) {
		Check(child, typeOf, ec)
	}
}

func Check(node ast.Node, typeOf ExprTypes, ec *ErrCollector) {
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
