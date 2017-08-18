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
	"testing"
	"unicode/utf8"
)

// Just some simple sanity tests for now.  Eventually we'll share end-to-end tests with the C++
// implementation but unsure if that should be done here or via some external framework.
// TODO(sbarzowski) figure out how to measure coverage on the external tests

type mainTest struct {
	name      string
	input     string
	golden    string
	errString string
}

var mainTests = []mainTest{
	{"numeric_literal", "100", "100", ""},
	{"boolean_literal", "true", "true", ""},
	{"simple_arith1", "3 + 3", "6", ""},
	{"simple_arith2", "3 + 3 + 3", "9", ""},
	{"simple_arith3", "(3 + 3) + (3 + 3)", "12", ""},
	{"unicode", `"\u263A"`, `"☺"`, ""},
	{"unicode2", `"\u263a"`, `"☺"`, ""},
	{"escaped_single_quote", `"\\'"`, `"\\'"`, ""},
	{"simple_arith_string", "\"aaa\" + \"bbb\"", "\"aaabbb\"", ""},
	{"simple_arith_string2", "\"aaa\" + \"\"", "\"aaa\"", ""},
	{"simple_arith_string3", "\"\" + \"bbb\"", "\"bbb\"", ""},
	{"simple_arith_string_empty", "\"\" + \"\"", "\"\"", ""},
	{"verbatim_string", `@"blah ☺"`, `"blah ☺"`, ""},
	{"empty_array", "[]", "[ ]", ""},
	{"array", "[1, 2, 1 + 2]", "[ 1, 2, 3 ]", ""},
	{"assert", `assert true; true`, "true", ""},
	{"assert", `assert false; true`, "", "RUNTIME ERROR: Assertion failed"},
	{"empty_object", "{}", "{ }", ""},
	{"object", `{"x": 1+1}`, `{ "x": 2 }`, ""},

	{"use_object", `{a: 1}.a`, "1", ""},
	{"use_object_in_object", `{a: {a: 1}.a, b: {b: 1}.b}.a`, "1", ""},
	{"variable", `local x = 2; x`, "2", ""},
	{"variable_not_visible", "local x1 = local nested = 42; nested, x2 = nested; x2", "", "variable_not_visible:1:44-50 Unknown variable: nested"},
	{"array_index1", `[1][0]`, "1", ""},
	{"array_index2", `[1, 2, 3][0]`, "1", ""},
	{"array_index3", `[1, 2, 3][1]`, "2", ""},
	{"array_index4", `[1, 2, 3][2]`, "3", ""},
	{"function", `function() 42`, "", "RUNTIME ERROR: Couldn't manifest function in JSON output."},
	{"function_call", `(function() 42)()`, "42", ""},
	{"function_with_argument", `(function(x) x)(42)`, "42", ""},
	{"function_capturing", `local y = 17; (function(x) y)(42)`, "17", ""},
	{"error", `error "42"`, "", "RUNTIME ERROR: 42"},
	{"filled_thunk", "local x = [1, 2, 3]; x[1] + x[1]", "4", ""},
	{"lazy", `local x = {'x': error "blah"}; x.x`, "", "RUNTIME ERROR: blah"},
	{"lazy", `local x = {'x': error "blah"}, f = function(x) 42, z = x.x; f(x.x)`, "42", ""},
	{"lazy_operator1", `false && error "shouldn't happen"`, "false", ""},
	{"lazy_operator2", `true && error "should happen"`, "", "RUNTIME ERROR: should happen"},

	{"std", `std.nullValue`, "null", ""},
	{"std_in_local", `local x = std.nullValue; x`, "null", ""},
	{"call_number", `42()`, "", "RUNTIME ERROR: Unexpected type number, expected function"},
	{"object_within_object", `{x: {y: 42}}.x.y`, "42", ""},
	{"local_within_nested_object", `local a = 42; {x: {x: a,},}.x.x`, "42", ""},

	{"self", `{x: self.y, y: 42}.x`, "42", ""},

	{"std.length", `std.length([])`, "0", ""},
	{"std.makeArray", `std.makeArray(5, function(x) 42)`, "[ 42, 42, 42, 42, 42 ]", ""},

	{"function_in_object", `local r = {f: function(x) 42};r.f(null)`, "42", ""},
	{"method_call", `local r = {f(a): 42};r.f(null)`, "42", ""},

	{"ifthenelse_true", `if true then 42 else error "no way"`, "42", ""},
	{"ifthenelse_false", `if false then error "no way" else 42`, "42", ""},

	{"ifthen_false", `if false then error "no way"`, "null", ""},
	{"ifthen_false", `if true then 42`, "42", ""},

	{"argcapture_builtin_call", `local r = { f(x): local a = x; std.length(a) }; r.f([1, 2, 3])`, "3", ""},

	{"type", "std.type(42)", `"number"`, ""},

	{"recursive_local", `local f(x) = if x == 0 then 0 else 1 + f(x - 1); f(5)`, "5", ""},

	{"object_sum", `{} + {}`, "{ }", ""},
	{"object_sum2", `{"a": 1} + {"a": 2}`, `{ "a": 2 }`, ""},
	{"object_sum3", `{"a": 1} + {"b": 2}`, `{ "a": 1, "b": 2 }`, ""},

	{"object_super", `{"a": 1} + {"a": 2, b: super.a}`, `{ "a": 2, "b": 1 }`, ""},

	{"object_super_deep", `{"a": 1} + {} + {"a": 2, b: super.a}`, `{ "a": 2, "b": 1 }`, ""},

	{"object_super_within", `{"a": 42, d: 1} + {c: super.d, d: 42} + {"a": 2, b: super.c, d: 42}`,
		`{ "a": 2, "b": 1, "c": 1, "d": 42 }`, ""},

	{"less", `if 2 < 1 then error "x"`, "null", ""},
	{"greater", `if 1 > 2 then error "x"`, "null", ""},
	{"lessEq", `if 2 <= 1 then error "x"`, "null", ""},
	{"lessEq2", `if 2 <= 2 then 42`, "42", ""},
	{"greaterEq", `if 1 >= 2 then error "x"`, "null", ""},
	{"greaterEq2", `if 1 >= 1 then 42`, "42", ""},
	{"binaryNot", `~12345`, "-12346", ""},
	// TODO(sbarzowski) - array comprehension
	// {"array_comp", `[x for x in [1, 2, 3]]`, "[1, 2, 3]", ""},
}

func removeExcessiveWhitespace(s string) string {
	var buf bytes.Buffer
	separated := true
	for i, w := 0, 0; i < len(s); i += w {
		runeValue, width := utf8.DecodeRuneInString(s[i:])
		if runeValue == '\n' || runeValue == ' ' {
			if !separated {
				buf.WriteString(" ")
				separated = true
			}
		} else {
			buf.WriteRune(runeValue)
			separated = false
		}
		w = width
	}
	return buf.String()
}

func TestMain(t *testing.T) {
	for _, test := range mainTests {
		vm := MakeVM()
		output, err := vm.evaluateSnippet(test.name, test.input)
		var errString string
		if err != nil {
			errString = err.Error()
		}
		output = removeExcessiveWhitespace(output)
		errString = removeExcessiveWhitespace(errString)
		if errString != test.errString {
			t.Errorf("%s: error result does not match. got\n\t%+v\nexpected\n\t%+v",
				test.input, errString, test.errString)
		}
		if err == nil && output != test.golden {
			t.Errorf("%s: got\n\t%#v\nexpected\n\t%#v", test.name, output, test.golden)
		}
	}
}

type errorFormattingTest struct {
	name      string
	input     string
	errString string
}

func genericTestErrorMessage(t *testing.T, tests []errorFormattingTest, format func(RuntimeError) string) {
	for _, test := range tests {
		vm := MakeVM()
		output, err := vm.evaluateSnippet(test.name, test.input)
		var errString string
		if err != nil {
			switch typedErr := err.(type) {
			case RuntimeError:
				errString = format(typedErr)
			default:
				t.Errorf("%s: unexpected error: %v", test.name, err)
			}

		}
		if errString != test.errString {
			t.Errorf("%s: error result does not match. got\n\t%+#v\nexpected\n\t%+#v",
				test.name, errString, test.errString)
		}
		if err == nil {
			t.Errorf("%s, Expected error, but execution succeded and the here's the result:\n %v\n", test.name, output)
		}
	}
}

// TODO(sbarzowski) Perhaps we should have just one set of tests with all the variants?
// TODO(sbarzowski) Perhaps this should be handled in external tests?
var oneLineTests = []errorFormattingTest{
	{"error", `error "x"`, "RUNTIME ERROR: x"},
}

func TestOneLineError(t *testing.T) {
	genericTestErrorMessage(t, oneLineTests, func(r RuntimeError) string {
		return r.Error()
	})
}

// TODO(sbarzowski) checking if the whitespace is right is quite unpleasant, what can we do about it?
var minimalErrorTests = []errorFormattingTest{
	{"error", `error "x"`, "RUNTIME ERROR: x\n" +
		"	During evaluation	\n" +
		"	error:1:1-9	<main>\n"}, // TODO(sbarzowski) if seems we have off-by-one in location
	{"error_in_func", `local x(n) = if n == 0 then error "x" else x(n - 1); x(3)`, "RUNTIME ERROR: x\n" +
		"	During evaluation	\n" +
		"	error_in_func:1:54-58	<main>\n" +
		"	error_in_func:1:44-52	function <anonymous>\n" +
		"	error_in_func:1:44-52	function <anonymous>\n" +
		"	error_in_func:1:44-52	function <anonymous>\n" +
		"	error_in_func:1:29-37	function <anonymous>\n" +
		""},
	{"error_in_error", `error (error "x")`, "RUNTIME ERROR: x\n" +
		"	During evaluation	\n" +
		"	error_in_error:1:8-16	<main>\n" +
		""},
}

func TestMinimalError(t *testing.T) {
	formatter := ErrorFormatter{}
	genericTestErrorMessage(t, minimalErrorTests, func(r RuntimeError) string {
		return formatter.format(r)
	})
}
