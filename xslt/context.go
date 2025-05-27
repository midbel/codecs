package xslt

import (
	"fmt"

	"github.com/midbel/codecs/xml"
)

type Context struct {
	XslNode     xml.Node
	ContextNode xml.Node

	Index int
	Size  int
	Mode  string

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

func (c *Context) queryXSL(query string) (xml.Sequence, error) {
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
		Index:       1,
		Size:        1,
		Stylesheet:  c.Stylesheet,
		Env:         c.Env,
		Depth:       c.Depth + 1,
	}
	return &child
}

func (c *Context) NotFound(err error, mode string) (xml.Sequence, error) {
	var tmp xml.Node
	switch mode := c.getMode(mode); mode.NoMatch {
	case MatchDeepCopy:
		tmp = cloneNode(c.ContextNode)
	case MatchShallowCopy:
		elem, err := getElementFromNode(c.ContextNode)
		if err != nil {
			return nil, err
		}
		curr := xml.NewElement(elem.QName)
		for i := range elem.Attrs {
			curr.SetAttribute(elem.Attrs[i])
		}
		tmp = curr
	case MatchTextOnlyCopy:
		tmp = xml.NewText(c.ContextNode.Value())
	case MatchFail:
		return nil, err
	default:
		return nil, err
	}
	return xml.Singleton(tmp), nil
}

type Resolver interface {
	Resolve(string) (xml.Expr, error)
}

type Env struct {
	other     Resolver
	Namespace string
	Vars      xml.Environ[xml.Expr]
	Params    xml.Environ[xml.Expr]
	Builtins  xml.Environ[xml.BuiltinFunc]
	Depth     int
}

func Empty() *Env {
	return Enclosed(nil)
}

func Enclosed(other Resolver) *Env {
	return &Env{
		other:    other,
		Vars:     xml.Empty[xml.Expr](),
		Params:   xml.Empty[xml.Expr](),
		Builtins: xml.DefaultBuiltin(),
	}
}

func (e *Env) Sub() *Env {
	return &Env{
		other:     e.other,
		Namespace: e.Namespace,
		Vars:      xml.Enclosed[xml.Expr](e.Vars),
		Params:    xml.Enclosed[xml.Expr](e.Params),
		Builtins:  e.Builtins,
		Depth:     e.Depth + 1,
	}
}

func (e *Env) ExecuteQuery(query string, datum xml.Node) (xml.Sequence, error) {
	return e.ExecuteQueryWithNS(query, "", datum)
}

func (e *Env) ExecuteQueryWithNS(query, namespace string, datum xml.Node) (xml.Sequence, error) {
	if query == "" {
		i := xml.NewNodeItem(datum)
		return xml.Singleton(i), nil
	}
	q, err := e.CompileQueryWithNS(query, namespace)
	if err != nil {
		return nil, err
	}
	return q.Find(datum)
}

func (e *Env) queryXSL(query string, datum xml.Node) (xml.Sequence, error) {
	return e.ExecuteQueryWithNS(query, e.Namespace, datum)
}

func (e *Env) CompileQuery(query string) (xml.Expr, error) {
	return e.CompileQueryWithNS(query, "")
}

func (e *Env) CompileQueryWithNS(query, namespace string) (xml.Expr, error) {
	q, err := xml.Build(query)
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
	if m, ok := e.Vars.(interface{ Merge(xml.Environ[xml.Expr]) }); ok {
		m.Merge(other.Vars)
	}
	if m, ok := e.Params.(interface{ Merge(xml.Environ[xml.Expr]) }); ok {
		m.Merge(other.Params)
	}
}

func (e *Env) Resolve(ident string) (xml.Expr, error) {
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

func (e *Env) Define(ident string, expr xml.Expr) {
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
		e.DefineExprParam(param, xml.NewValueFromSequence(items))
	}
	return err
}

func (e *Env) DefineExprParam(param string, expr xml.Expr) {
	e.Params.Define(param, expr)
}

func isTrue(seq xml.Sequence) bool {
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
