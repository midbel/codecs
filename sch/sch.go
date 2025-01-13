package sch

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"slices"
	"strings"

	"github.com/midbel/codecs/xml"
)

var ErrAssert = errors.New("assertion error")

type ListError struct {
	Errors []error
}

func (e *ListError) ErrOrNil() error {
	if e.Zero() {
		return nil
	}
	return e
}

func (e *ListError) Zero() bool {
	return len(e.Errors) == 0
}

func (e *ListError) Push(err error) {
	if e1, ok := err.(*ListError); ok {
		e.Errors = slices.Concat(e.Errors, e1.Errors)
	} else {
		e.Errors = append(e.Errors, err)
	}
}

func (e *ListError) Error() string {
	return fmt.Sprintf("%d errors detected", len(e.Errors))
}

type Namespace struct {
	URI    string
	Prefix string
}

type Let struct {
	Ident string
	Value string
}

type Schema struct {
	Title string
	xml.Environ

	Patterns  []*Pattern
	Spaces    []*Namespace
	Functions []*Function
}

func Exec(file string) error {
	schema, err := Open(file)
	if err != nil {
		return err
	}
	_ = schema
	return nil
}

func Open(file string) (*Schema, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return parseSchema(r)
}

func (s *Schema) Asserts() iter.Seq[*Assert] {
	it := func(yield func(*Assert) bool) {
		for _, p := range s.Patterns {
			for a := range p.Asserts() {
				ok := yield(a)
				if !ok {
					return
				}
			}
		}
	}
	return it
}

type Function struct{}

type Pattern struct {
	Title string
	xml.Environ

	Rules []*Rule
}

func (p *Pattern) Exec(doc *xml.Document) error {
	var list ListError
	for _, r := range p.Rules {
		err := r.Exec(doc)
		if err != nil {
			list.Push(err)
			continue
		}
	}
	return list.ErrOrNil()
}

func (p *Pattern) Asserts() iter.Seq[*Assert] {
	it := func(yield func(*Assert) bool) {
		for _, r := range p.Rules {
			for _, a := range r.Asserts {
				ok := yield(a)
				if !ok {
					return
				}
			}
		}
	}
	return it
}

type Rule struct {
	xml.Environ

	Title   string
	Context string
	Asserts []*Assert
}

func (r *Rule) Count(doc *xml.Document) (int, error) {
	expr, err := compileContext(r.Context)
	if err != nil {
		return 0, err
	}
	var items []xml.Item
	if f, ok := expr.(interface {
		FindWithEnv(xml.Node, xml.Environ) ([]xml.Item, error)
	}); ok {
		items, err = f.FindWithEnv(doc, xml.Enclosed(r))
	} else {
		items, err = expr.Find(doc)
	}
	return len(items), err
}

func (r *Rule) Exec(doc *xml.Document) error {
	items, err := r.getItems(doc)
	if err != nil {
		return err
	}
	var list ListError
	for _, a := range r.Asserts {
		ok, err := a.Eval(items, r)
		if err != nil {
			if !errors.Is(err, ErrAssert) {
				return err
			}
			list.Push(err)
		}
		_ = ok
	}
	return list.ErrOrNil()
}

func (r *Rule) getItems(doc *xml.Document) ([]xml.Item, error) {
	expr, err := compileContext(r.Context)
	if err != nil {
		return nil, err
	}
	var items []xml.Item
	if f, ok := expr.(interface {
		FindWithEnv(xml.Node, xml.Environ) ([]xml.Item, error)
	}); ok {
		items, err = f.FindWithEnv(doc, r)
	} else {
		items, err = expr.Find(doc)
	}
	return items, err
}

type Assert struct {
	Ident   string
	Flag    string
	Test    string
	Message string
}

func (a *Assert) Eval(items []xml.Item, env xml.Environ) (bool, error) {
	test, err := compileExpr(a.Test)
	if err != nil {
		return false, err
	}
	for i := range items {
		res, err := items[i].Assert(test, env)
		if err != nil {
			return false, err
		}
		ok := isTrue(res)
		if !ok {
			return ok, fmt.Errorf("%w: %s", ErrAssert, a.Message)
		}
	}
	return true, nil
}

func isTrue(res []xml.Item) bool {
	if len(res) == 0 {
		return false
	}
	var ok bool
	if !res[0].Atomic() {
		return true
	}
	switch res := res[0].Value().(type) {
	case bool:
		ok = res
	case float64:
		ok = res != 0
	case string:
		ok = res != ""
	default:
	}
	return ok	
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
			rule.Asserts = append(rule.Asserts, ass)
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
