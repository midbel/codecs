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

type Function struct {
	xml.QName
	as   xml.QName
	args []Parameter
	body []xpath.Expr
}

func (f Function) Call(ctx xpath.Context, args []xpath.Expr) (xpath.Sequence, error) {
	if len(args) != len(f.args) {
		return nil, fmt.Errorf("invalid number of arguments given")
	}
	env := ctx.Environ
	defer func() {
		ctx.Environ = env
	}()
	ctx.Environ = environ.Enclosed[xpath.Expr](ctx.Environ)
	for i := range f.args {
		e := xpath.As(args[i], f.args[i].as)
		ctx.Environ.Define(f.args[i].name, e)
	}
	is, err := xpath.Call(ctx, f.body)
	if err != nil {
		return nil, err
	}
	if !f.as.Zero() {

	}
	return is, nil
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
	b := NewBuilder()
	return b.Build(r)
}

func (s *Schema) ResolveFunc(name string) (xpath.Callable, error) {
	return s.Funcs.Resolve(name)
}

func (s *Schema) Exec(doc *xml.Document) iter.Seq[Result] {
	return s.ExecContext(context.Background(), doc)
}

func (s *Schema) ExecContext(ctx context.Context, doc *xml.Document) iter.Seq[Result] {
	fn := func(yield func(Result) bool) {
		for _, p := range s.Patterns {
			for r := range p.ExecContext(ctx, doc) {
				ok := yield(r)
				if !ok {
					return
				}
			}
		}
	}
	return fn
}

func (s *Schema) Asserts() iter.Seq[*Assert] {
	fn := func(yield func(*Assert) bool) {
		for _, p := range s.Patterns {
			for a := range p.Asserts() {
				ok := yield(a)
				if !ok {
					return
				}
			}
		}
	}
	return fn
}

type Pattern struct {
	Title string
	Ident string
	environ.Environ[xpath.Expr]
	Funcs environ.Environ[xpath.Callable]

	Rules []*Rule
}

func (p *Pattern) ResolveFunc(name string) (xpath.Callable, error) {
	return p.Funcs.Resolve(name)
}

func (p *Pattern) Exec(doc *xml.Document) iter.Seq[Result] {
	return p.ExecContext(context.Background(), doc)
}

func (p *Pattern) ExecContext(ctx context.Context, doc *xml.Document) iter.Seq[Result] {
	fn := func(yield func(Result) bool) {
		for _, r := range p.Rules {
			for r := range r.ExecContext(ctx, doc) {
				ok := yield(r)
				if !ok {
					return
				}
			}
		}
	}
	return fn
}

func (p *Pattern) Asserts() iter.Seq[*Assert] {
	it := func(yield func(*Assert) bool) {
		for _, r := range p.Rules {
			for _, a := range r.Asserts {
				ok := yield(a)
				if !ok {
					return
				}
			}
		}
	}
	return it
}

type Rule struct {
	environ.Environ[xpath.Expr]
	Funcs environ.Environ[xpath.Callable]

	Title   string
	Context string
	Asserts []*Assert
}

func (r *Rule) ResolveFunc(name string) (xpath.Callable, error) {
	return r.Funcs.Resolve(name)
}

func (r *Rule) Count(doc *xml.Document) (int, error) {
	expr, err := compileContext(r.Context)
	if err != nil {
		return 0, err
	}
	var items []xpath.Item
	if f, ok := expr.(interface {
		FindWithEnv(xml.Node, environ.Environ[xpath.Expr]) (xpath.Sequence, error)
	}); ok {
		items, err = f.FindWithEnv(doc, environ.Enclosed(r))
	} else {
		items, err = expr.Find(doc)
	}
	return len(items), err
}

func (r *Rule) Exec(doc *xml.Document) iter.Seq[Result] {
	return r.ExecContext(context.Background(), doc)
}

func (r *Rule) ExecContext(ctx context.Context, doc *xml.Document) iter.Seq[Result] {
	fn := func(yield func(Result) bool) {
		items, err := r.getItems(doc)
		if err != nil {
			res := Result{
				Ident: "RULE",
				Level: LevelFatal,
				Error: err,
			}
			yield(res)
			return
		}
		for _, a := range r.Asserts {
			if err := ctx.Err(); err != nil {
				res := Result{
					Ident:   a.Ident,
					Level:   LevelFatal,
					Message: "cancel",
					Total:   len(items),
					Error:   err,
				}
				yield(res)
				return
			}
			now := time.Now()
			pass, all, err := a.Eval(ctx, items, r)

			res := Result{
				Ident:   a.Ident,
				Level:   a.Flag,
				Message: a.Message,
				Total:   len(items),
				Pass:    pass,
				Error:   err,
				Items:   all,
				Rule:    r.Context,
				Test:    a.Test,
				Elapsed: time.Since(now),
			}

			ok := yield(res)
			if !ok {
				break
			}
		}
	}
	return fn
}

func (r *Rule) getItems(doc *xml.Document) ([]xpath.Item, error) {
	expr, err := compileContext(r.Context)
	if err != nil {
		return nil, err
	}
	var items []xpath.Item
	if f, ok := expr.(interface {
		FindWithEnv(xml.Node, environ.Environ[xpath.Expr]) (xpath.Sequence, error)
	}); ok {
		items, err = f.FindWithEnv(doc, r)
	} else {
		items, err = expr.Find(doc)
	}
	return items, err
}

type Assert struct {
	Ident   string
	Flag    string
	Test    string
	Message string
}

func (a *Assert) Eval(ctx context.Context, items []xpath.Item, env environ.Environ[xpath.Expr]) (int, []*ResultItem, error) {
	if len(items) == 0 {
		return 0, nil, nil
	}
	test, err := compileExpr(a.Test)
	if err != nil {
		return 0, nil, err
	}
	var (
		pass int
		all  []*ResultItem
	)
	for i := range items {
		if err := ctx.Err(); err != nil {
			return pass, all, err
		}
		ast, ok := items[i].(Asserter)
		if !ok {
			continue
		}
		res, err := ast.Assert(test, env)
		if err != nil {
			return 0, nil, fmt.Errorf("%s (%s)", a.Message, err)
		}
		r := ResultItem{
			Item:    items[i],
			Returns: res,
		}
		if r.Pass = isTrue(res); r.Pass {
			pass++
		}
		all = append(all, &r)
	}
	if pass < len(items) {
		return pass, all, fmt.Errorf("%w: %s", ErrAssert, a.Message)
	}
	return pass, all, nil
}

func isTrue(res []xpath.Item) bool {
	if len(res) == 0 {
		return false
	}
	var ok bool
	if !res[0].Atomic() {
		return true
	}
	switch res := res[0].Value().(type) {
	case bool:
		ok = res
	case float64:
		ok = res != 0
	case string:
		ok = res != ""
	default:
	}
	return ok
}

func compileContext(expr string) (xpath.Expr, error) {
	return xpath.CompileMode(strings.NewReader(expr), xpath.ModeXsl)
}

func compileExpr(expr string) (xpath.Expr, error) {
	return xpath.Compile(strings.NewReader(expr))
}
