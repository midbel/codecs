package xslt

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/midbel/codecs/xml"
)

func transformNode(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	fn, ok := executers[elem.QName]
	if !ok {
		return processNode(ctx)
	}
	if fn == nil {
		return nil, fmt.Errorf("%s: %w", elem.QualifiedName(), errImplemented)
	}
	seq, err := fn(ctx)
	if err != nil {
		return nil, err
	}
	if seq.Len() > 0 {
		parent, err := getElementFromNode(elem.Parent())
		if err != nil {
			return nil, err
		}
		for _, i := range seq {
			parent.Append(i.Node())
		}
	}
	return nil, nil
}

func appendNode(ctx *Context) error {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return ctx.errorWithContext(err)
	}
	parent, err := getElementFromNode(elem.Parent())
	if err != nil {
		return ctx.errorWithContext(err)
	}
	for _, n := range slices.Clone(elem.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		parent.Append(c)
		if _, err := transformNode(ctx.WithXsl(c)); err != nil {
			return err
		}
	}
	return nil
}

func processParam(node xml.Node, env *Env) error {
	elem, err := getElementFromNode(node)
	if err != nil {
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

func processNode(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, err
	}
	if err := processAVT(ctx); err != nil {
		return nil, err
	}
	if err := ctx.SetAttributes(elem); err != nil {
		return nil, err
	}
	var (
		nodes = slices.Clone(elem.Nodes)
		res   = xml.NewSequence()
	)
	for i := range nodes {
		if nodes[i].Type() != xml.TypeElement {
			res.Append(xml.NewNodeItem(nodes[i]))
			continue
		}
		seq, err := transformNode(ctx.WithXsl(nodes[i]))
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

func getElementFromNode(node xml.Node) (*xml.Element, error) {
	el, ok := node.(*xml.Element)
	if !ok {
		return nil, fmt.Errorf("%s: xml element expected", node.QualifiedName())
	}
	return el, nil
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

func toString(item xml.Item) string {
	var v string
	switch x := item.Value().(type) {
	case time.Time:
		v = x.Format("2006-01-02")
	case float64:
		v = strconv.FormatFloat(x, 'f', -1, 64)
	case []byte:
	case string:
		v = x
	default:
	}
	return v
}
