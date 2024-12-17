package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/midbel/codecs/xml"
)

type Env struct {
	values map[string]any
}

func Empty() *Env {
	e := Env{
		values: make(map[string]any),
	}
	return &e
}

func (e *Env) Resolve(ident string) (any, error) {
	return nil, nil
}

func (e *Env) Define(ident string, value any) error {
	return nil
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

func (a Assert) Execute(doc *xml.Document) error {
	return nil
}

type Rule struct {
	Context    string
	Assertions []Assert
	env        *Env
}

func (r Rule) Execute(doc *xml.Document) error {
	return nil
}

type Pattern struct {
	Title string
	Rules []Rule
}

func (p Pattern) Execute(doc *xml.Document) error {
	return nil
}

func main() {
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer r.Close()

	rs := xml.NewReader(r)
	if err := readIntro(rs); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for {
		node, err := rs.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil && !errors.Is(err, xml.ErrClosed) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		switch node := node.(type) {
		case *xml.Element:
			err = readTop(rs, node)
		case *xml.Instruction:
		case *xml.Comment:
		default:
			fmt.Fprintln(os.Stderr, "unexpected element type")
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}

func readTop(rs *xml.Reader, node *xml.Element) error {
	switch node.LocalName() {
	case "pattern":
		return readPattern(rs)
	case "let":
		return readLet(rs)
	case "ns":
		return readNS(rs)
	case "function":
		return readFunction(rs)
	default:
		return fmt.Errorf("%s: unexpected element", node.LocalName())
	}
}

func readPattern(rs *xml.Reader) error {
	var (
		pat Pattern
		err error
	)
	if pat.Title, err = getTitleElement(rs); err != nil {
		return err
	}
	for {
		el, err := getElementFromReader(rs)
		if err != nil {
			if errors.Is(err, xml.ErrClosed) {
				break
			}
			return err
		}
		rule, err := readRule(rs, el)
		if !errors.Is(err, xml.ErrClosed) {
			return fmt.Errorf("missing closing rule element")
		}
		pat.Rules = append(pat.Rules, rule)
	}
	fmt.Printf("pattern: %+v\n", pat)
	return nil
}

func readRule(rs *xml.Reader, elem *xml.Element) (Rule, error) {
	var (
		rule Rule
		err  error
	)
	if qn := elem.QualifiedName(); qn != "rule" {
		return rule, fmt.Errorf("%s: unexpected element", qn)
	}
	ctx, err := getAttribute(elem, "context")
	if err != nil {
		return rule, nil
	}
	if _, err := xml.Compile(strings.NewReader(ctx)); err != nil {
		return rule, err
	}
	rule.Context = ctx
	// if rule.Context, err = getAttribute(elem, "context"); err != nil {
	// 	return rule, err
	// }
	rule.env = Empty()
	for {
		el, err := getElementFromReader(rs)
		if err != nil {
			return rule, err
		}
		ass, err := readAssert(rs, el)
		if err != nil {
			return rule, err
		}
		rule.Assertions = append(rule.Assertions, ass)
	}
}

func readAssert(rs *xml.Reader, elem *xml.Element) (Assert, error) {
	var (
		ass Assert
		err error
	)
	if qn := elem.QualifiedName(); qn != "assert" {
		return ass, fmt.Errorf("%s: unexpected element", qn)
	}
	test, err := getAttribute(elem, "test")
	if err != nil {
		return ass, err
	}
	if _, err := xml.Compile(strings.NewReader(test)); err != nil {
		return ass, err
	}
	ass.Test = test
	ass.Ident, _ = getAttribute(elem, "id")
	ass.Flag, _ = getAttribute(elem, "flag")

	ass.Message, err = getStringFromReader(rs)
	if err != nil {
		return ass, err
	}
	return ass, isClosed(rs, "assert")
}

func readLet(rs *xml.Reader) error {
	return nil
}

func readNS(rs *xml.Reader) error {
	return nil
}

func readFunction(rs *xml.Reader) error {
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
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	switch node := node.(type) {
	case *xml.Instruction:
		return readIntro(rs)
	case *xml.Comment:
		return readIntro(rs)
	case *xml.Element:
		if node.LocalName() != "schema" {
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
	if node.LocalName() != name {
		return fmt.Errorf("expected closing element for %s", name)
	}
	return nil
}

func getElementFromReader(rs *xml.Reader) (*xml.Element, error) {
	node, err := rs.Read()
	if err != nil {
		return nil, err
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
	return elem.Attrs[ix].Value, nil
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
