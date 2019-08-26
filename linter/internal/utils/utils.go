package utils

import (
	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/parser"
)

type ErrCollector struct {
	Errs []parser.StaticError
}

func (ec *ErrCollector) Collect(err parser.StaticError) {
	ec.Errs = append(ec.Errs, err)
}

func (ec *ErrCollector) StaticErr(msg string, loc *ast.LocationRange) {
	ec.Collect(parser.MakeStaticError(msg, *loc))
}
