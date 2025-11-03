package sch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"strings"
	"time"

	"github.com/midbel/codecs/environ"
	"github.com/midbel/codecs/xml"
	"github.com/midbel/codecs/xpath"
)

var ErrAssert = errors.New("assertion error")

type Asserter interface {
	Assert(xpath.Expr, environ.Environ[xpath.Expr]) (xpath.Sequence, error)
}

const (
	LevelFatal = "fatal"
	LevelWarn  = "warning"
)

type Namespace struct {
	URI    string
	Prefix string
}

type Phase struct {
	Ident   string
	Actives []string
}

type Let struct {
	Ident string
	Value string
}

type Parameter struct {
	name string
	as   xml.QName
}

type ResultItem struct {
	xpath.Item
	Returns []xpath.Item
	Pass    bool
}

type Result struct {
	Ident   string
	Level   string
	Message string
	Total   int
	Pass    int
	Error   error

	Items []*ResultItem
	Rule  string
	Test  string

	Elapsed time.Duration
}

func (r Result) Failed() bool {
	return r.Error != nil
}

type Schema struct {
	Title string
	Mode  xpath.StepMode
	environ.Environ[xpath.Expr]
	Funcs environ.Environ[xpath.Callable]

	Phases    []*Phase
	Patterns  []*Pattern
	Spaces    []*Namespace
	Functions []*Function
}

func Default() *Schema {
	s := Schema{
		Environ: environ.Empty[xpath.Expr](),
		Funcs:   environ.Empty[xpath.Callable](),
	}
	return &s
}

func Open(file string) (*Schema, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return Parse(r)
}

func Parse(r io.Reader) (*Schema, error) {
	return nil, nil
}

type Pattern struct {
	Title string
	Ident string

	Rules []*Rule
}

type Rule struct {
	Title   string
	Context string
	Asserts []*Assert
}

type Assert struct {
	Ident   string
	Flag    string
	Test    string
	Message string
}

func compileContext(expr string) (xpath.Expr, error) {
	return xpath.CompileMode(strings.NewReader(expr), xpath.ModeXsl)
}

func compileExpr(expr string) (xpath.Expr, error) {
	return xpath.Compile(strings.NewReader(expr))
}
