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
	"math"
	"path"
	"reflect"
	"sort"

	"github.com/google/go-jsonnet/ast"
)

// TODO(sbarzowski) use it as a pointer in most places b/c it can sometimes be shared
// for example it can be shared between array elements and function arguments
type environment struct {
	sb selfBinding

	// Bindings introduced in this frame. The way previous bindings are treated
	// depends on the type of a frame.
	// If isCall == true then previous bindings are ignored (it's a clean
	// environment with just the variables we have here).
	// If isCall == false then if this frame doesn't contain a binding
	// previous bindings will be used.
	upValues bindingFrame
}

func makeEnvironment(upValues bindingFrame, sb selfBinding) environment {
	return environment{
		upValues: upValues,
		sb:       sb,
	}
}

func callFrameToTraceFrame(frame *callFrame) TraceFrame {
	return traceElementToTraceFrame(frame.trace)
}

func (i *interpreter) getCurrentStackTrace(additional *TraceElement) []TraceFrame {
	var result []TraceFrame
	for _, f := range i.stack.stack {
		if f.isCall {
			result = append(result, callFrameToTraceFrame(f))
		}
	}
	if additional != nil {
		result = append(result, traceElementToTraceFrame(additional))
	}
	return result
}

type callFrame struct {
	// True if it switches to a clean environment (function call or array element)
	// False otherwise, e.g. for local
	// This makes callFrame a misnomer as it is technically not always a call...
	isCall bool

	// Tracing information about the place where it was called from.
	trace *TraceElement

	/** Reuse this stack frame for the purpose of tail call optimization. */
	tailCall bool // TODO what is it?

	env environment
}

func dumpCallFrame(c *callFrame) string {
	return fmt.Sprintf("<callFrame isCall = %t location = %v tailCall = %t>",
		c.isCall,
		*c.trace.loc,
		c.tailCall,
	)
}

type callStack struct {
	calls int
	limit int
	stack []*callFrame
}

func dumpCallStack(c *callStack) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "<callStack calls = %d limit = %d stack:\n", c.calls, c.limit)
	for _, callFrame := range c.stack {
		fmt.Fprintf(&buf, "  %v\n", dumpCallFrame(callFrame))
	}
	buf.WriteString("\n>")
	return buf.String()
}

func (s *callStack) top() *callFrame {
	return s.stack[len(s.stack)-1]
}

func (s *callStack) pop() {
	if s.top().isCall {
		s.calls--
	}
	s.stack = s.stack[:len(s.stack)-1]
}

// TODO(sbarzowski) I don't get this. When we have a tail call why can't we just
// pop the last call from stack before pushing our new thing.
// https://github.com/google/go-jsonnet/pull/24#pullrequestreview-58524217
/** If there is a tailstrict annotated frame followed by some locals, pop them all. */
func (s *callStack) tailCallTrimStack() {
	for i := len(s.stack) - 1; i >= 0; i-- {
		if s.stack[i].isCall {
			if !s.stack[i].tailCall { // TODO(sbarzowski) we may need to check some more stuff
				return
			}
			// Remove this stack frame and everything above it
			s.stack = s.stack[:i]
			s.calls--
			return
		}
	}
}

func (i *interpreter) newCall(trace *TraceElement, env environment) error {
	s := &i.stack
	s.tailCallTrimStack()
	if s.calls >= s.limit {
		// TODO(sbarzowski) add tracing information
		return makeRuntimeError("Max stack frames exceeded.", i.getCurrentStackTrace(trace))
	}
	s.stack = append(s.stack, &callFrame{
		isCall:   true,
		trace:    trace,
		env:      env,
		tailCall: false,
	})
	s.calls++
	return nil
}

func (i *interpreter) newLocal(vars bindingFrame) {
	s := &i.stack
	s.stack = append(s.stack, &callFrame{
		env: makeEnvironment(vars, selfBinding{}),
	})
}

// getSelfBinding resolves the self construct
func (s *callStack) getSelfBinding() selfBinding {
	for i := len(s.stack) - 1; i >= 0; i-- {
		if s.stack[i].isCall {
			return s.stack[i].env.sb
		}
	}
	panic(fmt.Sprintf("malformed stack %v", dumpCallStack(s)))
}

// lookUpVar finds for the closest variable in scope that matches the given name.
func (s *callStack) lookUpVar(id ast.Identifier) potentialValue {
	for i := len(s.stack) - 1; i >= 0; i-- {
		bind := s.stack[i].env.upValues[id]
		if bind != nil {
			return bind
		}
		if s.stack[i].isCall {
			// Nothing beyond the captured environment of the thunk / closure.
			break
		}
	}
	return nil
}

func makeCallStack(limit int) callStack {
	return callStack{
		calls: 0,
		limit: limit,
	}
}

// TODO(dcunnin): Add multi output.

// Keeps current execution context and evaluates things
type interpreter struct {
	// Current stack. It is used for:
	// 1) Keeping environment (object we're in, variables)
	// 2) Diagnostic information in case of failure
	stack callStack

	// External variables
	extVars map[ast.Identifier]potentialValue

	// A part of std object common to all files
	baseStd valueObject

	// Keeps imports
	importCache *ImportCache
}

// Build a binding frame containing specified variables.
func (i *interpreter) capture(freeVars ast.Identifiers) bindingFrame {
	env := make(bindingFrame)
	for _, fv := range freeVars {
		env[fv] = i.stack.lookUpVar(fv)
		if env[fv] == nil {
			panic(fmt.Sprintf("Variable %v vanished", fv))
		}
	}
	return env
}

func addBindings(a, b bindingFrame) bindingFrame {
	result := make(bindingFrame)

	for k, v := range a {
		result[k] = v
	}

	for k, v := range b {
		result[k] = v
	}

	return result
}

func (i *interpreter) getCurrentEnv(ast ast.Node) environment {
	return makeEnvironment(
		i.capture(ast.FreeVariables()),
		i.stack.getSelfBinding(),
	)
}

func (i *interpreter) evaluate(a ast.Node) (value, error) {
	e := &evaluator{
		trace: &TraceElement{
			loc:     a.Loc(),
			context: a.Context(),
		},
		i: i,
	}

	switch ast := a.(type) {
	case *ast.Array:
		sb := i.stack.getSelfBinding()
		var elements []potentialValue
		for _, el := range ast.Elements {
			env := makeEnvironment(i.capture(el.FreeVariables()), sb)
			elThunk := makeThunk(env, el)
			elements = append(elements, elThunk)
		}
		return makeValueArray(elements), nil

	case *ast.Binary:
		// Some binary operators are lazy, so thunks are needed in general
		env := i.getCurrentEnv(ast)
		// TODO(sbarzowski) make sure it displays nicely in stack trace (thunk names etc.)
		// TODO(sbarzowski) it may make sense not to show a line in stack trace for operators
		// 					at all in many cases. 1 + 2 + 3 + 4 + error "x" will show 5 lines
		//					of stack trace now, and it's not that nice.
		left := makeThunk(env, ast.Left)
		right := makeThunk(env, ast.Right)

		builtin := bopBuiltins[ast.Op]

		result, err := builtin.function(e, left, right)
		if err != nil {
			return nil, err
		}
		return result, nil

	case *ast.Unary:
		env := i.getCurrentEnv(ast)
		arg := makeThunk(env, ast.Expr)

		builtin := uopBuiltins[ast.Op]

		result, err := builtin.function(e, arg)
		if err != nil {
			return nil, err
		}
		return result, nil

	case *ast.Conditional:
		cond, err := e.evalInCurrentContext(ast.Cond)
		if err != nil {
			return nil, err
		}
		condBool, err := e.getBoolean(cond)
		if err != nil {
			return nil, err
		}
		if condBool.value {
			return e.evalInCurrentContext(ast.BranchTrue)
		}
		return e.evalInCurrentContext(ast.BranchFalse)

	case *ast.DesugaredObject:
		// Evaluate all the field names.  Check for null, dups, etc.
		fields := make(valueSimpleObjectFieldMap)
		for _, field := range ast.Fields {
			fieldNameValue, err := e.evalInCurrentContext(field.Name)
			if err != nil {
				return nil, err
			}
			var fieldName string
			switch fieldNameValue := fieldNameValue.(type) {
			case *valueString:
				fieldName = fieldNameValue.getString()
			case *valueNull:
				// Omitted field.
				continue
			default:
				return nil, e.Error(fmt.Sprintf("Field name must be string, got %v", fieldNameValue.typename()))
			}

			if _, ok := fields[fieldName]; ok {
				return nil, e.Error(duplicateFieldNameErrMsg(fieldName))
			}
			var f unboundField = &codeUnboundField{field.Body}
			if field.PlusSuper {
				f = &PlusSuperUnboundField{f}
			}
			fields[fieldName] = valueSimpleObjectField{field.Hide, f}
		}
		var asserts []unboundField
		for _, assert := range ast.Asserts {
			asserts = append(asserts, &codeUnboundField{assert})
		}
		upValues := i.capture(ast.FreeVariables())
		return makeValueSimpleObject(upValues, fields, asserts), nil

	case *ast.Error:
		msgVal, err := e.evalInCurrentContext(ast.Expr)
		if err != nil {
			// error when evaluating error message
			return nil, err
		}
		msg, err := e.getString(msgVal)
		if err != nil {
			return nil, err
		}
		return nil, e.Error(msg.getString())

	case *ast.Index:
		targetValue, err := e.evalInCurrentContext(ast.Target)
		if err != nil {
			return nil, err
		}
		index, err := e.evalInCurrentContext(ast.Index)
		if err != nil {
			return nil, err
		}
		switch target := targetValue.(type) {
		// TODO(sbarzowski) better error handling if bad index type
		case valueObject:
			indexString := index.(*valueString).getString()
			return target.index(e, indexString)
		case *valueArray:
			indexInt := int(index.(*valueNumber).value)
			return e.evaluate(target.elements[indexInt])
		case *valueString:
			indexInt := int(index.(*valueNumber).value)
			return target.index(e, indexInt)
		}

		return nil, e.Error(fmt.Sprintf("Value non indexable: %v", reflect.TypeOf(targetValue)))

	case *ast.Import:
		codeDir := path.Dir(ast.Loc().FileName)
		return i.importCache.ImportCode(codeDir, ast.File.Value, e)

	case *ast.ImportStr:
		codeDir := path.Dir(ast.Loc().FileName)
		return i.importCache.ImportString(codeDir, ast.File.Value, e)

	case *ast.LiteralBoolean:
		return makeValueBoolean(ast.Value), nil

	case *ast.LiteralNull:
		return makeValueNull(), nil

	case *ast.LiteralNumber:
		return makeValueNumber(ast.Value), nil

	case *ast.LiteralString:
		return makeValueString(ast.Value), nil

	case *ast.Local:
		vars := make(bindingFrame)
		bindEnv := i.getCurrentEnv(a)
		for _, bind := range ast.Binds {
			th := makeThunk(bindEnv, bind.Body)

			// recursive locals
			vars[bind.Variable] = th
			bindEnv.upValues[bind.Variable] = th
		}
		i.newLocal(vars)
		// Add new stack frame, with new thunk for this variable
		// execute body WRT stack frame.
		v, err := e.evalInCurrentContext(ast.Body)
		i.stack.pop()
		return v, err

	case *ast.Self:
		sb := i.stack.getSelfBinding()
		return sb.self, nil

	case *ast.Var:
		return e.evaluate(e.lookUpVar(ast.Id))

	case *ast.SuperIndex:
		index, err := e.evalInCurrentContext(ast.Index)
		if err != nil {
			return nil, err
		}
		indexStr, err := e.getString(index)
		if err != nil {
			return nil, err
		}
		return objectIndex(e, i.stack.getSelfBinding().super(), indexStr.getString())

	case *ast.InSuper:
		index, err := e.evalInCurrentContext(ast.Index)
		if err != nil {
			return nil, err
		}
		indexStr, err := e.getString(index)
		if err != nil {
			return nil, err
		}
		field := tryObjectIndex(i.stack.getSelfBinding().super(), indexStr.getString(), withHidden)
		return makeValueBoolean(field != nil), nil

	case *ast.Function:
		return &valueFunction{
			ec: makeClosure(i.getCurrentEnv(a), ast),
		}, nil

	case *ast.Apply:
		// Eval target
		target, err := e.evalInCurrentContext(ast.Target)
		if err != nil {
			return nil, err
		}
		function, err := e.getFunction(target)
		if err != nil {
			return nil, err
		}

		// environment in which we can evaluate arguments
		argEnv := i.getCurrentEnv(a)

		arguments := callArguments{
			positional: make([]potentialValue, len(ast.Arguments.Positional)),
		}
		for i, arg := range ast.Arguments.Positional {
			arguments.positional[i] = makeThunk(argEnv, arg)
		}

		return e.evaluate(function.call(arguments))

	default:
		return nil, e.Error(fmt.Sprintf("Executing this AST type not implemented yet: %v", reflect.TypeOf(a)))
	}
}

// unparseString Wraps in "" and escapes stuff to make the string JSON-compliant and human-readable.
func unparseString(v string) string {
	var buf bytes.Buffer
	buf.WriteString("\"")
	for _, c := range v {
		switch c {
		case '"':
			buf.WriteString("\\\"")
		case '\\':
			buf.WriteString("\\\\")
		case '\b':
			buf.WriteString("\\b")
		case '\f':
			buf.WriteString("\\f")
		case '\n':
			buf.WriteString("\\n")
		case '\r':
			buf.WriteString("\\r")
		case '\t':
			buf.WriteString("\\t")
		case 0:
			buf.WriteString("\\u0000")
		default:
			if c < 0x20 || (c >= 0x7f && c <= 0x9f) {
				buf.WriteString(fmt.Sprintf("\\u%04x", int(c)))
			} else {
				buf.WriteRune(c)
			}
		}
	}
	buf.WriteString("\"")
	return buf.String()
}

func unparseNumber(v float64) string {
	if v == math.Floor(v) {
		return fmt.Sprintf("%.0f", v)
	}

	// See "What Every Computer Scientist Should Know About Floating-Point Arithmetic"
	// Theorem 15
	// http://docs.oracle.com/cd/E19957-01/806-3568/ncg_goldberg.html
	return fmt.Sprintf("%.17g", v)
}

// manifestJSON converts to standard JSON representation as in "encoding/json" package
func (i *interpreter) manifestJSON(trace *TraceElement, v value) (interface{}, error) {
	e := &evaluator{i: i, trace: trace}
	switch v := v.(type) {

	case *valueBoolean:
		return v.value, nil

	case *valueFunction:
		return nil, makeRuntimeError("Couldn't manifest function in JSON output.", i.getCurrentStackTrace(trace))

	case *valueNumber:
		return v.value, nil

	case *valueString:
		return v.getString(), nil

	case *valueNull:
		return nil, nil

	case *valueArray:
		result := make([]interface{}, 0, len(v.elements))
		for _, th := range v.elements {
			elVal, err := th.getValue(i, trace) // TODO(sbarzowski) perhaps manifestJSON should just take potentialValue
			if err != nil {
				return nil, err
			}
			elem, err := i.manifestJSON(trace, elVal)
			if err != nil {
				return nil, err
			}
			result = append(result, elem)
		}
		return result, nil

	case valueObject:
		fieldNames := objectFields(v, withoutHidden)
		sort.Strings(fieldNames)

		err := checkAssertions(e, v)
		if err != nil {
			return nil, err
		}

		result := make(map[string]interface{})

		for _, fieldName := range fieldNames {
			fieldVal, err := v.index(e, fieldName)
			if err != nil {
				return nil, err
			}

			field, err := i.manifestJSON(trace, fieldVal)
			if err != nil {
				return nil, err
			}
			result[fieldName] = field
		}

		return result, nil

	default:
		return nil, makeRuntimeError(
			fmt.Sprintf("Manifesting this value not implemented yet: %s", reflect.TypeOf(v)),
			i.getCurrentStackTrace(trace),
		)

	}
}

func serializeJSON(v interface{}, multiline bool, indent string, buf *bytes.Buffer) {
	switch v := v.(type) {
	case nil:
		buf.WriteString("null")

	case []interface{}:
		if len(v) == 0 {
			buf.WriteString("[ ]")
		} else {
			var prefix string
			var indent2 string
			if multiline {
				prefix = "[\n"
				indent2 = indent + "   "
			} else {
				prefix = "["
				indent2 = indent
			}
			for _, elem := range v {
				buf.WriteString(prefix)
				buf.WriteString(indent2)
				serializeJSON(elem, multiline, indent2, buf)
				if multiline {
					prefix = ",\n"
				} else {
					prefix = ", "
				}
			}
			if multiline {
				buf.WriteString("\n")
			}
			buf.WriteString(indent)
			buf.WriteString("]")
		}

	case bool:
		if v {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}

	case float64:
		buf.WriteString(unparseNumber(v))

	case map[string]interface{}:
		fieldNames := make([]string, 0, len(v))
		for name := range v {
			fieldNames = append(fieldNames, name)
		}
		sort.Strings(fieldNames)

		if len(fieldNames) == 0 {
			buf.WriteString("{ }")
		} else {
			var prefix string
			var indent2 string
			if multiline {
				prefix = "{\n"
				indent2 = indent + "   "
			} else {
				prefix = "{"
				indent2 = indent
			}
			for _, fieldName := range fieldNames {
				fieldVal := v[fieldName]

				buf.WriteString(prefix)
				buf.WriteString(indent2)

				buf.WriteString(unparseString(fieldName))
				buf.WriteString(": ")

				serializeJSON(fieldVal, multiline, indent2, buf)

				if multiline {
					prefix = ",\n"
				} else {
					prefix = ", "
				}
			}

			if multiline {
				buf.WriteString("\n")
			}
			buf.WriteString(indent)
			buf.WriteString("}")
		}

	case string:
		buf.WriteString(unparseString(v))

	default:
		panic(fmt.Sprintf("Unsupported value for serialization %#+v", v))
	}
}

func (i *interpreter) manifestAndSerializeJSON(trace *TraceElement, v value, multiline bool, indent string) (string, error) {
	var buf bytes.Buffer
	manifested, err := i.manifestJSON(trace, v)
	if err != nil {
		return "", err
	}
	serializeJSON(manifested, multiline, indent, &buf)
	return buf.String(), nil
}

func (i *interpreter) EvalInCleanEnv(fromWhere *TraceElement, env *environment, ast ast.Node) (value, error) {
	err := i.newCall(fromWhere, *env)
	if err != nil {
		return nil, err
	}
	val, err := i.evaluate(ast)
	i.stack.pop()
	return val, err
}

func buildStdObject(i *interpreter) (valueObject, error) {
	objVal, err := evaluateStd(i)
	if err != nil {
		return nil, err
	}
	obj := objVal.(*valueSimpleObject)
	builtinFields := map[string]unboundField{}
	for key, ec := range funcBuiltins {
		function := valueFunction{ec: ec} // TODO(sbarzowski) better way to build function value
		builtinFields[key] = &readyValue{&function}
	}

	for name, value := range builtinFields {
		obj.fields[name] = valueSimpleObjectField{ast.ObjectFieldHidden, value}
	}
	return obj, nil
}

func evaluateStd(i *interpreter) (value, error) {
	beforeStdEnv := makeEnvironment(
		bindingFrame{},
		makeUnboundSelfBinding(),
	)
	evalLoc := ast.MakeLocationRangeMessage("During evaluation of std")
	evalTrace := &TraceElement{loc: &evalLoc}
	node, err := snippetToAST("<std>", getStdCode())
	if err != nil {
		return nil, err
	}
	return i.EvalInCleanEnv(evalTrace, &beforeStdEnv, node)
}

func prepareExtVars(i *interpreter, ext vmExtMap) map[ast.Identifier]potentialValue {
	result := make(map[ast.Identifier]potentialValue)
	for name, content := range ext {
		if content.isCode {
			varLoc := ast.MakeLocationRangeMessage("During evaluation")
			varTrace := &TraceElement{
				loc: &varLoc,
			}
			e := &evaluator{
				i:     i,
				trace: varTrace,
			}
			result[ast.Identifier(name)] = codeToPV(e, "<extvar:"+name+">", content.value)
		} else {
			result[ast.Identifier(name)] = &readyValue{makeValueString(content.value)}
		}
	}
	return result
}

func buildObject(hide ast.ObjectFieldHide, fields map[string]value) valueObject {
	fieldMap := valueSimpleObjectFieldMap{}
	for name, v := range fields {
		fieldMap[name] = valueSimpleObjectField{hide, &readyValue{v}}
	}
	return makeValueSimpleObject(bindingFrame{}, fieldMap, nil)
}

func buildInterpreter(ext vmExtMap, maxStack int, importer Importer) (*interpreter, error) {
	i := interpreter{
		stack:       makeCallStack(maxStack),
		importCache: MakeImportCache(importer),
	}

	stdObj, err := buildStdObject(&i)
	if err != nil {
		return nil, err
	}

	i.baseStd = stdObj

	i.extVars = prepareExtVars(&i, ext)

	return &i, nil
}

func makeInitialEnv(filename string, baseStd valueObject) environment {
	fileSpecific := buildObject(ast.ObjectFieldHidden, map[string]value{
		"thisFile": makeValueString(filename),
	})
	return makeEnvironment(
		bindingFrame{
			"std": &readyValue{makeValueExtendedObject(baseStd, fileSpecific)},
		},
		makeUnboundSelfBinding(),
	)
}

func evaluate(node ast.Node, ext vmExtMap, maxStack int, importer Importer) (string, error) {
	i, err := buildInterpreter(ext, maxStack, importer)
	if err != nil {
		return "", err
	}
	evalLoc := ast.MakeLocationRangeMessage("During evaluation")
	evalTrace := &TraceElement{
		loc: &evalLoc,
	}
	env := makeInitialEnv(node.Loc().FileName, i.baseStd)
	result, err := i.EvalInCleanEnv(evalTrace, &env, node)
	if err != nil {
		return "", err
	}
	manifestationLoc := ast.MakeLocationRangeMessage("During manifestation")
	manifestationTrace := &TraceElement{
		loc: &manifestationLoc,
	}
	s, err := i.manifestAndSerializeJSON(manifestationTrace, result, true, "")
	if err != nil {
		return "", err
	}
	return s, nil
}
