package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

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
		fmt.Fprintln(os.Stderr, "document does not conform to given schema: %s", err)
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
	curr, ok := node.(*xml.Element)
	if !ok {
		return fmt.Errorf("node is not a xml element")
	}
	var (
		groups = groupNodes(curr)
		// offset int
		j      int
	)
	for i := range elem.Elements {
		el, ok := elem.Elements[i].(relax.Element)
		if !ok {
			continue
		}
		if el.QualifiedName() != groups[j][0].QualifiedName() {
			if el.Arity == relax.ZeroOrOne || el.Arity == relax.ZeroOrMore {
				continue
			}
			return fmt.Errorf("invalid element: want %s, got %s", el.QualifiedName(), groups[j][0].QualifiedName())
		}
		switch el.Arity {
		case relax.ZeroOrOne, 0:
			if len(groups[j]) != 1 {
				return fmt.Errorf("invalid number of elements for %s", el.QualifiedName())
			}
		case relax.OneOrMore:
			if len(groups[j]) < 1 {
				return fmt.Errorf("invalid number of elements for %s", el.QualifiedName())
			}
		case relax.ZeroOrMore:
		default:
			return fmt.Errorf("can not check number of elements")
		}
		for _, n := range groups[j] {
			if err := validateNode(n, el); err != nil {
				return err
			}
		}
		j++
	}
	return nil
}

func validateAttribute(node xml.Node, attr relax.Attribute) error {
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

func groupNodes(node *xml.Element) [][]xml.Node {
	var groups [][]xml.Node
	for i := 0; i < len(node.Nodes); {
		n := node.Nodes[i]
		if _, ok := n.(*xml.Element); !ok {
			i++
			continue
		}
		tmp := []xml.Node{n}
		if i >= len(node.Nodes) {
			groups = append(groups, tmp)
			break
		}
		for _, x := range node.Nodes[i+1:] {
			if _, ok := x.(*xml.Element); !ok {
				continue
			}
			if x.QualifiedName() != n.QualifiedName() {
				break
			}
			tmp = append(tmp, x)
		}
		groups = append(groups, tmp)
		i += len(tmp)
	}
	return groups
}

func printPattern(pattern relax.Pattern, depth int) {
	var prefix string
	if depth > 1 {
		prefix = strings.Repeat(">", depth)
	}
	switch p := pattern.(type) {
	case relax.Element:
		fmt.Println(prefix, "element:", p.Local)
		for i := range p.Attributes {
			printPattern(p.Attributes[i], depth+1)
		}
		for i := range p.Elements {
			printPattern(p.Elements[i], depth+1)
		}
	case relax.Attribute:
		fmt.Println(prefix, "attribute:", p.Local)
	case relax.Text:
		fmt.Println(prefix, "text")
	case relax.Empty:
		fmt.Println(prefix, "empty")
	case relax.Link:
		fmt.Println(prefix, "link", p.Ident)
	default:
		fmt.Println(prefix, "unknown")
	}
}
