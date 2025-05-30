package xslt

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/midbel/codecs/xml"
	"github.com/midbel/codecs/xpath"
)

func transformNode(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	fn, ok := executers[elem.QName]
	if !ok {
		seq, err := processNode(ctx)
		return seq, err
	}
	if fn == nil {
		return nil, fmt.Errorf("%s: %w", elem.QualifiedName(), errImplemented)
	}
	return fn(ctx)
}

func processNode(ctx *Context) (xpath.Sequence, error) {
	ctx.Enter(ctx)
	defer ctx.Leave(ctx)

	elem, err := getElementFromNode(cloneNode(ctx.XslNode))
	if err != nil {
		return nil, err
	}
	var (
		nested = ctx.WithXsl(elem)
		nodes  = slices.Clone(elem.Nodes)
	)
	elem.Nodes = elem.Nodes[:0]
	if err := processAVT(nested); err != nil {
		return nil, err
	}
	if err := nested.SetAttributes(elem); err != nil {
		return nil, err
	}
	for _, n := range nodes {
		if n.Type() != xml.TypeElement {
			c := cloneNode(n)
			if c != nil {
				elem.Nodes = append(elem.Nodes, c)

			}
			continue
		}
		res, err := transformNode(nested.WithXsl(n))
		if err != nil {
			return nil, err
		}
		for i := range res {
			elem.Append(res[i].Node())
		}
	}
	return xpath.Singleton(elem), nil
}

func appendNode(ctx *Context) (xpath.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	seq := xpath.NewSequence()
	for _, n := range elem.Nodes {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		res, err := transformNode(ctx.WithXsl(c))
		if err != nil {
			return nil, err
		}
		seq.Concat(res)
	}
	return seq, nil
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
		var seq xpath.Sequence
		for i := range elem.Nodes {
			seq.Append(xpath.NewNodeItem(elem.Nodes[i]))
		}
		env.DefineExprParam(ident, xpath.NewValueFromSequence(seq))
	}
	return err
}

func cloneNode(n xml.Node) xml.Node {
	cloner, ok := n.(xml.Cloner)
	if !ok {
		return nil
	}
	return cloner.Clone()
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

func toString(item xpath.Item) string {
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
