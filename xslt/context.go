package xslt

import (
	"fmt"
	"slices"

	"github.com/midbel/codecs/environ"
	"github.com/midbel/codecs/xml"
	"github.com/midbel/codecs/xpath"
)

func Catchable(err error) bool {
	return true
}

type Context struct {
	XslNode     xml.Node
	ContextNode xml.Node
	Mode        string

	Index int
	Size  int
	Depth int

	*Stylesheet
	*Env
}

func (c *Context) ApplyTemplate() ([]xml.Node, error) {
	ex, err := c.Match(c.ContextNode, c.Mode)
	if err != nil {
		return nil, err
	}
	return ex.Execute(c)
}

func (c *Context) Find(name, mode string) (Executer, error) {
	return c.Stylesheet.Find(name, c.getMode(mode))
}

func (c *Context) Match(node xml.Node, mode string) (Executer, error) {
	return c.Stylesheet.Match(node, c.getMode(mode))
}

func (c *Context) MatchImport(node xml.Node, mode string) (Executer, error) {
	return c.Stylesheet.MatchImport(node, c.getMode(mode))
}

func (c *Context) getMode(mode string) string {
	switch mode {
	case currentMode:
		return c.Mode
	case defaultMode:
		return c.Stylesheet.DefaultMode
	default:
		return mode
	}
}

func (c *Context) WithNodes(ctxNode, xslNode xml.Node) *Context {
	return c.clone(xslNode, ctxNode)
}

func (c *Context) WithXsl(xslNode xml.Node) *Context {
	return c.clone(xslNode, c.ContextNode)
}

func (c *Context) WithXpath(ctxNode xml.Node) *Context {
	return c.clone(c.XslNode, ctxNode)
}

func (c *Context) WithMode(mode string) *Context {
	child := c.clone(c.XslNode, c.ContextNode)
	child.Mode = mode
	return child
}

func (c *Context) Nest() *Context {
	child := c.clone(c.XslNode, c.ContextNode)
	child.Env = child.Env.Sub()
	return child
}

func (c *Context) Last() *Context {
	x := c.Copy()
	x.Env = x.Env.Unwrap()
	return x
}

func (c *Context) Copy() *Context {
	return c.clone(c.XslNode, c.ContextNode)
}

func (c *Context) clone(xslNode, ctxNode xml.Node) *Context {
	child := Context{
		XslNode:     xslNode,
		ContextNode: ctxNode,
		Mode:        c.Mode,
		Index:       1,
		Size:        1,
		Stylesheet:  c.Stylesheet,
		Env:         c.Env,
		Depth:       c.Depth + 1,
	}
	return &child
}

func (c *Context) errorWithContext(err error) error {
	if c.XslNode == nil {
		return err
	}
	return errorWithContext(c.XslNode.QualifiedName(), err)
}

type Resolver interface {
	Resolve(string) (xpath.Expr, error)
}

type Env struct {
	other    Resolver
	Vars     environ.Environ[xpath.Expr]
	Params   environ.Environ[xpath.Expr]
	Builtins environ.Environ[xpath.BuiltinFunc]
	Depth    int
}

func Empty() *Env {
	return Enclosed(nil)
}

func Enclosed(other Resolver) *Env {
	return &Env{
		other:    other,
		Vars:     environ.Empty[xpath.Expr](),
		Params:   environ.Empty[xpath.Expr](),
		Builtins: xpath.DefaultBuiltin(),
	}
}

func (e *Env) Len() int {
	return e.Vars.Len() + e.Params.Len()
}

func (e *Env) Names() []string {
	return e.localNames()
}

func (e *Env) Sub() *Env {
	return &Env{
		other:    e.other,
		Vars:     environ.Enclosed[xpath.Expr](e.Vars),
		Params:   environ.Enclosed[xpath.Expr](e.Params),
		Builtins: e.Builtins,
		Depth:    e.Depth + 1,
	}
}

func (e *Env) Unwrap() *Env {
	x := &Env{
		other:    e.other,
		Vars:     e.Vars,
		Params:   e.Params,
		Builtins: e.Builtins,
		Depth:    e.Depth,
	}
	if u, ok := x.Vars.(interface {
		Detach() environ.Environ[xpath.Expr]
	}); ok {
		x.Vars = u.Detach()
	}
	if u, ok := x.Params.(interface {
		Detach() environ.Environ[xpath.Expr]
	}); ok {
		x.Params = u.Detach()
	}
	return x
}

func (e *Env) ExecuteQuery(query string, datum xml.Node) (xpath.Sequence, error) {
	return e.ExecuteQueryWithNS(query, "", datum)
}

func (e *Env) ExecuteQueryWithNS(query, namespace string, datum xml.Node) (xpath.Sequence, error) {
	if query == "" {
		i := xpath.NewNodeItem(datum)
		return xpath.Singleton(i), nil
	}
	q, err := e.CompileQueryWithNS(query, namespace)
	if err != nil {
		return nil, err
	}
	return q.Find(datum)
}

func (e *Env) CompileQuery(query string) (xpath.Expr, error) {
	return e.CompileQueryWithNS(query, "")
}

func (e *Env) CompileQueryWithNS(query, namespace string) (xpath.Expr, error) {
	q, err := xpath.Build(query)
	if err != nil {
		return nil, err
	}
	q.Environ = e
	q.Builtins = e.Builtins
	if namespace != "" {
		q.UseNamespace(namespace)
	}
	return q, nil
}

func (e *Env) TestNode(query string, datum xml.Node) (bool, error) {
	seq, err := e.ExecuteQuery(query, datum)
	if err != nil {
		return false, err
	}
	return seq.True(), nil
}

func (e *Env) Merge(other *Env) {
	if m, ok := e.Vars.(interface {
		Merge(environ.Environ[xpath.Expr])
	}); ok && other.Vars.Len() > 0 {
		m.Merge(other.Vars)
	}
	if m, ok := e.Params.(interface {
		Merge(environ.Environ[xpath.Expr])
	}); ok && other.Params.Len() > 0 {
		m.Merge(other.Params)
	}
}

func (e *Env) Resolve(ident string) (xpath.Expr, error) {
	expr, err := e.Vars.Resolve(ident)
	if err == nil {
		return expr, nil
	}
	expr, err = e.Params.Resolve(ident)
	if err == nil {
		return expr, nil
	}
	if e.other != nil {
		return e.other.Resolve(ident)
	}
	return nil, err
}

func (e *Env) Define(ident string, expr xpath.Expr) {
	ok := slices.Contains(e.localNames(), ident)
	if ok {
		return
	}
	e.Vars.Define(ident, expr)
}

func (e *Env) Eval(ident, query string, datum xml.Node) error {
	items, err := e.ExecuteQuery(query, datum)
	if err == nil {
		e.Define(ident, xpath.NewValueFromSequence(items))
	}
	return err
}

func (e *Env) DefineParam(ident, value string) error {
	expr, err := e.CompileQuery(value)
	if err == nil {
		e.DefineExprParam(ident, expr)
	}
	return err
}

func (e *Env) EvalParam(ident, query string, datum xml.Node) error {
	items, err := e.ExecuteQuery(query, datum)
	if err == nil {
		e.DefineExprParam(ident, xpath.NewValueFromSequence(items))
	}
	return err
}

func (e *Env) DefineExprParam(ident string, expr xpath.Expr) {
	ok := slices.Contains(e.localNames(), ident)
	if ok {
		// pass
	}
	e.Params.Define(ident, expr)
}

func (e *Env) localNames() []string {
	return slices.Concat(e.Vars.Names(), e.Params.Names())
}

func errorWithContext(ctx string, err error) error {
	return fmt.Errorf("%s: %w", ctx, err)
}
