package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"iter"

	"github.com/midbel/codecs/xml"
)

type Schema struct {
	Title string

	Patterns  []*Pattern
	Variables []*Let
	Spaces    []*Namespace
	Functions []*Function
}

type Namespace struct {
	URI    string
	Prefix string
}

type Let struct {
	Ident string
	Value string
}

type Function struct{}

type Assert struct {
	Ident   string
	Flag    string
	Test    string
	Message string
}

type Rule struct {
	Context    string
	Assertions []*Assert
	Variables  []*Let
}

type Pattern struct {
	Title     string
	Rules     []*Rule
	Variables []*Let
}

func main() {
	flag.Parse()
	sch, err := parseSchema(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for a := range getAssertions(sch) {
		fmt.Printf("%-16s | %-8s | %-s", a.Ident, a.Flag, a.Message)
		fmt.Println()
	}
}

func getAssertions(sch *Schema) iter.Seq[Assert] {
	return func(yield func(Assert) bool) {
		for _, p := sch.Patterns {
			for _, r := range p.Rules {
				for _, a := range r.Assertions {
					if !yield(a) {
						return
					}
				}
			}
		}

	}
}

func parseSchema(file string) (*Schema, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var (
		rs  = xml.NewReader(r)
		sch Schema
	)
	if err := readIntro(rs); err != nil {
		return nil, err
	}
	for {
		node, err := rs.Read()
		if errors.Is(err, xml.ErrClosed) && node.QualifiedName() == "schema" {
			break
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil && !errors.Is(err, xml.ErrClosed) {
			return nil, err
		}
		switch node := node.(type) {
		case *xml.Element:
			err = readTop(rs, node, &sch)
		case *xml.Instruction:
		case *xml.Comment:
		default:
			fmt.Fprintln(os.Stderr, "unexpected element type")
		}
		if err != nil {
			return nil, err
		}
	}
	return &sch, nil
}

func readTop(rs *xml.Reader, node *xml.Element, sch *Schema) error {
	switch node.QualifiedName() {
	case "pattern":
		return readPattern(rs, sch)
	case "let":
		let, err := readLet(rs, node)
		if err != nil {
			return err
		}
		sch.Variables = append(sch.Variables, let)
		return nil
	case "ns":
		ns, err := readNS(rs, node)
		if err != nil {
			return err
		}
		sch.Spaces = append(sch.Spaces, ns)
		return nil
	case "function":
		return readFunction(rs, node)
	default:
		return fmt.Errorf("%s: unexpected element", node.QualifiedName())
	}
}

func readPattern(rs *xml.Reader, sch *Schema) error {
	var pat Pattern
	for {
		el, err := getElementFromReaderMaybeClosed(rs, "pattern")
		if err != nil {
			if errors.Is(err, xml.ErrClosed) {
				break
			}
			return err
		}
		switch qn := el.QualifiedName(); qn {
		case "let":
			let, err := readLet(rs, el)
			if err != nil && !errors.Is(err, xml.ErrClosed) {
				return err
			}
			pat.Variables = append(pat.Variables, let)
		case "rule":
			rule, err := readRule(rs, el)
			if !errors.Is(err, xml.ErrClosed) {
				return fmt.Errorf("missing closing rule element")
			}
			pat.Rules = append(pat.Rules, rule)
		default:
			return fmt.Errorf("pattern: unexpected %s element", qn)
		}
	}
	sch.Patterns = append(sch.Patterns, &pat)
	return nil
}

func readRule(rs *xml.Reader, elem *xml.Element) (*Rule, error) {
	var (
		rule Rule
		err  error
	)
	if qn := elem.QualifiedName(); qn != "rule" {
		return nil, fmt.Errorf("%s: unexpected element", qn)
	}
	if rule.Context, err = getAttribute(elem, "context"); err != nil {
		return nil, err
	}
	for {
		el, err := getElementFromReaderMaybeClosed(rs, "rule")
		if err != nil {
			return &rule, err
		}
		switch qn := el.QualifiedName(); qn {
		case "let":
			let, err := readLet(rs, el)
			if err != nil && !errors.Is(err, xml.ErrClosed) {
				return nil, err
			}
			rule.Variables = append(rule.Variables, let)
		case "assert":
			ass, err := readAssert(rs, el)
			if err != nil {
				return nil, err
			}
			rule.Assertions = append(rule.Assertions, ass)
		default:
			return nil, fmt.Errorf("rule: unexpected %s element", qn)
		}
	}
	fmt.Printf("rule: %+v\n", rule)
	return &rule, nil
}

func readAssert(rs *xml.Reader, elem *xml.Element) (*Assert, error) {
	var (
		ass Assert
		err error
	)
	if qn := elem.QualifiedName(); qn != "assert" {
		return nil, fmt.Errorf("%s: unexpected element", qn)
	}
	if ass.Test, err = getAttribute(elem, "test"); err != nil {
		return nil, err
	}
	ass.Ident, _ = getAttribute(elem, "id")
	ass.Flag, _ = getAttribute(elem, "flag")

	ass.Message, err = getStringFromReader(rs)
	if err != nil {
		return nil, err
	}
	fmt.Printf("assert: %+v\n", ass)
	return &ass, isClosed(rs, "assert")
}

func readLet(rs *xml.Reader, elem *xml.Element) (*Let, error) {
	var (
		let Let
		err error
	)
	let.Ident, err = getAttribute(elem, "name")
	if err != nil {
		return nil, err
	}
	let.Value, err = getAttribute(elem, "value")
	if err != nil {
		return nil, err
	}
	fmt.Printf("let: %+v\n", let)
	return &let, nil
}

func readNS(rs *xml.Reader, elem *xml.Element) (*Namespace, error) {
	var (
		ns  Namespace
		err error
	)
	ns.URI, err = getAttribute(elem, "uri")
	if err != nil {
		return nil, err
	}
	ns.Prefix, err = getAttribute(elem, "prefix")
	if err != nil {
		return nil, err
	}
	fmt.Printf("namespace: %+v\n", ns)
	return &ns, nil
}

func readFunction(rs *xml.Reader, elem *xml.Element) error {
	for {
		el, err := rs.Read()
		if errors.Is(err, xml.ErrClosed) && el.QualifiedName() == "function" {
			break
		}
	}
	return nil
}

func readIntro(rs *xml.Reader) error {
	node, err := rs.Read()
	if err != nil && !errors.Is(err, xml.ErrClosed) {
		return err
	}
	switch node := node.(type) {
	case *xml.Instruction:
		return readIntro(rs)
	case *xml.Comment:
		return readIntro(rs)
	case *xml.Element:
		if node.QualifiedName() != "schema" {
			return fmt.Errorf("expected schema element")
		}
		if _, err := getTitleElement(rs); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("expected schema element")
	}
}

func isClosed(rs *xml.Reader, name string) error {
	node, err := rs.Read()
	if !errors.Is(err, xml.ErrClosed) {
		return fmt.Errorf("expected closing element")
	}
	if _, err := getElement(node); err != nil {
		return err
	}
	if node.QualifiedName() != name {
		return fmt.Errorf("expected closing element for %s", name)
	}
	return nil
}

func getElementFromReaderMaybeClosed(rs *xml.Reader, name string) (*xml.Element, error) {
	node, err := rs.Read()
	if node.QualifiedName() == name && errors.Is(err, xml.ErrClosed) {
		return nil, err
	}
	if err != nil && !errors.Is(err, xml.ErrClosed) {
		return nil, err
	}
	switch el := node.(type) {
	case *xml.Comment:
		return getElementFromReaderMaybeClosed(rs, name)
	case *xml.Element:
		return el, nil
	default:
		return nil, fmt.Errorf("unexpected xml type")
	}
}

func getElementFromReader(rs *xml.Reader) (*xml.Element, error) {
	node, err := rs.Read()
	if err != nil {
		return nil, err
	}
	if _, ok := node.(*xml.Comment); ok {
		return getElementFromReader(rs)
	}
	return getElement(node)
}

func getElement(node xml.Node) (*xml.Element, error) {
	el, ok := node.(*xml.Element)
	if !ok {
		return nil, fmt.Errorf("%s: xml element expected", node.QualifiedName())
	}
	return el, nil
}

func getTextFromReader(rs *xml.Reader) (*xml.Text, error) {
	node, err := rs.Read()
	if err != nil {
		return nil, err
	}
	return getText(node)
}

func getText(node xml.Node) (*xml.Text, error) {
	el, ok := node.(*xml.Text)
	if !ok {
		return nil, fmt.Errorf("text element expected")
	}
	return el, nil
}

func getStringFromReader(rs *xml.Reader) (string, error) {
	text, err := getTextFromReader(rs)
	if err != nil {
		return "", err
	}
	return text.Content, nil
}

func getAttribute(elem *xml.Element, name string) (string, error) {
	ix := slices.IndexFunc(elem.Attrs, func(a xml.Attribute) bool {
		return a.Name == name
	})
	if ix < 0 {
		return "", fmt.Errorf("%s: attribute %q is missing", elem.QualifiedName(), name)
	}
	value := elem.Attrs[ix].Value
	value = strings.Map(func(r rune) rune {
		if r == '\r' || r == '\t' {
			return -1
		}
		if r == '\n' {
			return ' '
		}
		return r
	}, value)
	return strings.TrimSpace(value), nil
}

func getTitleElement(rs *xml.Reader) (string, error) {
	el, err := getElementFromReader(rs)
	if err != nil {
		return "", err
	}
	if el.QualifiedName() != "title" {
		return "", fmt.Errorf("title element expected")
	}
	title, err := getStringFromReader(rs)
	if err != nil {
		return "", err
	}
	return title, isClosed(rs, "title")
}
