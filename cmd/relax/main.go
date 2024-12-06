package main

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/midbel/codecs/relax"
	"github.com/midbel/codecs/xml"
)

func main() {
	debug := flag.Bool("g", false, "print parsed schema")
	flag.Parse()

	schema, err := parseSchema(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "parsing schema:", err)
		os.Exit(21)
	}
	if *debug {
		printSchema(schema, 0)
		return
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

func printSchema(schema relax.Pattern, depth int) {
	var prefix string
	if depth > 0 {
		prefix = strings.Repeat(" ", depth*2)
	}
	fmt.Print(prefix)
	switch p := schema.(type) {
	case relax.Element:
		fmt.Printf("element(%s)", p.QualifiedName())
		if len(p.Patterns) > 0 {
			fmt.Print("[")
		}
		fmt.Println()
		for i := range p.Patterns {
			printSchema(p.Patterns[i], depth+1)
		}
		if len(p.Patterns) > 0 {
			fmt.Print(prefix)
			fmt.Println("]")
		}
	case relax.Attribute:
		fmt.Printf("attribute(%s)", p.QualifiedName())
		fmt.Println()
	case relax.Choice:
		fmt.Printf("choice(%d)[", len(p.List))
		fmt.Println()
		depth++
		for i := range p.List {
			pfx := strings.Repeat(" ", depth*2)
			fmt.Print(pfx)
			fmt.Printf("choice#%d[", i+1)
			fmt.Println()
			printSchema(p.List[i], depth+1)
			fmt.Print(pfx)
			fmt.Println("]")
		}
		fmt.Print(prefix)
		fmt.Println("]")
	case relax.Group:
		fmt.Printf("group(%d)[", len(p.List))
		fmt.Println()
		for i := range p.List {
			printSchema(p.List[i], depth+1)
		}
		fmt.Print(prefix)
		fmt.Println("]")
	default:
		fmt.Println("unknown")
	}
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
	case relax.Choice:
		return validateChoice(root, pattern)
	default:
		return fmt.Errorf("pattern not yet supported")
	}
}

func validateChoice(node xml.Node, elem relax.Choice) error {
	var err error
	for _, el := range elem.List {
		if err = validateNode(node, el); err == nil {
			break
		}
	}
	return err
}

func validateNodes(nodes []xml.Node, elem relax.Pattern) (int, error) {
	var (
		count int
		ptr   int
		prv   = -1
	)
	for ; ptr < len(nodes); ptr++ {
		if _, ok := nodes[ptr].(*xml.Element); !ok {
			continue
		}
		if prv >= 0 && nodes[ptr].QualifiedName() != nodes[prv].QualifiedName() {
			break
		}
		if err := validateNode(nodes[ptr], elem); err != nil {
			a, ok := elem.(relax.Element)
			if ok && a.Zero() {
				return 0, nil
			}
			return 0, err
		}
		count++
		prv = ptr
	}
	a, ok := elem.(relax.Element)
	if !ok {
		return ptr, nil
	}
	switch {
	case count == 0 && a.Arity.Zero():
	case count == 1 && a.Arity.One():
	case count > 1 && a.Arity.More():
	default:
		return 0, fmt.Errorf("element count mismatched")
	}
	return ptr, nil
}

func validateElement(node xml.Node, elem relax.Element) error {
	if elem.QualifiedName() != node.QualifiedName() {
		return fmt.Errorf("element name mismatched! want %s, got %s", elem.QualifiedName(), node.QualifiedName())
	}
	curr, ok := node.(*xml.Element)
	if !ok {
		return fmt.Errorf("xml element expected")
	}
	var offset int
	for _, el := range elem.Patterns {
		var err error
		switch el := el.(type) {
		case relax.Element:
			step, err1 := validateNodes(curr.Nodes[offset:], el)
			offset += step
			err = err1
		case relax.Attribute:
			err = validateAttribute(curr, el)
		case relax.Choice:
			err = validateNode(curr, el)
			if err == nil {
				break
			}
			step, err1 := validateNodes(curr.Nodes[offset:], el)
			offset += step
			err = err1
		default:
			return fmt.Errorf("element: unsupported pattern %T", el)
		}
		if err != nil {
			return err
		}
	}
	return validateValue(curr, elem.Value)
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
		return fmt.Errorf("missing attribute: %s", attr.QualifiedName())
	}
	switch vs := attr.Value.(type) {
	case relax.Enum:
		ok := slices.Contains(vs.List, el.Attrs[ix].Value)
		if !ok {
			return fmt.Errorf("attribute value not acceptable")
		}
	case relax.Type:
		return validateType(vs, el.Attrs[ix].Value)
	case relax.Text:
	default:
		return fmt.Errorf("unsupported pattern for attribute")
	}
	return nil
}

func validateValue(node xml.Node, value relax.Pattern) error {
	if value == nil {
		return nil
	}
	var err error
	switch v := value.(type) {
	case relax.Text:
		err = validateText(node)
	case relax.Empty:
		err = validateEmpty(node)
	case relax.Type:
	case relax.TimeType:
		err = validateTime(node, v)
	case relax.IntType:
		err = validateInt(node, v)
	case relax.FloatType:
		err = validateFloat(node, v)
	case relax.StringType:
		err = validateString(node, v)
	default:
		return fmt.Errorf("type pattern not supported (%T)", v)
	}
	return err
}

var (
	errRange  = errors.New("value out of range")
	errLength = errors.New("invalid length")
	errFormat = errors.New("invalid format")
)

func validateInt(node xml.Node, value relax.IntType) error {
	val, err := strconv.ParseInt(node.Value(), 0, 64)
	if err != nil {
		return errFormat
	}
	if val < int64(value.MinValue) {
		return errRange
	}
	if val > int64(value.MaxValue) {
		return errRange
	}
	return nil
}

func validateFloat(node xml.Node, value relax.FloatType) error {
	val, err := strconv.ParseFloat(node.Value(), 64)
	if err != nil {
		return errFormat
	}
	if val < value.MinValue {
		return errRange
	}
	if val > value.MaxValue {
		return errRange
	}
	return nil
}

func validateTime(node xml.Node, value relax.TimeType) error {
	layout := "2006-01-02"
	if value.Format != "" {
		layout = value.Format
	}
	when, err := time.Parse(layout, node.Value())
	if err != nil {
		return errFormat
	}
	if !value.MinValue.IsZero() && when.Before(value.MinValue) {
		return errRange
	}
	if !value.MaxValue.IsZero() && when.After(value.MaxValue) {
		return errRange
	}
	return nil
}

func validateString(node xml.Node, value relax.StringType) error {
	var (
		err error
		str = node.Value()
	)
	if value.MinLength > 0 && len(str) < value.MinLength {
		return errLength
	}
	if value.MaxLength > 0 && len(str) > value.MaxLength {
		return errLength
	}
	switch value.Format {
	case "uri":
		_, err = url.Parse(str)
	case "hex":
		_, err = hex.DecodeString(str)
		return err
	case "base64":
		_, err = base64.StdEncoding.DecodeString(str)
	default:
	}
	if err != nil {
		return errFormat
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
