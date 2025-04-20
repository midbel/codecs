package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/midbel/codecs/xml"
)

func main() {
	quiet := flag.Bool("q", false, "quiet")
	flag.Parse()

	doc, err := loadDocument(flag.Arg(1))
	if err != nil {
		fmt.Fprintln(os.Stderr, "fail to load document:", err)
		os.Exit(2)
	}
	result, err := transform(flag.Arg(0), doc)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fail to load stylesheet:", err)
		os.Exit(2)
	}
	var w io.Writer = os.Stdout
	if *quiet {
		w = io.Discard
	}
	writer := xml.NewWriter(w)
	writer.Write(result)
}

func transform(file string, source *xml.Document) (*xml.Document, error) {
	doc, err := loadDocument(file)
	if err != nil {
		return nil, err
	}
	tpl, err := getMainTemplate(doc)
	if err != nil {
		return nil, err
	}
	expr, err := xml.CompileString("/")
	if err != nil {
		return nil, err
	}

	items, err := expr.Find(source)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no root template found")
	}
	root, err := processTemplate(tpl, items[0].Node())
	if err != nil {
		return nil, err
	}
	return xml.NewDocument(root), nil
}

func processTemplate(node xml.Node, datum xml.Node) (xml.Node, error) {
	el, ok := node.(*xml.Element)
	if !ok {
		return nil, fmt.Errorf("template: invalid element")
	}
	if len(el.Nodes) != 1 {
		return nil, fmt.Errorf("template should have no more than one element")
	}
	root := el.Nodes[0]
	if err := transformNode(root, datum); err != nil {
		return nil, err
	}
	return root, nil
}

func transformNode(node, datum xml.Node) error {
	el, ok := node.(*xml.Element)
	if !ok {
		return fmt.Errorf("node: xml element expected (got %T)", el)
	}
	fmt.Printf("transformNode: %s\n", el.QualifiedName())
	if el.Space == "xsl" && el.Name == "for-each" {
		return processForeach(el, datum)
	} else if el.Space == "xsl" && el.Name == "value-of" {
		return processValueOf(el, datum)
	} else {
		for i := range el.Nodes {
			if el.Nodes[i].Type() != xml.TypeElement {
				continue
			}
			err := transformNode(el.Nodes[i], datum)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func processForeach(node, datum xml.Node) error {
	el := node.(*xml.Element)
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "select"
	})
	if ix < 0 {
		return fmt.Errorf("for-each: missing select attribute")
	}
	parent, ok := el.Parent().(*xml.Element)
	if !ok {
		return fmt.Errorf("for-each: xml element expected as parent")
	}
	parent.Nodes = parent.Nodes[:0]

	expr, err := xml.CompileString(el.Attrs[ix].Value())
	if err != nil {
		return err
	}
	items, err := expr.Find(datum)
	if err != nil {
		return err
	}

	for i := range items {
		value := items[i].Node()
		for _, n := range el.Nodes {
			c := cloneNode(n)
			if c == nil {
				continue
			}
			parent.Append(c)
			if err := transformNode(c, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func processValueOf(node, datum xml.Node) error {
	el := node.(*xml.Element)
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "select"
	})
	if ix < 0 {
		return fmt.Errorf("value-of: missing select attribute")
	}
	expr, err := xml.CompileString(el.Attrs[ix].Value())
	if err != nil {
		return err
	}
	items, err := expr.Find(datum)
	if err != nil || len(items) == 0 {
		return err
	}
	text := xml.NewText(items[0].Node().Value())
	parent, ok := el.Parent().(*xml.Element)
	if !ok {
		return fmt.Errorf("value-of: xml element expected as parent")
	}
	_, _ = text, parent
	parent.Nodes = parent.Nodes[:0]
	parent.Append(text)
	return nil
}

func cloneNode(n xml.Node) xml.Node {
	cloner, ok := n.(xml.Cloner)
	if !ok {
		return nil
	}
	return cloner.Clone()
}

func getMainTemplate(doc *xml.Document) (xml.Node, error) {
	tpl, err := doc.Query("//xsl:template[@match=\"/\"]")
	if err != nil {
		return nil, err
	}
	root, ok := tpl.Root().(*xml.Element)
	if !ok || len(root.Nodes) == 0 {
		return nil, fmt.Errorf("invalid root element")
	}
	return root.Nodes[0], nil
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
