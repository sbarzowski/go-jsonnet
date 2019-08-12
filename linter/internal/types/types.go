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

type SimpleTypeDesc struct {
	Bool     bool
	Number   bool
	String   bool
	Null     bool
	Function bool // TODO(sbarzowski) better rep
	Object   bool // TODO(sbarzowski) better rep
	Array    bool // TODO(sbarzowski) better rep
}

type objectDesc struct {
	allContain     []placeholderID
	fieldContains  map[string][]placeholderID
	allFieldsKnown bool
}

func (o *objectDesc) widen(other *objectDesc) {
	if other == nil {
		return
	}
	o.allContain = append(o.allContain, other.allContain...)
	for name, placeholders := range other.fieldContains {
		o.fieldContains[name] = append(o.fieldContains[name], placeholders...)
	}
	o.allFieldsKnown = o.allFieldsKnown || other.allFieldsKnown
}

type functionDesc struct {
	resultContains []placeholderID

	// TODO(sbarzowski) arity
}

func (f *functionDesc) widen(other *functionDesc) {
	if other == nil {
		return
	}

	f.resultContains = append(f.resultContains, other.resultContains...)
}

// TODO(sbarzowski) unexport this
type TypeDesc struct {
	Bool                 bool
	Number               bool
	String               bool
	Null                 bool
	FunctionDesc         *functionDesc
	ObjectDesc           *objectDesc
	Array                bool
	ArrayElementContains []placeholderID // TODO(sbarzowski) better rep
}

func (t *TypeDesc) Any() bool {
	return t.Bool && t.Number && t.String && t.Null && t.Function() && t.Object() && t.Array
}

func (t *TypeDesc) Void() bool {
	return !t.Bool && !t.Number && !t.String && !t.Null && !t.Function() && !t.Object() && !t.Array
}

func (t *TypeDesc) Function() bool {
	return t.FunctionDesc != nil
}

func (t *TypeDesc) Object() bool {
	return t.ObjectDesc != nil
}

func voidTypeDesc() TypeDesc {
	return TypeDesc{}
}

func Describe(t *TypeDesc) string {
	if t.Any() {
		return "any"
	}
	if t.Void() {
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
	if t.Function() {
		parts = append(parts, "function")
	}
	if t.Object() {
		parts = append(parts, "object")
	}
	if t.Array {
		parts = append(parts, "array")
	}
	return strings.Join(parts, " or ")
}

func (a *TypeDesc) widen(b *TypeDesc) {
	a.Bool = a.Bool || b.Bool
	a.Number = a.Number || b.Number
	a.String = a.String || b.String
	a.Null = a.Null || b.Null

	if a.FunctionDesc != nil {
		a.FunctionDesc.widen(b.FunctionDesc)
	} else if a.FunctionDesc == nil && b.FunctionDesc != nil {
		copy := *b.FunctionDesc
		a.FunctionDesc = &copy
	}

	if a.ObjectDesc != nil {
		a.ObjectDesc.widen(b.ObjectDesc)
	} else if a.ObjectDesc == nil && b.ObjectDesc != nil {
		copy := *b.ObjectDesc
		a.ObjectDesc = &copy
	}

	a.Array = a.Array || b.Array
	a.ArrayElementContains = append(a.ArrayElementContains, b.ArrayElementContains...)
}

type placeholderID int
type stronglyConnectedComponentID int

// 0 value for placeholderID acting as "nil" for placeholders
var noType placeholderID
var anyType placeholderID = 1
var boolType placeholderID = 2
var numberType placeholderID = 3
var stringType placeholderID = 4
var nullType placeholderID = 5
var anyObjectType placeholderID = 6
var anyFunctionType placeholderID = 7

type indexSpec struct {
	indexed placeholderID

	// TODO(sbarzowski) this name is ambigous think of something better or at least document it and make it consistent with helper function names
	stringIndex string

	knownStringIndex bool
	functionIndex    bool
}

func unknownIndexSpec(indexed placeholderID) *indexSpec {
	return &indexSpec{
		indexed:          indexed,
		stringIndex:      "",
		knownStringIndex: false,
	}
}

func knownObjectIndex(indexed placeholderID, index string) *indexSpec {
	return &indexSpec{
		indexed:          indexed,
		stringIndex:      index,
		knownStringIndex: true,
	}
}

func functionCallIndex(function placeholderID) *indexSpec {
	return &indexSpec{
		indexed:       function,
		functionIndex: true,
	}
}

type elementDesc struct {
	genericIndex placeholderID
	stringIndex  map[string]placeholderID
	callIndex    placeholderID
}

func (g *typeGraph) getOrCreateElementType(target placeholderID, index *indexSpec) (bool, placeholderID) {
	// In case there was no previous indexing
	if g.elementType[target] == nil {
		g.elementType[target] = &elementDesc{}
	}

	elementType := g.elementType[target]

	created := false

	// Actual specific indexing depending on the index type
	if index.knownStringIndex {
		if elementType.stringIndex == nil {
			elementType.stringIndex = make(map[string]placeholderID)
		}
		if elementType.stringIndex[index.stringIndex] == noType {
			created = true
			elID := g.newPlaceholder()
			elementType.stringIndex[index.stringIndex] = elID
			return created, elID
		} else {
			return created, elementType.stringIndex[index.stringIndex]
		}
	} else if index.functionIndex {
		if elementType.callIndex == noType {
			created = true
			elementType.callIndex = g.newPlaceholder()
		}
		return created, elementType.callIndex
	} else {
		if elementType.genericIndex == noType {
			created = true
			elementType.genericIndex = g.newPlaceholder()
		}
		return created, elementType.genericIndex
	}
}

func (g *typeGraph) setElementType(target placeholderID, index *indexSpec, newID placeholderID) {
	elementType := g.elementType[target]

	if index.knownStringIndex {
		elementType.stringIndex[index.stringIndex] = newID
	} else if index.functionIndex {
		elementType.callIndex = newID
	} else {
		elementType.genericIndex = newID
	}
}

type typePlaceholder struct {
	// Derived from AST
	concrete TypeDesc

	contains []placeholderID

	index *indexSpec
}

type typeGraph struct {
	_placeholders   []typePlaceholder
	exprPlaceholder map[ast.Node]placeholderID

	topoOrder []placeholderID
	sccOf     []stronglyConnectedComponentID

	elementType []*elementDesc
	// elementType            []placeholderID
	// stringIndexElementType []map[string]placeholderID

	upperBound []TypeDesc

	// Additional information about the program
	varAt map[ast.Node]*common.Variable
}

func (g *typeGraph) placeholder(id placeholderID) *typePlaceholder {
	return &g._placeholders[id]
}

func (g *typeGraph) newPlaceholder() placeholderID {
	g._placeholders = append(g._placeholders, typePlaceholder{})
	g.elementType = append(g.elementType, nil)

	return placeholderID(len(g._placeholders) - 1)
}

// simplifyReferences removes indirection through simple references, i.e. placeholders which contain
// exactly one other placeholder and which don't add anything else.
func (g *typeGraph) simplifyReferences() {
	mapping := make([]placeholderID, len(g._placeholders))
	for i, p := range g._placeholders {
		if p.concrete.Void() && p.index == nil && len(p.contains) == 1 {
			mapping[i] = p.contains[0]
		} else {
			mapping[i] = placeholderID(i)
		}
	}

	// transitive closure
	for i := range mapping {
		if mapping[mapping[i]] != mapping[i] {
			mapping[i] = mapping[mapping[i]]
		}
	}

	for i := range g._placeholders {
		p := g.placeholder(placeholderID(i))
		for j := range p.contains {
			p.contains[j] = mapping[p.contains[j]]
		}
		if p.index != nil {
			p.index.indexed = mapping[p.index.indexed]
		}
	}

	for k := range g.exprPlaceholder {
		g.exprPlaceholder[k] = mapping[g.exprPlaceholder[k]]
	}
}

func (g *typeGraph) separateElementTypes() {
	var getElementType func(container placeholderID, index *indexSpec) placeholderID
	getElementType = func(container placeholderID, index *indexSpec) placeholderID {
		c := g.placeholder(container)
		created, elID := g.getOrCreateElementType(container, index)

		if !created {
			return elID
		}

		indexType := "[]"
		if index.functionIndex {
			indexType = "()"
		}

		// Now we need to put all the stuff into element type
		contains := make([]placeholderID, 0, 1)

		// Direct indexing
		if index.knownStringIndex {
			if c.concrete.Object() {
				contains = append(contains, c.concrete.ObjectDesc.allContain...)
				contains = append(contains, c.concrete.ObjectDesc.fieldContains[index.stringIndex]...)
			}
			// TODO(sbarzowski) but here we need to save the right element type, not the generic one
		} else if index.functionIndex {
			if c.concrete.Function() {
				contains = append(contains, c.concrete.FunctionDesc.resultContains...)
			}
		} else {
			// TODO(sbarzowski) performance issues when the object is big
			if c.concrete.Object() {
				contains = append(contains, c.concrete.ObjectDesc.allContain...)
				for _, placeholders := range c.concrete.ObjectDesc.fieldContains {
					contains = append(contains, placeholders...)
				}
			}

			for _, p := range c.concrete.ArrayElementContains {
				contains = append(contains, p)
			}
		}

		// The indexed thing may itself be indexing something, so we need to go deeper
		if c.index != nil {
			elInC := getElementType(c.index.indexed, c.index)
			contains = append(contains, getElementType(elInC, index))
		}

		// The indexed thing may contain other values, we need to index those as well
		for _, contained := range c.contains {
			contains = append(contains, getElementType(contained, index))
		}

		fmt.Println("---------------------------------------\ngetElementType", container, "->", elID, indexType)
		spew.Dump(contains)

		g._placeholders[elID].contains = contains

		// Immediate path compression
		// TODO(sbarzowski) test which checks deep and recursive structure
		if len(contains) == 1 {
			g.setElementType(container, index, contains[0])
			return contains[0]
		}

		return elID
	}

	for i := range g._placeholders {
		index := g.placeholder(placeholderID(i)).index
		if index != nil {
			fmt.Println("Removing explicit indexing", i, "indexed", index.indexed)
			el := getElementType(index.indexed, index)
			fmt.Println("Index type of", i, "indexing", index.indexed, "is", el)
			// We carefully take a new pointer here, because getElementType might have reallocated it
			tp := &g._placeholders[i]
			tp.index = nil
			tp.contains = append(tp.contains, el)
		}
	}
}

func (g *typeGraph) makeTopoOrder() {
	visited := make([]bool, len(g._placeholders))

	g.topoOrder = make([]placeholderID, 0, len(g._placeholders))

	var visit func(p placeholderID)
	visit = func(p placeholderID) {
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
			visit(placeholderID(i))
		}
	}
}

func (g *typeGraph) findTypes() {
	dependentOn := make([][]placeholderID, len(g._placeholders))
	for i, p := range g._placeholders {
		for _, dependency := range p.contains {
			dependentOn[dependency] = append(dependentOn[dependency], placeholderID(i))
		}
	}

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
	for i, p := range g._placeholders {
		for _, contained := range p.contains {
			fmt.Println(i, "contains", contained)
		}
	}
	fmt.Println("topoOrder:", g.topoOrder)
}

func (g *typeGraph) resolveTypesInSCC(scc []placeholderID) {
	sccID := g.sccOf[scc[0]]

	common := voidTypeDesc()

	fmt.Println("======= resolving SCC: ", scc)

	for _, p := range scc {
		for _, contained := range g.placeholder(p).contains {
			if g.sccOf[contained] != sccID {
				common.widen(&g.upperBound[contained])
			}
		}
	}

	for _, p := range scc {
		common.widen(&g.placeholder(p).concrete)
		if g.placeholder(p).index != nil {
			panic(fmt.Sprintf("All indexing should have been rewritten to direct references at this point (indexing %d, indexed %d)", p, g.placeholder(p).index.indexed))
		}
	}

	for _, p := range scc {
		g.upperBound[p] = common
		// fmt.Println("----------------------------------------------------\n", p, Describe(&g.upperBound[p]))
		// spew.Dump(g.upperBound[p])
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
		contains: []placeholderID{p1, p2},
	}
}

func tpIndex(index *indexSpec) typePlaceholder {
	return typePlaceholder{
		concrete: voidTypeDesc(),
		contains: nil,
		index:    index,
	}
}

func tpRef(p placeholderID) typePlaceholder {
	return typePlaceholder{
		contains: []placeholderID{p},
	}
}

func prepareTPWithPlaceholder(node ast.Node, g *typeGraph, p placeholderID) {
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
		elements := make([]placeholderID, len(node.Elements))
		for i, el := range node.Elements {
			elements[i] = g.exprPlaceholder[el]
		}
		return concreteTP(TypeDesc{Array: true, ArrayElementContains: elements})
	case *ast.Binary:
		// complicated
		return tpRef(anyType)
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
			return concreteTP(TypeDesc{ObjectDesc: &objectDesc{
				allFieldsKnown: false,
				allContain:     []placeholderID{anyType},
			}})
		}
		if v.Param {
			return tpRef(anyType)
		}
		return tpRef(g.exprPlaceholder[v.BindNode])
	case *ast.DesugaredObject:
		// TODO
		obj := &objectDesc{
			allFieldsKnown: true,
			fieldContains:  make(map[string][]placeholderID),
		}
		for _, field := range node.Fields {
			switch fieldName := field.Name.(type) {
			case *ast.LiteralString:
				obj.fieldContains[fieldName.Value] = append(obj.fieldContains[fieldName.Value], g.exprPlaceholder[field.Body])
			default:
				obj.allContain = append(obj.allContain, g.exprPlaceholder[field.Body])
				obj.allFieldsKnown = false
			}
		}
		return concreteTP(TypeDesc{ObjectDesc: obj})
	case *ast.Error:
		return concreteTP(voidTypeDesc())
	case *ast.Index:
		// indexType := typeOf[node.Index]
		// TODO
		switch index := node.Index.(type) {
		case *ast.LiteralString:
			return tpIndex(knownObjectIndex(g.exprPlaceholder[node.Target], index.Value))
		default:
			return tpIndex(unknownIndexSpec(g.exprPlaceholder[node.Target]))
		}
	case *ast.Import:
		// complicated
		return tpRef(anyType)
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
		return tpRef(g.exprPlaceholder[node.Body])
	case *ast.Self:
		// no recursion yet
		return tpRef(anyObjectType)
	case *ast.SuperIndex:
		return tpRef(anyObjectType)
	case *ast.InSuper:
		return tpRef(boolType)
	case *ast.Function:
		// TODO(sbarzowski) more fancy description of functions...
		fmt.Println("Body", g.exprPlaceholder[node.Body])
		return concreteTP(TypeDesc{FunctionDesc: &functionDesc{
			resultContains: []placeholderID{g.exprPlaceholder[node.Body]},
		}})
	case *ast.Apply:
		return tpIndex(functionCallIndex(g.exprPlaceholder[node.Target]))
	}
	panic(fmt.Sprintf("Unexpected %t", node))
}

type ExprTypes map[ast.Node]TypeDesc

func PrepareTypes(node ast.Node, typeOf ExprTypes, varAt map[ast.Node]*common.Variable) {
	g := typeGraph{
		exprPlaceholder: make(map[ast.Node]placeholderID),
		varAt:           varAt,
	}

	anyObjectDesc := &objectDesc{
		allFieldsKnown: false,
		allContain:     []placeholderID{anyType},
	}

	anyFunctionDesc := &functionDesc{
		resultContains: []placeholderID{anyType},
	}

	// Create the "no-type" sentinel placeholder
	g.newPlaceholder()

	// any type
	g.newPlaceholder()
	g._placeholders[anyType] = concreteTP(TypeDesc{
		Bool:                 true,
		Number:               true,
		String:               true,
		Null:                 true,
		FunctionDesc:         anyFunctionDesc,
		ObjectDesc:           anyObjectDesc,
		Array:                true,
		ArrayElementContains: []placeholderID{anyType},
	})

	g.newPlaceholder()
	g._placeholders[boolType] = concreteTP(TypeDesc{
		Bool: true,
	})

	g.newPlaceholder()
	g._placeholders[numberType] = concreteTP(TypeDesc{
		Number: true,
	})

	g.newPlaceholder()
	g._placeholders[stringType] = concreteTP(TypeDesc{
		String: true,
	})

	g.newPlaceholder()
	g._placeholders[nullType] = concreteTP(TypeDesc{
		Null: true,
	})

	g.newPlaceholder()
	g._placeholders[anyObjectType] = concreteTP(TypeDesc{
		ObjectDesc: anyObjectDesc,
	})

	g.newPlaceholder()
	g._placeholders[anyFunctionType] = concreteTP(TypeDesc{
		FunctionDesc: anyFunctionDesc,
	})

	prepareTP(node, &g)

	g.simplifyReferences()

	g.separateElementTypes()
	g.makeTopoOrder()
	g.findTypes()
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
		t := typeOf[node.Target]
		if !t.Function() {
			ec.staticErr("Called value must be a function, but it is assumed to be "+Describe(&t), node.Loc())
		}
	case *ast.Index:
		targetType := typeOf[node.Target]
		indexType := typeOf[node.Index]
		// spew.Dump(indexType)
		// spew.Dump(targetType)
		if !targetType.Array && !targetType.Object() && !targetType.String {
			ec.staticErr("Indexed value is neither an array nor an object nor a string", node.Loc())
		} else if !targetType.Object() {
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
			if targetType.ObjectDesc.allFieldsKnown {
				switch indexNode := node.Index.(type) {
				case *ast.LiteralString:
					if _, hasField := targetType.ObjectDesc.fieldContains[indexNode.Value]; !hasField {
						ec.staticErr(fmt.Sprintf("Indexed object has no field %#v", indexNode.Value), node.Loc())
					}
				}
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
