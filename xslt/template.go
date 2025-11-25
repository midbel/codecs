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
			tpl.Matcher, err = compileMatchWithEnv(env, tpl.Match)
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

type builtinNoMatch struct{}

func (builtinNoMatch) Execute(ctx *Context) ([]xml.Node, error) {
	var nodes []xml.Node
	switch ctx.ContextNode.Type() {
	case xml.TypeDocument:
		doc := ctx.ContextNode.(*xml.Document)
		return ctx.WithXpath(doc.Root()).ApplyTemplate()
	case xml.TypeElement:
		el := ctx.ContextNode.(*xml.Element)
		for i := range el.Nodes {
			t := el.Nodes[i].Type()
			if t == xml.TypeComment || t == xml.TypeInstruction {
				continue
			}
			others, err := ctx.WithXpath(el.Nodes[i]).ApplyTemplate()
			if err != nil {
				return nil, err
			}
			nodes = slices.Concat(nodes, others)
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
		return ctx.WithXpath(doc.Root()).ApplyTemplate()
	case xml.TypeElement:
		el := ctx.ContextNode.(*xml.Element)
		for i := range el.Nodes {
			t := el.Nodes[i].Type()
			if t == xml.TypeComment || t == xml.TypeInstruction {
				continue
			}
			others, err := ctx.WithXpath(el.Nodes[i]).ApplyTemplate()
			if err != nil {
				return nil, err
			}
			nodes = slices.Concat(nodes, others)
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
	switch ctx.ContextNode.Type() {
	case xml.TypeDocument:
		doc := ctx.ContextNode.(*xml.Document)
		return ctx.WithXpath(doc.Root()).ApplyTemplate()
	case xml.TypeElement:
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
	default:
		return []xml.Node{ctx.ContextNode}, nil
	}
}

type deepSkip struct{}

func (deepSkip) Execute(ctx *Context) ([]xml.Node, error) {
	return nil, nil
}

type shallowSkip struct{}

func (shallowSkip) Execute(ctx *Context) ([]xml.Node, error) {
	switch ctx.ContextNode.Type() {
	case xml.TypeDocument:
		doc := ctx.ContextNode.(*xml.Document)
		return ctx.WithXpath(doc.Root()).ApplyTemplate()
	case xml.TypeElement:
		var (
			elem  = ctx.ContextNode.(*xml.Element)
			nodes []xml.Node
		)
		for _, n := range elem.Nodes {
			others, err := ctx.WithXpath(n).ApplyTemplate()
			if err != nil {
				return nil, err
			}
			nodes = slices.Concat(nodes, others)
		}
		return nodes, nil
	default:
		return nil, nil
	}
}
