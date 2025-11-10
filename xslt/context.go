package xslt

import (
	"fmt"

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

func (c *Context) Last() *Context {
	return c
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

type Env struct {
	eval  *xpath.Evaluator
	Funcs environ.Environ[*Function]
	Depth int

	aliases environ.Environ[string]
	other   *Env
}

func Empty() *Env {
	return Enclosed(nil)
}

func Enclosed(other *Env) *Env {
	return &Env{
		other:   other,
		eval:    xpath.NewEvaluator(),
		Funcs:   environ.Empty[*Function](),
		aliases: environ.Empty[string](),
	}
}

func (e *Env) GetXpathNamespace() string {
	return e.eval.GetElemNS()
}

func (e *Env) SetXpathNamespace(ns string) {
	e.eval.SetElemNS(ns)
}

func (e *Env) Sub() *Env {
	return &Env{
		other: e.other,
		Funcs: e.Funcs,
		Depth: e.Depth + 1,
		eval:  e.eval.Sub(),
	}
}

func (e *Env) Merge(other *Env) *Env {
	return e
}

func (e *Env) ExecuteQuery(query string, node xml.Node) (xpath.Sequence, error) {
	if query == "" {
		i := xpath.NewNodeItem(node)
		return xpath.Singleton(i), nil
	}
	q, err := e.CompileQuery(query)
	if err != nil {
		return nil, err
	}
	return q.Find(node)
}

func (e *Env) CompileQuery(query string) (xpath.Expr, error) {
	q, err := e.eval.Create(query)
	if err != nil {
		return nil, err
	}
	return q, nil
}

func (e *Env) TestNode(query string, node xml.Node) (bool, error) {
	seq, err := e.ExecuteQuery(query, node)
	if err != nil {
		return false, err
	}
	return seq.True(), nil
}

func (e *Env) Resolve(ident string) (xpath.Expr, error) {
	expr, err := e.eval.Resolve(ident)
	if err == nil {
		return expr, nil
	}
	if e.other != nil {
		return e.other.Resolve(ident)
	}
	return nil, err
}

func (e *Env) ResolveAliasNS(ident string) (xml.NS, error) {
	if e.other != nil {
		return e.other.ResolveAliasNS(ident)
	}
	var (
		ns  xml.NS
		err error
	)
	ns.Prefix, err = e.aliases.Resolve(ident)
	if err != nil {
		return ns, err
	}
	ns.Uri, err = e.eval.ResolveNS(ns.Prefix)
	if err != nil {
		return ns, err
	}
	return ns, nil
}

func (e *Env) ResolveFunc(ident string) (xpath.Callable, error) {
	fn, err := e.Funcs.Resolve(ident)
	if err != nil {
		b, err := e.eval.ResolveFunc(ident)
		if err == nil {
			return b, nil
		}
		if e.other != nil {
			return e.other.ResolveFunc(ident)
		}
		return nil, err
	}
	return fn, nil
}

func (e *Env) RegisterFunc(ident string, fn xpath.BuiltinFunc) {
	e.eval.RegisterFunc(ident, fn)
}

func (e *Env) Define(ident string, expr xpath.Expr) {
	e.eval.Set(ident, expr)
}

func (e *Env) Eval(ident, query string, node xml.Node) error {
	items, err := e.ExecuteQuery(query, node)
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

func (e *Env) EvalParam(ident, query string, node xml.Node) error {
	items, err := e.ExecuteQuery(query, node)
	if err == nil {
		e.Define(ident, xpath.NewValueFromSequence(items))
	}
	return err
}

func (e *Env) DefineExprParam(ident string, expr xpath.Expr) {
	e.eval.Set(ident, expr)
}

func (e *Env) RegisterNS(prefix, uri string) {
	e.eval.Define(prefix, uri)
}

func errorWithContext(ctx string, err error) error {
	return fmt.Errorf("%s: %w", ctx, err)
}
