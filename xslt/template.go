package xslt

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"

	"github.com/midbel/codecs/xml"
	"github.com/midbel/codecs/xpath"
)

type Template struct {
	Name     string
	Match    string
	Mode     string
	Priority float64
	Matcher

	Nodes []xml.Node

	params map[string]xpath.Expr
}

func NewTemplate(env *xpath.Evaluator, node xml.Node) (*Template, error) {
	el, err := getElementFromNode(node)
	if err != nil {
		return nil, err
	}
	var tpl Template
	for _, a := range el.Attributes() {
		switch attr := a.Value(); a.Name {
		case "priority":
			p, err := strconv.ParseFloat(attr, 64)
			if err != nil {
				return nil, err
			}
			tpl.Priority = p
		case "name":
			tpl.Name = attr
		case "match":
			tpl.Match = attr
			tpl.Matcher, err = CompileMatch(tpl.Match)
			if err != nil {
				return nil, err
			}
		case "mode":
			tpl.Mode = attr
		default:
		}
	}
	tpl.params = make(map[string]xpath.Expr)
	for i, n := range el.Nodes {
		if n.QualifiedName() != "xsl:param" {
			tpl.Nodes = append(tpl.Nodes, el.Nodes[i:]...)
			break
		}
		if err := tpl.setParam(env, n); err != nil {
			return nil, err
		}
	}
	return &tpl, nil
}

func (t *Template) Clone() *Template {
	tpl := *t
	tpl.Nodes = slices.Clone(tpl.Nodes)
	tpl.params = maps.Clone(t.params)
	return &tpl
}

func (t *Template) Execute(ctx *Context) ([]xml.Node, error) {
	if err := t.fillWithDefaults(ctx); err != nil {
		return nil, err
	}
	var nodes []xml.Node
	for _, n := range slices.Clone(t.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		res, err := transformNode(ctx.WithXsl(c))
		if err != nil {
			if errors.Is(err, errSkip) {
				continue
			}
			return nil, err
		}
		for i := range res {
			nodes = append(nodes, res[i].Node())
		}
	}
	return nodes, nil
}

func (t *Template) fillWithDefaults(ctx *Context) error {
	for n, e := range t.params {
		_, err := ctx.env.Resolve(n)
		if err == nil {
			continue
		}
		seq, err := e.Find(ctx.ContextNode)
		if err != nil {
			return err
		}
		ctx.env.Set(n, xpath.NewValueFromSequence(seq))
	}
	return nil
}

func (t *Template) hasParam(ident string) bool {
	_, ok := t.params[ident]
	return ok
}

func (t *Template) setParam(parent *xpath.Evaluator, node xml.Node) error {
	elem, err := getElementFromNode(node)
	if err != nil {
		return err
	}
	ident, err := getAttribute(elem, "name")
	if err != nil {
		return err
	}
	var expr xpath.Expr
	if query, err1 := getAttribute(elem, "select"); err1 == nil {
		if len(elem.Nodes) > 0 {
			return fmt.Errorf("using select and children nodes is not allowed")
		}
		expr, err = parent.Create(query)
	} else {
		var seq xpath.Sequence
		for i := range elem.Nodes {
			seq.Append(xpath.NewNodeItem(elem.Nodes[i]))
		}
		expr = xpath.NewValueFromSequence(seq)
	}
	if err == nil {
		if _, ok := t.params[ident]; ok {
			return fmt.Errorf("%s: param already defined", ident)
		}
		t.params[ident] = expr
	}
	return err
}

type virtualApplyTemplate struct {
	exec Executer
}

func ApplyVirtual(exec Executer) Executer {
	return virtualApplyTemplate{
		exec: exec,
	}
}

func (a virtualApplyTemplate) Execute(ctx *Context) ([]xml.Node, error) {
	nodes, err := a.exec.Execute(ctx)
	if err != nil {
		return nil, err
	}
	var (
		others []xml.Node
		fake   = xml.NewElement(xml.LocalName("fake"))
	)
	ctx = ctx.WithXsl(fake)
	for _, n := range nodes {
		if t := n.Type(); t != xml.TypeElement && t != xml.TypeDocument {
			others = append(others, n)
			continue
		}
		c := cloneNode(n)
		if c == nil {
			continue
		}
		exec, err := ctx.Match(c, ctx.Mode)
		if err != nil {
			return nil, err
		}
		res, err := exec.Execute(ctx.WithXpath(c))
		if err != nil {
			return nil, err
		}
		others = slices.Concat(others, res)
	}
	return others, nil
}

func templateMatch(expr xpath.Expr, node xml.Node) (bool, int) {
	for curr := node; curr != nil; {
		items, err := expr.Find(curr)
		if err != nil {
			break
		}
		if items.Len() > 0 {
			ok := slices.ContainsFunc(items, func(i xpath.Item) bool {
				n := i.Node()
				return n.Identity() == node.Identity()
			})
			return ok, expr.MatchPriority()
		}
		curr = curr.Parent()
	}
	return false, 0
}

type builtinNoMatch struct{}

func (builtinNoMatch) Execute(ctx *Context) ([]xml.Node, error) {
	var nodes []xml.Node
	switch ctx.ContextNode.Type() {
	case xml.TypeDocument:
		doc := ctx.ContextNode.(*xml.Document)
		nodes = append(nodes, doc.Root())
	case xml.TypeElement:
		el := ctx.ContextNode.(*xml.Element)
		for i := range el.Nodes {
			t := el.Nodes[i].Type()
			if t == xml.TypeComment || t == xml.TypeInstruction {
				continue
			}
			nodes = append(nodes, el.Nodes[i])
		}
	case xml.TypeText:
		nodes = append(nodes, ctx.ContextNode)
	default:
	}
	return nodes, nil
}

type textOnlyCopy struct{}

func (textOnlyCopy) Execute(ctx *Context) ([]xml.Node, error) {
	var nodes []xml.Node
	switch ctx.ContextNode.Type() {
	case xml.TypeDocument:
		doc := ctx.ContextNode.(*xml.Document)
		nodes = append(nodes, doc.Root())
	case xml.TypeElement:
		el := ctx.ContextNode.(*xml.Element)
		for i := range el.Nodes {
			t := el.Nodes[i].Type()
			if t == xml.TypeComment || t == xml.TypeInstruction {
				continue
			}
			nodes = append(nodes, el.Nodes[i])
		}
	case xml.TypeText:
		nodes = append(nodes, ctx.ContextNode)
	default:
		return nil, nil
	}
	return nodes, nil
}

type deepCopy struct{}

func (deepCopy) Execute(ctx *Context) ([]xml.Node, error) {
	node := cloneNode(ctx.ContextNode)
	return []xml.Node{node}, nil
}

type shallowCopy struct{}

func (shallowCopy) Execute(ctx *Context) ([]xml.Node, error) {
	if ctx.ContextNode.Type() == xml.TypeDocument {
		doc := ctx.ContextNode.(*xml.Document)
		return ctx.WithXpath(doc.Root()).ApplyTemplate()
	}
	elem, err := getElementFromNode(ctx.ContextNode)
	if err != nil {
		return nil, err
	}
	clone, ok := elem.Copy().(*xml.Element)
	if !ok {
		return nil, nil
	}
	for _, n := range slices.Clone(elem.Nodes) {
		others, err := ctx.WithXpath(n).ApplyTemplate()
		if err != nil {
			return nil, err
		}
		for i := range others {
			clone.Append(others[i])
		}
	}
	return []xml.Node{clone}, nil
}

type deepSkip struct{}

func (deepSkip) Execute(ctx *Context) ([]xml.Node, error) {
	return nil, nil
}

type shallowSkip struct{}

func (shallowSkip) Execute(ctx *Context) ([]xml.Node, error) {
	var list []xml.Node
	switch ctx.ContextNode.Type() {
	case xml.TypeDocument:
		doc := ctx.ContextNode.(*xml.Document)
		for _, n := range doc.Nodes {
			res, err := ctx.WithXpath(n).ApplyTemplate()
			if err != nil {
				return nil, err
			}
			list = slices.Concat(list, res)
		}
	case xml.TypeElement:
		elem := ctx.ContextNode.(*xml.Element)
		for _, n := range elem.Nodes {
			res, err := ctx.WithXpath(n).ApplyTemplate()
			if err != nil {
				return nil, err
			}
			list = slices.Concat(list, res)
		}
	default:
	}
	return list, nil
}
