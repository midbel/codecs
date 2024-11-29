package main

import (
	"flag"
	"fmt"
	"os"
	"slices"

	"github.com/midbel/codecs/relax"
	"github.com/midbel/codecs/xml"
)

func main() {
	flag.Parse()

	schema, err := parseSchema(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "parsing schema:", err)
		os.Exit(21)
	}
	doc, err := parseDocument(flag.Arg(1))
	if err != nil {
		fmt.Println(flag.Arg(1))
		fmt.Fprintln(os.Stderr, "parsing document:", err)
		os.Exit(11)
	}
	if err := validateDocument(doc, schema); err != nil {
		fmt.Fprintln(os.Stderr, "document does not conform to given schema")
		fmt.Fprintln(os.Stderr, err)
		os.Exit(32)
	}
	fmt.Println("document is valid")
}

func parseSchema(file string) (relax.Pattern, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	p := relax.Parse(r)
	return p.Parse()
}

func parseDocument(file string) (*xml.Document, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	p := xml.NewParser(r)
	p.TrimSpace = true
	p.OmitProlog = false

	return p.Parse()
}

func validateDocument(doc *xml.Document, schema relax.Pattern) error {
	return validateNode(doc.Root(), schema)
}

func validateNode(root xml.Node, pattern relax.Pattern) error {
	switch pattern := pattern.(type) {
	case relax.Element:
		return validateElement(root, pattern)
	case relax.Attribute:
		return validateAttribute(root, pattern)
	case relax.Text:
		return validateText(root)
	case relax.Empty:
		return validateEmpty(root)
	default:
		return fmt.Errorf("pattern not yet supported")
	}
}

func validateElement(node xml.Node, elem relax.Element) error {
	if elem.QualifiedName() != node.QualifiedName() {
		return fmt.Errorf("element name mismatched! want %s, got %s", elem.QualifiedName(), node.QualifiedName())
	}
	for _, a := range elem.Attributes {
		if err := validateNode(node, a); err != nil {
			return err
		}
	}
	curr, ok := node.(*xml.Element)
	if !ok {
		return fmt.Errorf("node is not a xml element")
	}
	var offset int
	for _, el := range elem.Elements {
		k, ok := el.(relax.Element)
		if !ok {
			return fmt.Errorf("missing element")
		}
		var count int
		for i := offset; i < len(curr.Nodes); i++ {
			offset++
			if _, ok := curr.Nodes[i].(*xml.Element); !ok {
				continue
			}
			if curr.Nodes[i].QualifiedName() != k.QualifiedName() {
				offset--
				break
			}
			if err := validateNode(curr.Nodes[i], k); err != nil {
				return err
			}
			count++
		}
		switch {
		case count == 0 && k.Arity.Zero():
		case count == 1 && k.Arity.One():
		case count > 1 && k.Arity.More():
		default:
			return fmt.Errorf("%s: invalid number of elements (%d)", k.QualifiedName(), count)
		}
	}
	return nil
}

func validateAttribute(node xml.Node, attr relax.Attribute) error {
	el, ok := node.(*xml.Element)
	if !ok {
		return fmt.Errorf("node is not a xml element")
	}
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.QualifiedName() == attr.QualifiedName()
	})
	if ix < 0 && !attr.Arity.Zero() {
		return fmt.Errorf("missing attribute")
	}
	return nil
}

func validateText(node xml.Node) error {
	if !node.Leaf() {
		return fmt.Errorf("expected text content")
	}
	return nil
}

func validateEmpty(node xml.Node) error {
	el, ok := node.(*xml.Element)
	if ok && len(el.Nodes) != 0 {
		return fmt.Errorf("expected element to be empty")
	}
	return nil
}
