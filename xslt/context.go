package xslt

import (
	"fmt"

	"github.com/midbel/codecs/environ"
	"github.com/midbel/codecs/xml"
	"github.com/midbel/codecs/xpath"
)

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

func (c *Context) errorWithContext(err error) error {
	if c.XslNode == nil {
		return err
	}
	return errorWithContext(c.XslNode.QualifiedName(), err)
}

func (c *Context) queryXSL(query string) (xpath.Sequence, error) {
	c.Query(c, query)
	return c.Env.queryXSL(query, c.XslNode)
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

type Resolver interface {
	Resolve(string) (xpath.Expr, error)
}

type Env struct {
	other     Resolver
	Namespace string
	Vars      environ.Environ[xpath.Expr]
	Params    environ.Environ[xpath.Expr]
	Builtins  environ.Environ[xpath.BuiltinFunc]
	Depth     int
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

func (e *Env) Sub() *Env {
	return &Env{
		other:     e.other,
		Namespace: e.Namespace,
		Vars:      environ.Enclosed[xpath.Expr](e.Vars),
		Params:    environ.Enclosed[xpath.Expr](e.Params),
		Builtins:  e.Builtins,
		Depth:     e.Depth + 1,
	}
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

func (e *Env) queryXSL(query string, datum xml.Node) (xpath.Sequence, error) {
	return e.ExecuteQueryWithNS(query, e.Namespace, datum)
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
	items, err := e.ExecuteQuery(query, datum)
	if err != nil {
		return false, err
	}
	return isTrue(items), nil
}

func (e *Env) Merge(other *Env) {
	if m, ok := e.Vars.(interface {
		Merge(environ.Environ[xpath.Expr])
	}); ok {
		m.Merge(other.Vars)
	}
	if m, ok := e.Params.(interface {
		Merge(environ.Environ[xpath.Expr])
	}); ok {
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
	e.Vars.Define(ident, expr)
}

func (e *Env) DefineParam(param, value string) error {
	expr, err := e.CompileQuery(value)
	if err == nil {
		e.DefineExprParam(param, expr)
	}
	return err
}

func (e *Env) EvalParam(param, query string, datum xml.Node) error {
	items, err := e.ExecuteQuery(query, datum)
	if err == nil {
		e.DefineExprParam(param, xpath.NewValueFromSequence(items))
	}
	return err
}

func (e *Env) DefineExprParam(param string, expr xpath.Expr) {
	e.Params.Define(param, expr)
}

func isTrue(seq xpath.Sequence) bool {
	if seq.Empty() {
		return false
	}
	first, ok := seq.First()
	if !first.Atomic() {
		return true
	}
	switch res := first.Value().(type) {
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

func errorWithContext(ctx string, err error) error {
	return fmt.Errorf("%s: %w", ctx, err)
}
