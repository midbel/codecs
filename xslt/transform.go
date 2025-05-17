package xslt

import (
	"fmt"
	"slices"

	"github.com/midbel/codecs/xml"
)

func transformNode(ctx *Context, node xml.Node) (xml.Sequence, error) {
	elem, ok := node.(*xml.Element)
	if !ok {
		return nil, fmt.Errorf("node: xml element expected (got %s)", elem.QualifiedName())
	}
	fn, ok := executers[elem.QName]
	if ok {
		if fn == nil {
			return nil, fmt.Errorf("%s not yet implemented", elem.QualifiedName())
		}
		return fn(ctx, node)
	}
	return processNode(ctx, node)
}

func appendNode(ctx *Context, node xml.Node) error {
	elem, ok := node.(*xml.Element)
	if !ok {
		return fmt.Errorf("%s: expected xml element", node.QualifiedName())
	}
	parent, ok := node.Parent().(*xml.Element)
	if !ok {
		return fmt.Errorf("%s: expected xml element", node.QualifiedName())
	}
	for _, n := range elem.Nodes {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		parent.Append(c)
		if _, err := transformNode(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

func processParam(node xml.Node, env *Env) error {
	elem, ok := node.(*xml.Element)
	if !ok {
		return fmt.Errorf("xml element expected")
	}
	ident, err := getAttribute(elem, "name")
	if err != nil {
		return err
	}
	if query, err := getAttribute(elem, "select"); err == nil {
		err = env.DefineParam(ident, query)
	} else {
		var seq xml.Sequence
		for i := range elem.Nodes {
			seq.Append(xml.NewNodeItem(elem.Nodes[i]))
		}
		env.DefineExprParam(ident, xml.NewValueFromSequence(seq))
	}
	return err
}

func processNode(ctx *Context, node xml.Node) (xml.Sequence, error) {
	var (
		elem  = node.(*xml.Element)
		nodes = slices.Clone(elem.Nodes)
	)
	if err := processAVT(ctx, node); err != nil {
		return nil, err
	}
	if err := ctx.SetAttributes(node); err != nil {
		return nil, err
	}
	res := xml.NewSequence()
	for i := range nodes {
		if nodes[i].Type() != xml.TypeElement {
			continue
		}
		seq, err := transformNode(ctx, nodes[i])
		if err != nil {
			return nil, err
		}
		res = slices.Concat(res, seq)
	}
	return res, nil
}

func cloneNode(n xml.Node) xml.Node {
	cloner, ok := n.(xml.Cloner)
	if !ok {
		return nil
	}
	return cloner.Clone()
}

func removeNode(elem, node xml.Node) error {
	if node == nil {
		return nil
	}
	return removeAt(elem, node.Position())
}

func removeAt(elem xml.Node, pos int) error {
	p := elem.Parent()
	r, ok := p.(interface{ RemoveNode(int) error })
	if !ok {
		return fmt.Errorf("node can not be removed from parent element of %s", elem.QualifiedName())
	}
	return r.RemoveNode(pos)
}

func removeSelf(elem xml.Node) error {
	return removeNode(elem, elem)
}

func replaceNode(elem, node xml.Node) error {
	if node == nil {
		return nil
	}
	p := elem.Parent()
	r, ok := p.(interface{ ReplaceNode(int, xml.Node) error })
	if !ok {
		return fmt.Errorf("node can not be replaced from parent element of %s", elem.QualifiedName())
	}
	return r.ReplaceNode(elem.Position(), node)
}

func insertNodes(elem xml.Node, nodes ...xml.Node) error {
	if len(nodes) == 0 {
		return nil
	}
	p := elem.Parent()
	i, ok := p.(interface{ InsertNodes(int, []xml.Node) error })
	if !ok {
		return fmt.Errorf("nodes can not be inserted to parent element of %s", elem.QualifiedName())
	}
	return i.InsertNodes(elem.Position(), nodes)
}

func getAttribute(el *xml.Element, ident string) (string, error) {
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == ident
	})
	if ix < 0 {
		return "", fmt.Errorf("%s: missing attribute %q", el.QualifiedName(), ident)
	}
	return el.Attrs[ix].Value(), nil
}

func loadDocument(file string) (*xml.Document, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	p := xml.NewParser(r)
	return p.Parse()
}

func writeDocument(file, format string, doc *xml.Document, style *Stylesheet) error {
	w, err := os.Create(file)
	if err != nil {
		return err
	}
	defer w.Close()

	return style.writeDocument(w, format, doc)
}

func writeDoctypeHTML(w io.Writer) error {
	_, err := io.WriteString(w, "<!DOCTYPE html>")
	return err
}
