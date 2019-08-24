package types

import (
	"fmt"

	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/parser"
)

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

		if !targetType.Array() && !targetType.Object() && !targetType.String {
			ec.staticErr("Indexed value is neither an array nor an object nor a string", node.Loc())
		} else if !targetType.Object() {
			// It's not an object, so it must be an array or a string
			var assumedType string
			if targetType.Array() && targetType.String {
				assumedType = "an array or a string"
			} else if targetType.Array() {
				assumedType = "an array"
			} else {
				assumedType = "a string"
			}
			if !indexType.Number {
				ec.staticErr("Indexed value is assumed to be "+assumedType+", but index is not a number", node.Loc())
			}
		} else if !targetType.Array() {
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
