package xslt

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/midbel/codecs/xml"
	"github.com/midbel/codecs/xpath"
)

var errMissed = errors.New("missing attribute")

func transformNode(ctx *Context) (xpath.Sequence, error) {
	if ctx.XslNode.Type() != xml.TypeElement {
		c := cloneNode(ctx.XslNode)
		if c == nil {
			return nil, nil
		}
		return xpath.Singleton(c), nil
	}
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	fn, ok := executers[elem.QName]
	if !ok {
		if space := elem.QName.Space; space == ctx.xsltNamespace {
			err := fmt.Errorf("%s: instruction/declaration not expected here", space)
			return nil, ctx.errorWithContext(err)
		}
		seq, err := processNode(ctx)
		return seq, err
	}
	if fn == nil {
		return nil, fmt.Errorf("%s: %w", elem.QualifiedName(), errImplemented)
	}
	return fn(ctx)
}

func processNode(ctx *Context) (xpath.Sequence, error) {
	if ctx.XslNode.Type() != xml.TypeElement {
		c := cloneNode(ctx.XslNode)
		if c == nil {
			return nil, nil
		}
		return xpath.Singleton(c), nil
	}

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
	if ns, err := ctx.ResolveAliasNS(elem.Space); err == nil {
		elem.Space = ns.Prefix
		elem.Uri = ns.Uri
	}
	for i, a := range elem.Attrs {
		ns, err := ctx.ResolveAliasNS(a.Space)
		if err == nil {
			a.Space = ns.Prefix
			a.Uri = ns.Uri
			elem.Attrs[i] = a
		}
	}
	return xpath.Singleton(elem), nil
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
		return "", fmt.Errorf("%s: %w %q", el.QualifiedName(), errMissed, ident)
	}
	return el.Attrs[ix].Value(), nil
}

func hasAttribute(name string, attrs []xml.Attribute) bool {
	return slices.ContainsFunc(attrs, func(a xml.Attribute) bool {
		return a.Name == name
	})
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
