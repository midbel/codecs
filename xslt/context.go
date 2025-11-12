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

	env *xpath.Evaluator
}

func (c *Context) RegisterFunc(ident string, fn xpath.BuiltinFunc) {
	c.env.RegisterFunc(ident, fn)
}

func (c *Context) Execute(query string) (xpath.Sequence, error) {
	return c.env.Find(query, c.ContextNode)
}

func (c *Context) Compile(query string) (xpath.Expr, error) {
	return c.env.Create(query)
}

func (c *Context) Test(query string) (bool, error) {
	seq, err := c.Execute(query)
	if err != nil {
		return false, err
	}
	return seq.True(), nil
}

func (c *Context) Set(ident string, expr xpath.Expr) {
	c.env.Set(ident, expr)
}

func (c *Context) ApplyTemplate() ([]xml.Node, error) {
	ex, err := c.Match(c.ContextNode, c.Mode)
	if err != nil {
		return nil, err
	}
	return ex.Execute(c)
}

func (c *Context) SetXpathNamespace(ns string) {
	c.env.SetElemNS(ns)
}

func (c *Context) GetXpathNamespace() string {
	return c.env.GetElemNS()
}

func (c *Context) ResetXpathNamespace() string {
	old := c.env.GetElemNS()

	el, err := getElementFromNode(c.XslNode)
	if err == nil {
		n, err := getAttribute(el, c.getQualifiedName("xpath-default-namespace"))
		if err == nil {
			c.env.SetElemNS(n)
		}
	}
	return old
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

func (c *Context) Sub() *Context {
	child := c.clone(c.XslNode, c.ContextNode)
	child.env = child.env.Sub()
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
		env:         c.env,
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

func errorWithContext(ctx string, err error) error {
	return fmt.Errorf("%s: %w", ctx, err)
}
