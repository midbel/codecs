package xslt

import (
	"errors"
	"fmt"
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

	Nodes []xml.Node

	env *xpath.Evaluator
}

func NewTemplate(node xml.Node) (*Template, error) {
	elem, ok := node.(*xml.Element)
	if !ok {
		return nil, fmt.Errorf("%s: xml element expected to load template", node.QualifiedName())
	}
	tpl := Template{
		env: xpath.NewEvaluator(),
	}
	for _, a := range elem.Attributes() {
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
		case "mode":
			tpl.Mode = attr
		default:
		}
	}
	for i, n := range elem.Nodes {
		if n.QualifiedName() != "xsl:param" {
			tpl.Nodes = append(tpl.Nodes, elem.Nodes[i:]...)
			break
		}
		if err := tpl.setParam(n); err != nil {
			return nil, err
		}
	}
	return &tpl, nil
}

func (t *Template) Clone() *Template {
	tpl := *t
	tpl.Nodes = slices.Clone(tpl.Nodes)
	tpl.env = t.env.Clone()
	return &tpl
}

func (t *Template) FillWithDefaults(ctx *Context) {
	ctx.env.Merge(t.env)
}

func (t *Template) Call(ctx *Context) ([]xml.Node, error) {
	return t.Execute(ctx)
}

func (t *Template) Execute(ctx *Context) ([]xml.Node, error) {
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

func (t *Template) isRoot() bool {
	return t.Match == "/"
}

func (t *Template) setParam(node xml.Node) error {
	elem, err := getElementFromNode(node)
	if err != nil {
		return err
	}
	ident, err := getAttribute(elem, "name")
	if err != nil {
		return err
	}
	if query, err := getAttribute(elem, "select"); err == nil {
		if len(elem.Nodes) > 0 {
			return fmt.Errorf("using select and children nodes is not allowed")
		}
		expr, err1 := t.env.Create(query)
		if err1 == nil {
			t.env.Set(ident, expr)
		}
		err = err1
	} else {
		var seq xpath.Sequence
		for i := range elem.Nodes {
			seq.Append(xpath.NewNodeItem(elem.Nodes[i]))
		}
		t.env.Set(ident, xpath.NewValueFromSequence(seq))
	}
	return err
}

func templateMatch(expr xpath.Expr, node xml.Node) (bool, int) {
	var (
		depth int
		curr  = node
	)
	for curr != nil {
		items, err := expr.Find(curr)
		if err != nil {
			break
		}
		if items.Len() > 0 {
			ok := slices.ContainsFunc(items, func(i xpath.Item) bool {
				n := i.Node()
				return n.Identity() == node.Identity()
			})
			return ok, depth + expr.MatchPriority()
		}
		depth--
		curr = curr.Parent()
	}
	return false, 0
}

type textOnlyCopy struct{}

func (c textOnlyCopy) Execute(ctx *Context) ([]xml.Node, error) {
	switch ctx.ContextNode.Type() {
	case xml.TypeElement:
		var (
			list []xml.Node
			elem = ctx.ContextNode.(*xml.Element)
		)
		for i := range elem.Nodes {
			others, err := ctx.WithXpath(elem.Nodes[i]).ApplyTemplate()
			if err != nil {
				return nil, err
			}
			list = slices.Concat(list, others)
		}
		return list, nil
	case xml.TypeDocument:
		var (
			list []xml.Node
			doc  = ctx.ContextNode.(*xml.Document)
		)
		for i := range doc.Nodes {
			others, err := ctx.WithXpath(doc.Nodes[i]).ApplyTemplate()
			if err != nil {
				return nil, err
			}
			list = slices.Concat(list, others)
		}
		return list, nil
	case xml.TypeText:
		node := xml.NewText(ctx.ContextNode.Value())
		return []xml.Node{node}, nil
	default:
		return nil, nil
	}
}

type deepCopy struct{}

func (_ deepCopy) Execute(ctx *Context) ([]xml.Node, error) {
	node := cloneNode(ctx.ContextNode)
	return []xml.Node{node}, nil
}

type shallowCopy struct{}

func (_ shallowCopy) Execute(ctx *Context) ([]xml.Node, error) {
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

func (_ deepSkip) Execute(ctx *Context) ([]xml.Node, error) {
	return nil, nil
}

type shallowSkip struct{}

func (_ shallowSkip) Execute(ctx *Context) ([]xml.Node, error) {
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
