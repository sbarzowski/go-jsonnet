package types

import (
	"fmt"
	"math"
	"os"
	"sort"

	"github.com/fatih/color"

	"github.com/google/go-jsonnet"
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

// type SimpleTypeDesc struct {
// 	Bool     bool
// 	Number   bool
// 	String   bool
// 	Null     bool
// 	Function bool // TODO(sbarzowski) better rep
// 	Object   bool // TODO(sbarzowski) better rep
// 	Array    bool // TODO(sbarzowski) better rep
// }

type placeholderID int
type stronglyConnectedComponentID int

// 0 value for placeholderID acting as "nil" for placeholders
const (
	noType placeholderID = iota
	anyType
	boolType
	numberType
	stringType
	nullType
	anyArrayType
	anyObjectType
	anyFunctionType
	stdlibType
)

type placeholderIDs []placeholderID

func (p placeholderIDs) Len() int           { return len(p) }
func (p placeholderIDs) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p placeholderIDs) Less(i, j int) bool { return p[i] < p[j] }

func normalizePlaceholders(placeholders []placeholderID) []placeholderID {
	if len(placeholders) == 0 {
		return placeholders
	}
	sort.Sort(placeholderIDs(placeholders))
	// Unique
	count := 1
	for i := 1; i < len(placeholders); i++ {
		if placeholders[i] == anyType {
			placeholders[0] = anyType
			return placeholders[:1]
		}
		if placeholders[i] != placeholders[count-1] {
			placeholders[count] = placeholders[i]
			count++
		}
	}
	// We return a slice pointing to the same underlying array - reallocation to reduce it is not what we want probably
	return placeholders[:count]
}

func (g *typeGraph) getOrCreateElementType(target placeholderID, index *indexSpec) (bool, placeholderID) {
	// In case there was no previous indexing
	if g.elementType[target] == nil {
		g.elementType[target] = &elementDesc{}
	}

	elementType := g.elementType[target]

	created := false

	// Actual specific indexing depending on the index type
	if index.indexType == knownStringIndex {
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
	} else if index.indexType == knownIntIndex {
		if elementType.intIndex == nil {
			elementType.intIndex = make([]placeholderID, maxKnownCount)
		}
		if elementType.intIndex[index.intIndex] == noType {
			created = true
			elID := g.newPlaceholder()
			elementType.intIndex[index.intIndex] = elID
			return created, elID
		} else {
			return created, elementType.intIndex[index.intIndex]
		}
	} else if index.indexType == functionIndex {
		if elementType.callIndex == noType {
			created = true
			elementType.callIndex = g.newPlaceholder()
		}
		return created, elementType.callIndex
	} else if index.indexType == genericIndex {
		if elementType.genericIndex == noType {
			created = true
			elementType.genericIndex = g.newPlaceholder()
		}
		return created, elementType.genericIndex
	} else {
		panic("unknown index type")
	}
}

func (g *typeGraph) setElementType(target placeholderID, index *indexSpec, newID placeholderID) {
	elementType := g.elementType[target]

	if index.indexType == knownStringIndex {
		elementType.stringIndex[index.stringIndex] = newID
	} else if index.indexType == functionIndex {
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

		// Now we need to put all the stuff into element type
		contains := make([]placeholderID, 0, 1)

		// Direct indexing
		if index.indexType == knownStringIndex {
			if c.concrete.Object() {
				contains = append(contains, c.concrete.ObjectDesc.allContain...)
				contains = append(contains, c.concrete.ObjectDesc.fieldContains[index.stringIndex]...)
			}
		} else if index.indexType == knownIntIndex {
			// TODO(sbarzowski) what if it's a string
			if c.concrete.Array() {
				// TODO(sbarzowski) consider changing the representation to otherContain - it could be more useful
				contains = append(contains, c.concrete.ArrayDesc.allContain...)
				if index.intIndex < len(c.concrete.ArrayDesc.elementContains) {
					contains = append(contains, c.concrete.ArrayDesc.elementContains[index.intIndex]...)
				}
			}
		} else if index.indexType == functionIndex {
			if c.concrete.Function() {
				contains = append(contains, c.concrete.FunctionDesc.resultContains...)
			}
		} else if index.indexType == genericIndex {
			// TODO(sbarzowski) performance issues when the object is big
			if c.concrete.Object() {
				contains = append(contains, c.concrete.ObjectDesc.allContain...)
				for _, placeholders := range c.concrete.ObjectDesc.fieldContains {
					contains = append(contains, placeholders...)
				}
			}

			if c.concrete.ArrayDesc != nil {
				for _, placeholders := range c.concrete.ArrayDesc.elementContains {
					contains = append(contains, placeholders...)
				}
				contains = append(contains, c.concrete.ArrayDesc.allContain...)
			}

			// TODO(sbarzowski) what if it's a string
		} else {
			panic("unknown index type")
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

		contains = normalizePlaceholders(contains)
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
			el := getElementType(index.indexed, index)
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
}

func (g *typeGraph) resolveTypesInSCC(scc []placeholderID) {
	sccID := g.sccOf[scc[0]]

	common := voidTypeDesc()

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

	common.normalize()

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

type ExprTypes map[ast.Node]TypeDesc

func (g *typeGraph) newSimpleFuncType(returnType placeholderID, argNames ...ast.Identifier) placeholderID {
	p := g.newPlaceholder()
	g._placeholders[p] = concreteTP(TypeDesc{
		FunctionDesc: &functionDesc{
			resultContains: []placeholderID{returnType},
			params: &ast.Parameters{
				Required: argNames,
			},
			minArity: len(argNames),
			maxArity: len(argNames),
		},
	})
	return p
}

func prepareStdlib(g *typeGraph) {
	g.newPlaceholder()

	// TODO these
	arrayOfString := anyArrayType
	arrayOfNumber := anyArrayType
	stringOrArray := anyType
	stringOrNumber := anyType
	jsonType := anyType // It actually cannot functions anywhere

	fields := map[string]placeholderID{
		// External variables
		"extVar": g.newSimpleFuncType(anyType, "x"),

		// Types and reflection
		"thisFile":        stringType,
		"type":            g.newSimpleFuncType(stringType, "x"),
		"length":          g.newSimpleFuncType(numberType, "x"),
		"objectHas":       g.newSimpleFuncType(boolType, "o", "f"),
		"objectFields":    g.newSimpleFuncType(arrayOfString, "o"),
		"objectHasAll":    g.newSimpleFuncType(boolType, "o", "f"),
		"objectFieldsAll": g.newSimpleFuncType(arrayOfString, "o"),
		"prune":           g.newSimpleFuncType(anyObjectType, "o"),
		"mapWithKey":      g.newSimpleFuncType(anyObjectType, "func", "obj"),

		// Mathematical utilities
		"abs":      g.newSimpleFuncType(numberType, "n"),
		"sign":     g.newSimpleFuncType(numberType, "n"),
		"max":      g.newSimpleFuncType(numberType, "a", "b"),
		"min":      g.newSimpleFuncType(numberType, "a", "b"),
		"pow":      g.newSimpleFuncType(numberType, "x", "n"),
		"exp":      g.newSimpleFuncType(numberType, "x"),
		"log":      g.newSimpleFuncType(numberType, "x"),
		"exponent": g.newSimpleFuncType(numberType, "x"),
		"mantissa": g.newSimpleFuncType(numberType, "x"),
		"floor":    g.newSimpleFuncType(numberType, "x"),
		"ceil":     g.newSimpleFuncType(numberType, "x"),
		"sqrt":     g.newSimpleFuncType(numberType, "x"),
		"sin":      g.newSimpleFuncType(numberType, "x"),
		"cos":      g.newSimpleFuncType(numberType, "x"),
		"tan":      g.newSimpleFuncType(numberType, "x"),
		"asin":     g.newSimpleFuncType(numberType, "x"),
		"acos":     g.newSimpleFuncType(numberType, "x"),
		"atan":     g.newSimpleFuncType(numberType, "x"),

		// Assertions and debugging
		"assertEqual": g.newSimpleFuncType(boolType, "a", "b"),

		// String Manipulation

		"toString":            g.newSimpleFuncType(stringType, "a"),
		"codepoint":           g.newSimpleFuncType(numberType, "str"),
		"char":                g.newSimpleFuncType(stringType, "n"),
		"substr":              g.newSimpleFuncType(stringType, "s", "from", "len"),
		"findSubstr":          g.newSimpleFuncType(arrayOfNumber, "pat", "str"),
		"startsWith":          g.newSimpleFuncType(boolType, "a", "b"),
		"endsWith":            g.newSimpleFuncType(boolType, "a", "b"),
		"split":               g.newSimpleFuncType(arrayOfString, "str", "c"),
		"splitLimit":          g.newSimpleFuncType(arrayOfString, "str", "c", "maxsplits"),
		"strReplace":          g.newSimpleFuncType(stringType, "str", "from", "to"),
		"asciiUpper":          g.newSimpleFuncType(stringType, "str"),
		"asciiLower":          g.newSimpleFuncType(stringType, "str"),
		"stringChars":         g.newSimpleFuncType(stringType, "str"),
		"format":              g.newSimpleFuncType(stringType, "str", "vals"),
		"escapeStringBash":    g.newSimpleFuncType(stringType, "str"),
		"escapeStringDollars": g.newSimpleFuncType(stringType, "str"),
		"escapeStringJson":    g.newSimpleFuncType(stringType, "str"),
		"escapeStringPython":  g.newSimpleFuncType(stringType, "str"),

		// Parsing

		"parseInt":   g.newSimpleFuncType(numberType, "str"),
		"parseOctal": g.newSimpleFuncType(numberType, "str"),
		"parseHex":   g.newSimpleFuncType(numberType, "str"),
		"parseJson":  g.newSimpleFuncType(jsonType, "str"),
		"encodeUTF8": g.newSimpleFuncType(arrayOfNumber, "str"),
		"decodeUTF8": g.newSimpleFuncType(stringType, "arr"),

		// Manifestation

		"manifestIni":        g.newSimpleFuncType(stringType, "v"),
		"manifestPython":     g.newSimpleFuncType(stringType, "v"),
		"manifestPythonVars": g.newSimpleFuncType(stringType, "v"),
		"manifestJsonEx":     g.newSimpleFuncType(stringType, "value", "indent"),
		"manifestYamlDoc":    g.newSimpleFuncType(stringType, "value"),
		"manifestYamlStream": g.newSimpleFuncType(stringType, "value"),
		"manifestXmlJsonml":  g.newSimpleFuncType(stringType, "value"),

		// Arrays

		"makeArray":     g.newSimpleFuncType(anyArrayType, "sz", "func"),
		"count":         g.newSimpleFuncType(numberType, "arr", "x"),
		"find":          g.newSimpleFuncType(arrayOfNumber, "value", "arr"),
		"map":           g.newSimpleFuncType(anyArrayType, "func", "arr"),
		"mapWithIndex":  g.newSimpleFuncType(anyArrayType, "func", "arr"),
		"filterMap":     g.newSimpleFuncType(anyArrayType, "filter_func", "map_func", "arr"),
		"filter":        g.newSimpleFuncType(anyArrayType, "func", "arr"),
		"foldl":         g.newSimpleFuncType(anyType, "func", "arr", "init"),
		"foldr":         g.newSimpleFuncType(anyType, "func", "arr", "init"),
		"range":         g.newSimpleFuncType(arrayOfNumber, "from", "to"),
		"join":          g.newSimpleFuncType(stringOrArray, "sep", "arr"),
		"lines":         g.newSimpleFuncType(arrayOfString, "arr"),
		"flattenArrays": g.newSimpleFuncType(anyArrayType, "arrs"),
		// TODO(sbarzowski) support optional args
		// "sort": g.newSimpleFuncType(anyArrayType, "arr", keyF=id),
		// Don't we have keyF for uniq? Perhaps we should?
		"uniq": g.newSimpleFuncType(anyArrayType, "arr"),

		// Sets

		// TODO(sbarzowski) support optional args
		// "set": g.newSimpleFuncType(comparableArray, "arr", keyF=id)
		// "setInter": g.newSimpleFuncType(comparableArray, "a", b, keyF=id)
		// "setUnion": g.newSimpleFuncType(comparableArray, "a", b, keyF=id)
		// "setDiff": g.newSimpleFuncType(comparableArray, "a", b, keyF=id)
		// "setMember": g.newSimpleFuncType(comparableArray, "x", arr, keyF=id)

		// Encoding

		"base64":            g.newSimpleFuncType(stringType, "v"),
		"base64DecodeBytes": g.newSimpleFuncType(numberType, "s"),
		"base64Decode":      g.newSimpleFuncType(stringType, "s"),
		"md5":               g.newSimpleFuncType(stringType, "s"),

		// JSON Merge Patch

		"mergePatch": g.newSimpleFuncType(anyType, "target", "patch"),

		// Debugging

		"trace": g.newSimpleFuncType(anyType, "str", "rest"),

		// Undocumented
		"manifestJson":     g.newSimpleFuncType(stringType, "value"),
		"objectHasEx":      g.newSimpleFuncType(boolType, "obj", "fname", "hidden"),
		"objectFieldsEx":   g.newSimpleFuncType(arrayOfString, "obj", "hidden"),
		"flatMap":          g.newSimpleFuncType(anyArrayType, "func", "arr"),
		"modulo":           g.newSimpleFuncType(numberType, "x", "y"),
		"slice":            g.newSimpleFuncType(arrayOfString, "indexable", "index", "end", "step"),
		"primitiveEquals":  g.newSimpleFuncType(boolType, "x", "y"),
		"mod":              g.newSimpleFuncType(stringOrNumber, "a", "b"),
		"native":           g.newSimpleFuncType(anyFunctionType, "x"),
		"$objectFlatMerge": g.newSimpleFuncType(anyObjectType, "x"),
	}

	fieldContains := map[string][]placeholderID{}
	for name, t := range fields {
		fieldContains[name] = []placeholderID{t}
	}

	g._placeholders[stdlibType] = concreteTP(TypeDesc{
		ObjectDesc: &objectDesc{
			allFieldsKnown: true,
			allContain:     nil,
			fieldContains:  fieldContains,
		},
	})
}

func NewTypeGraph(varAt map[ast.Node]*common.Variable) *typeGraph {
	g := typeGraph{
		exprPlaceholder: make(map[ast.Node]placeholderID),
		varAt:           varAt,
	}

	anyObjectDesc := &objectDesc{
		allFieldsKnown: false,
		allContain:     []placeholderID{anyType},
	}

	anyFunctionDesc := &functionDesc{
		minArity: 0,
		// Technically on 64-bit system if someone really tries maybe they can
		// go over that and get strange errors. At this point I don't care.
		maxArity:       math.MaxInt32,
		resultContains: []placeholderID{anyType},
	}

	anyArrayDesc := &arrayDesc{
		allContain: []placeholderID{anyType},
	}

	// Create the "no-type" sentinel placeholder
	g.newPlaceholder()

	// any type
	g.newPlaceholder()
	g._placeholders[anyType] = concreteTP(TypeDesc{
		Bool:         true,
		Number:       true,
		String:       true,
		Null:         true,
		FunctionDesc: anyFunctionDesc,
		ObjectDesc:   anyObjectDesc,
		ArrayDesc:    anyArrayDesc,
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
	g._placeholders[anyArrayType] = concreteTP(TypeDesc{
		ArrayDesc: anyArrayDesc,
	})

	g.newPlaceholder()
	g._placeholders[anyObjectType] = concreteTP(TypeDesc{
		ObjectDesc: anyObjectDesc,
	})

	g.newPlaceholder()
	g._placeholders[anyFunctionType] = concreteTP(TypeDesc{
		FunctionDesc: anyFunctionDesc,
	})

	prepareStdlib(&g)

	return &g
}

func (g *typeGraph) AddToGraph(node ast.Node, varAt map[ast.Node]*common.Variable) {
	g.varAt = varAt //  TODO(sbarzowski) hacky! If it changes, we shouldn't put it in typeGraph at all
	prepareTP(node, g)
}

func (g *typeGraph) PrepareTypes(node ast.Node, typeOf ExprTypes, varAt map[ast.Node]*common.Variable) {
	g.AddToGraph(node, varAt)
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
