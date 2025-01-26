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

	"github.com/midbel/codecs/xml"
)

var ErrAssert = errors.New("assertion error")

const (
	LevelFatal = "fatal"
	LevelWarn  = "warning"
)

type FilterFunc func(*Assert) bool

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
	body []xml.Expr
}

func (f Function) Call(ctx xml.Context, args []xml.Expr) ([]xml.Item, error) {
	if len(args) != len(f.args) {
		return nil, fmt.Errorf("invalid number of arguments given")
	}
	env := ctx.Environ
	defer func() {
		ctx.Environ = env
	}()
	ctx.Environ = xml.Enclosed[xml.Expr](ctx.Environ)
	for i := range f.args {
		ctx.Environ.Define(f.args[i].name, args[i])
	}
	is, err := xml.Call(ctx, f.body)
	if err != nil {
		return nil, err
	}
	if !f.as.Zero() {

	}
	return is, nil
}

type Result struct {
	Ident   string
	Level   string
	Message string
	Total   int
	Pass    int
	Error   error

	Items []xml.Item
	Rule  string
	Test  string

	Elapsed time.Duration
}

func (r Result) Failed() bool {
	return r.Error != nil
}

type Schema struct {
	Title string
	Mode  xml.StepMode
	xml.Environ[xml.Expr]
	Funcs xml.Environ[xml.Callable]

	Phases    []*Phase
	Patterns  []*Pattern
	Spaces    []*Namespace
	Functions []*Function
}

func Default() *Schema {
	s := Schema{
		Environ: xml.Empty[xml.Expr](),
		Funcs:   xml.Empty[xml.Callable](),
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

func (s *Schema) ResolveFunc(name string) (xml.Callable, error) {
	return s.Funcs.Resolve(name)
}

func (s *Schema) Exec(doc *xml.Document, keep FilterFunc) iter.Seq[Result] {
	return s.ExecContext(context.Background(), doc, keep)
}

func (s *Schema) ExecContext(ctx context.Context, doc *xml.Document, keep FilterFunc) iter.Seq[Result] {
	fn := func(yield func(Result) bool) {
		for _, p := range s.Patterns {
			for r := range p.ExecContext(ctx, doc, keep) {
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
	xml.Environ[xml.Expr]
	Funcs xml.Environ[xml.Callable]

	Rules []*Rule
}

func (p *Pattern) ResolveFunc(name string) (xml.Callable, error) {
	return p.Funcs.Resolve(name)
}

func (p *Pattern) Exec(doc *xml.Document, keep FilterFunc) iter.Seq[Result] {
	return p.ExecContext(context.Background(), doc, keep)
}

func (p *Pattern) ExecContext(ctx context.Context, doc *xml.Document, keep FilterFunc) iter.Seq[Result] {
	fn := func(yield func(Result) bool) {
		for _, r := range p.Rules {
			for r := range r.ExecContext(ctx, doc, keep) {
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
	xml.Environ[xml.Expr]
	Funcs xml.Environ[xml.Callable]

	Title   string
	Context string
	Asserts []*Assert
}

func (r *Rule) ResolveFunc(name string) (xml.Callable, error) {
	return r.Funcs.Resolve(name)
}

func (r *Rule) Count(doc *xml.Document) (int, error) {
	expr, err := compileContext(r.Context)
	if err != nil {
		return 0, err
	}
	var items []xml.Item
	if f, ok := expr.(interface {
		FindWithEnv(xml.Node, xml.Environ[xml.Expr]) ([]xml.Item, error)
	}); ok {
		items, err = f.FindWithEnv(doc, xml.Enclosed(r))
	} else {
		items, err = expr.Find(doc)
	}
	return len(items), err
}

func (r *Rule) Exec(doc *xml.Document, keep FilterFunc) iter.Seq[Result] {
	return r.ExecContext(context.Background(), doc, keep)
}

func (r *Rule) ExecContext(ctx context.Context, doc *xml.Document, keep FilterFunc) iter.Seq[Result] {
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
			if ok := keep(a); !ok {
				continue
			}
			now := time.Now()
			pass, err := a.Eval(ctx, items, r)

			res := Result{
				Ident:   a.Ident,
				Level:   a.Flag,
				Message: a.Message,
				Total:   len(items),
				Pass:    pass,
				Error:   err,
				Items:   items,
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

func (r *Rule) getItems(doc *xml.Document) ([]xml.Item, error) {
	expr, err := compileContext(r.Context)
	if err != nil {
		return nil, err
	}
	var items []xml.Item
	if f, ok := expr.(interface {
		FindWithEnv(xml.Node, xml.Environ[xml.Expr]) ([]xml.Item, error)
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

func (a *Assert) Eval(ctx context.Context, items []xml.Item, env xml.Environ[xml.Expr]) (int, error) {
	test, err := compileExpr(a.Test)
	if err != nil {
		return 0, err
	}
	var pass int
	for i := range items {
		if err := ctx.Err(); err != nil {
			return pass, err
		}
		res, err := items[i].Assert(test, env)
		if err != nil {
			return 0, fmt.Errorf("%s (%s)", a.Message, err)
		}
		ok := isTrue(res)
		if !ok {
			return pass, fmt.Errorf("%w: %s", ErrAssert, a.Message)
		}
		pass++
	}
	return pass, nil
}

func isTrue(res []xml.Item) bool {
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

func compileContext(expr string) (xml.Expr, error) {
	return xml.CompileMode(strings.NewReader(expr), xml.ModeXsl)
}

func compileExpr(expr string) (xml.Expr, error) {
	return xml.Compile(strings.NewReader(expr))
}
