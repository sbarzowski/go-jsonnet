/*
Copyright 2017 Google Inc. All rights reserved.

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

// Format designed for human consumption. It may change at any time and be impossible
// to parse without going crazy.
func prettyErrorFormat(err *RuntimeError) string {
	return "TODO"
}

// Compatible with C++ version and concise, yet informative.
// Can be easily machine-consumed and is reasonable for humans.
func minimalErrorFormat(err *RuntimeError) string {
	return oneLineErrorFormat(err) + "\n" + buildStackTrace(err.StackTrace)
}

// No stack trace, just the error. Useful for tests and as a base of other formats.
func oneLineErrorFormat(err *RuntimeError) string {
	return "RUNTIME ERROR: " + err.Msg
}

type ErrorFormatter interface {
	formatInternal(err error) string
	formatRuntime(err *RuntimeError) string
	formatStatic(err *StaticError) string
}
