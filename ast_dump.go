/*
Copyright 2016 Google Inc. All rights reserved.

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

package jsonnet

import (
	"bytes"
	"fmt"
)

type dumpSettings struct {
	withLocation   bool
	withFormatting bool
	withStaticInfo bool
}

const indentUnit = "  "

func Dump(a astNode) string {
	var buf bytes.Buffer
	settings := dumpSettings{withLocation: false, withFormatting: true, withStaticInfo: true}
	simpleDump(a, "", &buf, &settings)
	return buf.String()
}

func baseDump(a astNode, curIndent string, buf *bytes.Buffer, settings *dumpSettings) {
	if settings.withLocation {
		fmt.Fprintf(buf, "%vloc: %v\n", curIndent, a.Loc())
	}
	if settings.withStaticInfo {
		fmt.Fprintf(buf, "%vfreeVariables: %v\n", curIndent, a.FreeVariables())
	}
}

func simpleDump(a astNode, curIndent string, buf *bytes.Buffer, settings *dumpSettings) {
	insideIndent := curIndent + indentUnit
	switch ast := a.(type) {
	case *astApply:
		buf.WriteString("Apply {\n")
		fmt.Fprintf(buf, "%vtarget: ", insideIndent)
		simpleDump(ast.target, insideIndent, buf, settings)
		fmt.Fprintf(buf, "%vargs: [\n", insideIndent)
		argIndent := insideIndent + curIndent
		for i, arg := range ast.arguments {
			fmt.Fprintf(buf, "%v%v: ", argIndent, i)
			simpleDump(arg, argIndent, buf, settings)
		}
		fmt.Fprintf(buf, "%v]\n", insideIndent)
	case *astVar:
		buf.WriteString("Binary {\n")
		fmt.Fprintf(buf, "%vid: %v\n", insideIndent, ast.id)
	case *astLiteralNull:
		buf.WriteString("LiteralNull {\n")
	case *astBinary:
		buf.WriteString("Binary {\n")
		fmt.Fprintf(buf, "%vop: %v\n", insideIndent, ast.op)
		fmt.Fprintf(buf, "%vleft: ", insideIndent)
		simpleDump(ast.left, insideIndent, buf, settings)
		fmt.Fprintf(buf, "%vright: ", insideIndent)
		simpleDump(ast.right, insideIndent, buf, settings)

	default:
		panic("Unsupported node type")
	}
	baseDump(a, insideIndent, buf, settings)
	fmt.Fprintf(buf, "%v}\n", curIndent)
}

// func ReflectionDump(a astNode) string {
// 	val := reflect.ValueOf(a).Elem()
// 	fmt.Printf("value %v\n", val)
// 	for i := 0; i < val.NumField(); i++ {
// 		valueField := val.Field(i)
// 		fmt.Printf("valueField %v\n", valueField)
// 		typeField := val.Type().Field(i)
// 		fmt.Printf("typeField %v\n", typeField)
// 		tag := typeField.Tag
// 		fmt.Printf("tag %v\n", tag)
// 		fmt.Printf("Field Name: %s,\t Tag Value: %s\n", typeField.Name, tag.Get(""))
// 	}
// 	return ""
// }

// // TODO(sbarzowski) use standard stringer instead?
// // It's possible it will become more customizable in the future
// // With optional printing of fodders etc.
// // TODO(sbarzowski) make it a part of astNode interface
// // Or use switch-case style instead
// type dumpable interface {
// 	dump() string
// }

// func (a *astLiteralNull) dump() string {
// 	return "null"
// }

// // TODO(sbarzowski) better name
// type dumpableFields map[string]dumpable

// // TODO(sbarzowski) better name
// func dumpFieldsObject(fields dumpableFields) string {
// 	var inside bytes.Buffer
// 	var result bytes.Buffer
// 	result.WriteString("{")
// 	for key, value := range fields {
// 		result.WriteString("\n")
// 		result.WriteString(key)
// 		result.WriteString(": ")
// 		result.WriteString(value.dump())
// 	}
// 	result.WriteString(indent(inside.String()))
// 	result.WriteString("\n}")
// 	return result.String()
// }

// func indent(s string) string {
// 	var buf bytes.Buffer
// 	scanner := bufio.NewScanner(strings.NewReader(s))
// 	for scanner.Scan() {
// 		buf.WriteString(indentUnit)
// 		buf.WriteString(scanner.Text())
// 		buf.WriteString("\n")
// 	}
// 	if err := scanner.Err(); err != nil {
// 		// This really shouldn't happen. The interface accomodates IO errors
// 		// but we are reading from our own buffer.
// 		panic(fmt.Sprintf("Error while indenting string %v", err))
// 	}
// 	return buf.String()
// }

// func dumpMulti(ds dumpableArray) string {
// 	var buf bytes.Buffer
// 	for _, d := range ds {
// 		buf.WriteString(d.dump())
// 		buf.WriteString("\n")
// 	}
// 	return buf.String()
// }

// type dumpableArray []dumpable

// func (arr dumpableArray) dump() string {
// 	var buf bytes.Buffer
// 	buf.WriteString("[")
// 	buf.WriteString(indent(dumpMulti(arr)))
// 	buf.WriteString("]")
// 	return buf.String()
// }

// func (loc *LocationRange) dump() string {
// 	return loc.String()
// }

// func (idents identifiers) dump() string {
// 	var buf bytes.Buffer
// 	for i, ident := range idents {
// 		if i != 0 {
// 			buf.WriteString(", ")
// 		}
// 		buf.WriteString(string(ident))
// 	}
// 	return buf.String()
// }

// func nodeBaseFields(base *astNodeBase) dumpableFields {
// 	return dumpableFields{
// 		"loc":           &base.loc,
// 		"freeVariables": base.freeVariables,
// 	}
// }

// func astNodesToDumpableArray(a astNodes) dumpableArray {
// 	b := make([]dumpable, len(a))
// 	for i := range a {
// 		b[i] = a[i].(dumpable)
// 	}
// 	return b
// }

// func bindsToDumpableArray(a astLocalBinds) dumpableArray {
// 	b := make([]dumpable, len(a))
// 	for i := range a {
// 		b[i] = &a[i]
// 	}
// 	return b
// }

// type dumped string

// func (d dumped) dump() string {
// 	return string(d)
// }

// func dumpedBool(b bool) dumped {
// 	return dumped(strconv.FormatBool(b))
// }

// func (a *astArray) dump() string {
// 	fields := nodeBaseFields(&a.astNodeBase)
// 	fields["elements"] = astNodesToDumpableArray(a.elements)
// 	fields["trailingComma"] = dumpedBool(a.trailingComma)
// 	return dumpFieldsObject(fields)
// }

// func (a *astLocal) dump() string {
// 	fields := nodeBaseFields(&a.astNodeBase)
// 	fields["binds"] = bindsToDumpableArray(a.binds)
// 	fields["body"] = a.body.(dumpable)
// 	return dumpFieldsObject(fields)
// }

// func (b *astLocalBind) dump() string {
// 	return dumpFieldsObject(dumpableFields{
// 		"variable":      dumped(b.variable),
// 		"body":          b.body.(dumpable),
// 		"functionSugar": dumpedBool(b.functionSugar),
// 		"params":        b.params,
// 		"trailingComma": dumpedBool(b.trailingComma),
// 	})
// }

// func (id *identifier) dump() string {
// 	if id == nil {
// 		return "(nil)"
// 	}
// 	return string(*id)
// }

// func (a *astIndex) dump() string {
// 	fields := nodeBaseFields(&a.astNodeBase)
// 	fields["target"] = a.target.(dumpable)
// 	fields["index"] = dumpableOrNil(a.index)
// 	fields["id"] = a.id
// 	return dumpFieldsObject(fields)
// }

// // TODO(sbarzowski) better name
// func dumpableOrNil(a interface{}) dumpable {
// 	if a == nil {
// 		return dumped("(nil)")
// 	}
// 	return a.(dumpable)
// }

// func (a *astObjectField) dump() string {
// 	return dumpFieldsObject(dumpableFields{
// 		"kind":          dumped(a.kind.String()),
// 		"hide":          dumped(a.hide.String()),
// 		"superSugar":    dumpedBool(a.superSugar),
// 		"methodSugar":   dumpedBool(a.methodSugar),
// 		"expr1":         dumpableOrNil(a.expr1),
// 		"id":            a.id,
// 		"ids":           a.ids,
// 		"trailingComma": dumpedBool(a.trailingComma),
// 		"expr2":         dumpableOrNil(a.expr2),
// 		"expr3":         dumpableOrNil(a.expr3),
// 	})
// }

// func astObjectFieldsToDumpableArray(a astObjectFields) dumpableArray {
// 	b := make([]dumpable, len(a))
// 	for i := range a {
// 		b[i] = &a[i]
// 	}
// 	return b
// }

// func (a *astObject) dump() string {
// 	fields := nodeBaseFields(&a.astNodeBase)
// 	fields["fields"] = astObjectFieldsToDumpableArray(a.fields)
// 	fields["trailingComma"] = dumpedBool(a.trailingComma)
// 	return dumpFieldsObject(fields)
// }

// func (a *astLiteralString) dump() string {
// 	fields := nodeBaseFields(&a.astNodeBase)
// 	fields["value"] = dumped(fmt.Sprintf("%#v", a.value))
// 	fields["kind"] = dumped(a.kind.String())
// 	fields["blockIndent"] = dumped(fmt.Sprintf("%#v", a.blockIndent))
// 	return dumpFieldsObject(fields)
// }

// func (a *astError) dump() string {
// 	fields := nodeBaseFields(&a.astNodeBase)
// 	fields["expr"] = a.expr.(dumpable)
// 	return dumpFieldsObject(fields)
// }

// func (a *astVar) dump() string {
// 	fields := nodeBaseFields(&a.astNodeBase)
// 	fields["id"] = &a.id
// 	return dumpFieldsObject(fields)
// }

// type dumper struct {
// 	indentationLevel int
// 	buffer           bytes.Buffer
// }

// func (d *dumper) indent() {
// 	for i := 0; i < d.indentationLevel; i++ {
// 		d.buffer.WriteString(" ")
// 	}
// }

// func (d *dumper) startField(fieldName string) {
// 	d.indentationLevel += indentSize
// 	d.indent()
// 	d.buffer.WriteString(fmt.Sprintf("%v :", fieldName))
// }

// func (d *dumper) endField() {
// 	d.buffer.WriteString("\n")
// 	d.indentationLevel -= indentSize
// }

// func (d *dumper) startArray() {
// 	d.buffer.WriteString("[\n")
// }

// func (d *dumper) endArray() {
// 	// assume clean newline
// 	d.indent()
// 	d.buffer.WriteString("]")
// }

// func (d *dumper) nextInArray() {
// 	d.WriteString("\n")
// 	d.indent()
// }

// func (d *dumper) startObject(astNode astNode) {
// 	d.buffer.WriteString(fmt.Sprintf("%T", reflect.TypeOf(astNode)))
// 	d.buffer.WriteString("{\n")
// }

// func (d *dumper) endObject() {
// 	// assume clean newline
// 	d.indent()
// 	d.buffer.WriteString("}")
// }

// func dumpNode(a astNode, d *dumper) {
// 	d.startObject(a)
// 	dumpInside(a, d)
// 	d.endObject()
// }

// func dumpSimpleField(fieldName string, content string, d *dumper) {
// 	d.startField(fieldName)
// 	d.buffer.WriteString(content)
// 	d.endField()
// }

// func dumpAstField(fieldName string, a astNode, d *dumper) {
// 	d.startField(fieldName)
// 	dumpNode(a, d)
// 	d.endField()
// }

// func dumpMultiAstField(fieldName string, nodes astNodes, d *dumper) {
// 	d.startField(fieldName)
// 	d.startArray()
// 	for i, node := range nodes {
// 		if i != 0 {
// 			d.nextInArray()
// 		}
// 		dumpNode(node, d)
// 	}
// 	d.endArray()
// 	d.endField()
// }

// func identifiersToString(idents identifiers) string {
// 	var buf bytes.Buffer
// 	for i, ident := range idents {
// 		if i != 0 {
// 			buf.WriteString(", ")
// 		}
// 		buf.WriteString(string(ident))
// 	}
// 	return buf.String()
// }

// func dumpInside(a astNode, d *dumper) {
// 	// TODO(sbarzowski): Remove all uses of unimplErr.
// 	unimplErr := makeStaticError(fmt.Sprintf("Desugarer does not yet implement ast: %s", reflect.TypeOf(a)), *a.Loc())

// 	dumpSimpleField("loc", a.Loc().String(), d)
// 	dumpSimpleField("freeVariables", identifiersToString(a.FreeVariables()), d)
// 	switch ast := a.(type) {
// 	case *astApply:
// 		err = desugar(&ast.target, objLevel)
// 		if err != nil {
// 			return
// 		}
// 		for i := range ast.arguments {
// 			err = desugar(&ast.arguments[i], objLevel)
// 			if err != nil {
// 				return
// 			}
// 		}

// 	case *astApplyBrace:
// 		err = desugar(&ast.left, objLevel)
// 		if err != nil {
// 			return
// 		}
// 		err = desugar(&ast.right, objLevel)
// 		if err != nil {
// 			return
// 		}
// 		*astPtr = &astBinary{
// 			astNodeBase: ast.astNodeBase,
// 			left:        ast.left,
// 			op:          bopPlus,
// 			right:       ast.right,
// 		}

// 	case *astArray:
// 		for i := range ast.elements {
// 			err = desugar(&ast.elements[i], objLevel)
// 			if err != nil {
// 				return
// 			}
// 		}

// 	case *astArrayComp:
// 		return desugarArrayComp(ast, objLevel)

// 	case *astAssert:
// 		return unimplErr

// 	case *astBinary:
// 		err = desugar(&ast.left, objLevel)
// 		if err != nil {
// 			return
// 		}
// 		err = desugar(&ast.right, objLevel)
// 		if err != nil {
// 			return
// 		}
// 		// TODO(dcunnin): Need to handle bopPercent, bopManifestUnequal, bopManifestEqual

// 	case *astBuiltin:
// 		// Nothing to do.

// 	case *astConditional:
// 		err = desugar(&ast.cond, objLevel)
// 		if err != nil {
// 			return
// 		}
// 		err = desugar(&ast.branchTrue, objLevel)
// 		if err != nil {
// 			return
// 		}
// 		if ast.branchFalse != nil {
// 			ast.branchFalse = &astLiteralNull{}
// 		}

// 	case *astDollar:
// 		if objLevel == 0 {
// 			return makeStaticError("No top-level object found.", *ast.Loc())
// 		}
// 		*astPtr = &astVar{astNodeBase: ast.astNodeBase, id: identifier("$")}

// 	case *astError:
// 		err = desugar(&ast.expr, objLevel)
// 		if err != nil {
// 			return
// 		}

// 	case *astFunction:
// 		err = desugar(&ast.body, objLevel)
// 		if err != nil {
// 			return
// 		}

// 	case *astImport:
// 		// Nothing to do.

// 	case *astImportStr:
// 		// Nothing to do.

// 	case *astIndex:
// 		err = desugar(&ast.target, objLevel)
// 		if err != nil {
// 			return
// 		}
// 		if ast.id != nil {
// 			if ast.index != nil {
// 				panic("TODO")
// 			}
// 			ast.index = makeStr(string(*ast.id))
// 			ast.id = nil
// 		}
// 		err = desugar(&ast.index, objLevel)
// 		if err != nil {
// 			return
// 		}

// 	case *astLocal:
// 		for _, bind := range ast.binds {
// 			err = desugar(&bind.body, objLevel)
// 			if err != nil {
// 				return
// 			}
// 		}
// 		err = desugar(&ast.body, objLevel)
// 		if err != nil {
// 			return
// 		}
// 		// TODO(dcunnin): Desugar local functions

// 	case *astLiteralBoolean:
// 		// Nothing to do.

// 	case *astLiteralNull:
// 		// Nothing to do.

// 	case *astLiteralNumber:
// 		// Nothing to do.

// 	case *astLiteralString:
// 		unescaped, err := stringUnescape(ast.Loc(), ast.value)
// 		if err != nil {
// 			return err
// 		}
// 		ast.value = unescaped
// 		ast.kind = astStringDouble
// 		ast.blockIndent = ""

// 	case *astObject:
// 		// Hidden variable to allow $ binding.
// 		if objLevel == 0 {
// 			dollar := identifier("$")
// 			ast.fields = append(ast.fields, astObjectFieldLocalNoMethod(&dollar, &astSelf{}))
// 		}

// 		err = desugarFields(*ast.Loc(), &ast.fields, objLevel)
// 		if err != nil {
// 			return
// 		}

// 		fmt.Printf("??????????\n")

// 		var newFields astDesugaredObjectFields
// 		var newAsserts astNodes

// 		for _, field := range ast.fields {
// 			if field.kind == astObjectAssert {
// 				newAsserts = append(newAsserts, field.expr2)
// 			} else if field.kind == astObjectFieldExpr {
// 				newFields = append(newFields, astDesugaredObjectField{field.hide, field.expr1, field.expr2})
// 			} else {
// 				return fmt.Errorf("INTERNAL ERROR: field should have been desugared: %s", field.kind)
// 			}
// 		}

// 		*astPtr = &astDesugaredObject{ast.astNodeBase, newAsserts, newFields}

// 	case *astDesugaredObject:
// 		return unimplErr

// 	case *astObjectComp:
// 		return unimplErr

// 	case *astObjectComprehensionSimple:
// 		return unimplErr

// 	case *astSelf:
// 		// Nothing to do.

// 	case *astSuperIndex:
// 		return unimplErr

// 	case *astUnary:
// 		err = desugar(&ast.expr, objLevel)
// 		if err != nil {
// 			return
// 		}

// 	case *astVar:
// 		// Nothing to do.

// 	default:
// 		return makeStaticError(fmt.Sprintf("Desugarer does not recognize ast: %s", reflect.TypeOf(ast)), *ast.Loc())
// 	}

// 	return nil
// }

// func dump(a astNode) string {
// 	var d dumper
// 	dumpNode(a, d)
// 	return d.buffer.String()
// }
