package types

import "strings"

type arrayDesc struct {
	allContain []placeholderID

	elementContains [][]placeholderID
	// TODO(sbarzowski) length when directly available?
}

func (a *arrayDesc) widen(other *arrayDesc) {
	if other == nil {
		return
	}
	a.allContain = append(a.allContain, other.allContain...)
	for i := range other.elementContains {
		if len(a.elementContains) < i {
			a.elementContains = append(a.elementContains, nil)
		}
		a.elementContains[i] = append(a.elementContains[i], other.elementContains[i]...)
	}
}

func (a *arrayDesc) normalize() {
	normalizePlaceholders(a.allContain)
	for index, ps := range a.elementContains {
		a.elementContains[index] = normalizePlaceholders(ps)
	}
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

func (o *objectDesc) normalize() {
	o.allContain = normalizePlaceholders(o.allContain)
	for f, ps := range o.fieldContains {
		o.fieldContains[f] = normalizePlaceholders(ps)
	}
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

func (f *functionDesc) normalize() {
	f.resultContains = normalizePlaceholders(f.resultContains)
}

// TODO(sbarzowski) unexport this
type TypeDesc struct {
	Bool         bool
	Number       bool
	String       bool
	Null         bool
	FunctionDesc *functionDesc
	ObjectDesc   *objectDesc
	ArrayDesc    *arrayDesc
}

func (t *TypeDesc) Any() bool {
	// TODO(sbarzowski) BUG - must check that function, object and array allow any values
	return t.Bool && t.Number && t.String && t.Null && t.Function() && t.Object() && t.Array()
}

func (t *TypeDesc) Void() bool {
	return !t.Bool && !t.Number && !t.String && !t.Null && !t.Function() && !t.Object() && !t.Array()
}

func (t *TypeDesc) Function() bool {
	return t.FunctionDesc != nil
}

func (t *TypeDesc) Object() bool {
	return t.ObjectDesc != nil
}

func (t *TypeDesc) Array() bool {
	return t.ArrayDesc != nil
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
	if t.Array() {
		parts = append(parts, "array")
	}
	return strings.Join(parts, " or ")
}

func (t *TypeDesc) widen(b *TypeDesc) {
	t.Bool = t.Bool || b.Bool
	t.Number = t.Number || b.Number
	t.String = t.String || b.String
	t.Null = t.Null || b.Null

	if t.FunctionDesc != nil {
		t.FunctionDesc.widen(b.FunctionDesc)
	} else if t.FunctionDesc == nil && b.FunctionDesc != nil {
		copy := *b.FunctionDesc
		t.FunctionDesc = &copy
	}

	if t.ObjectDesc != nil {
		t.ObjectDesc.widen(b.ObjectDesc)
	} else if t.ObjectDesc == nil && b.ObjectDesc != nil {
		copy := *b.ObjectDesc
		t.ObjectDesc = &copy
	}

	if t.ArrayDesc != nil {
		t.ArrayDesc.widen(b.ArrayDesc)
	} else if t.ArrayDesc == nil && b.ArrayDesc != nil {
		copy := *b.ArrayDesc
		t.ArrayDesc = &copy
	}
}

func (t *TypeDesc) normalize() {
	if t.ArrayDesc != nil {
		t.ArrayDesc.normalize()
	}
	if t.FunctionDesc != nil {
		t.FunctionDesc.normalize()
	}
	if t.ObjectDesc != nil {
		t.ObjectDesc.normalize()
	}
}

type indexSpec struct {
	indexType indexType

	indexed placeholderID

	// TODO(sbarzowski) this name is ambigous, think of something better or at least document it and make it consistent with helper function names
	stringIndex string
	intIndex    int
}

type indexType int

const (
	genericIndex     = iota
	knownIntIndex    = iota
	knownStringIndex = iota
	functionIndex    = iota
)

func unknownIndexSpec(indexed placeholderID) *indexSpec {
	return &indexSpec{
		indexType:   genericIndex,
		indexed:     indexed,
		stringIndex: "",
	}
}

func knownObjectIndex(indexed placeholderID, index string) *indexSpec {
	return &indexSpec{
		indexType:   knownStringIndex,
		indexed:     indexed,
		stringIndex: index}
}

func functionCallIndex(function placeholderID) *indexSpec {
	return &indexSpec{
		indexType: functionIndex,
		indexed:   function,
	}
}

func arrayIndex(indexed placeholderID, index int) *indexSpec {
	return &indexSpec{
		indexType: knownIntIndex,
		indexed:   indexed,
		intIndex:  index,
	}
}

type elementDesc struct {
	genericIndex placeholderID
	stringIndex  map[string]placeholderID
	intIndex     []placeholderID
	callIndex    placeholderID
}
