package sch

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/midbel/codecs/xml"
)

type Schema struct {
	Title string
	xml.Environ

	Patterns  []*Pattern
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

type Pattern struct {
	Title string
	xml.Environ

	Rules []*Rule
}

type Rule struct {
	xml.Environ

	Context    string
	Expr       xml.Expr
	Assertions []*Assert
}

func (r *Rule) Eval() (bool, error) {
	return false, nil
}

type Assert struct {
	Ident   string
	Flag    string
	Test    string
	Message string
}

func (a *Assert) Eval(items []Item) (bool, error) {
	return false, nil
}

func Open(file string) (*Schema, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return parseSchema(r)
}

func parseSchema(r io.Reader) (*Schema, error) {
	var (
		rs  = xml.NewReader(r)
		sch Schema
	)
	sch.Environ = xml.Empty()
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
			return nil, fmt.Errorf("%s: unexpected element", node.QualifiedName())
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
		expr, err := compileExpr(let.Value)
		if err == nil {
			sch.Define(let.Ident, expr)
		} else {
			// fmt.Println("compilation fail", let.Value, err)
		}
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
		return unexpectedElement("top", node)
	}
}

func readPattern(rs *xml.Reader, sch *Schema) error {
	var pat Pattern
	pat.Environ = xml.Enclosed(sch)
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
			expr, err := compileExpr(let.Value)
			if err == nil {
				pat.Define(let.Ident, expr)
			} else {
				// fmt.Println("compilation fail", let.Value, err)
			}
		case "rule":
			rule, err := readRule(rs, el, pat)
			if !errors.Is(err, xml.ErrClosed) {
				return fmt.Errorf("rule: not closed")
			}
			pat.Rules = append(pat.Rules, rule)
		default:
			return unexpectedElement("pattern", el)
		}
	}
	sch.Patterns = append(sch.Patterns, &pat)
	return nil
}

func readRule(rs *xml.Reader, elem *xml.Element, env xml.Environ) (*Rule, error) {
	var (
		rule Rule
		err  error
	)
	rule.Environ = xml.Enclosed(env)
	if qn := elem.QualifiedName(); qn != "rule" {
		return nil, unexpectedElement("rule", elem)
	}
	if rule.Context, err = getAttribute(elem, "context"); err != nil {
		return nil, err
	}
	rule.Context = normalizeSpace(rule.Context)
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
			expr, err := compileExpr(let.Value)
			if err == nil {
				rule.Define(let.Ident, expr)
			} else {
				// fmt.Println("compilation fail", let.Value, err)
			}
		case "assert":
			ass, err := readAssert(rs, el)
			if err != nil {
				return nil, err
			}
			rule.Assertions = append(rule.Assertions, ass)
		default:
			return nil, unexpectedElement("rule", el)
		}
	}
	return &rule, nil
}

func readAssert(rs *xml.Reader, elem *xml.Element) (*Assert, error) {
	var (
		ass Assert
		err error
	)
	if qn := elem.QualifiedName(); qn != "assert" {
		return nil, unexpectedElement("assert", elem)
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
	let.Value = normalizeSpace(let.Value)
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
			return unexpectedElement("intro", node)
		}
		if _, err := getTitleElement(rs); err != nil {
			return err
		}
		return nil
	default:
		return unexpectedElement("intro", node)
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
	return normalizeSpace(text.Content), nil
}

func normalizeSpace(str string) string {
	var prev rune
	isSpace := func(r rune) bool {
		return r == ' ' || r == '\t'
	}
	clean := func(r rune) rune {
		if isSpace(r) && isSpace(prev) {
			return -1
		}
		if r == '\t' || r == '\n' {
			r = ' '
		}
		prev = r
		return r
	}
	return strings.Map(clean, str)
}

func getAttribute(elem *xml.Element, name string) (string, error) {
	ix := slices.IndexFunc(elem.Attrs, func(a xml.Attribute) bool {
		return a.Name == name
	})
	if ix < 0 {
		return "", fmt.Errorf("%s: attribute %q is missing", elem.QualifiedName(), name)
	}
	value := elem.Attrs[ix].Value()
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

func compileContext(expr string) (xml.Expr, error) {
	return xml.CompileMode(strings.NewReader(expr), xml.ModeXsl)
}

func compileExpr(expr string) (xml.Expr, error) {
	return xml.Compile(strings.NewReader(expr))
}

func unexpectedElement(ctx string, node xml.Node) error {
	return fmt.Errorf("%s: unexpected element %s", ctx, node.QualifiedName())
}
