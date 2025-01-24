package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"unicode"

	"github.com/midbel/codecs/xml"
)

func main() {
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer r.Close()

	b := New()
	sch, err := b.Build(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", " ")
	e.Encode(sch)
	// fmt.Println("done")
}

type buildContext int32

const (
	ctxSchema buildContext = 1 << iota
	ctxPattern
	ctxRule
	ctxAssert
	ctxPhase
	ctxActive
	ctxFunction
)

type Builder struct {
	schema  *Schema
	context buildContext
}

func New() *Builder {
	b := Builder{
		schema: Default(),
	}
	return &b
}

func (b *Builder) Build(r io.Reader) (*Schema, error) {
	rs := xml.NewReader(r)
	rs.OnElement(xml.LocalName("schema"), b.onSchema)
	rs.OnElement(xml.LocalName("title"), b.onTitle)
	rs.OnElement(xml.LocalName("pattern"), b.onPattern)
	rs.OnElement(xml.LocalName("rule"), b.onRule)
	rs.OnElement(xml.LocalName("assert"), b.onAssert)
	rs.OnElement(xml.LocalName("let"), b.onLet)
	rs.OnElementOpen(xml.LocalName("phase"), b.onPhase)
	rs.OnElementOpen(xml.LocalName("function"), b.onFunction)
	return b.schema, rs.Start()
}

func (b *Builder) setPattern(p Pattern) {
	b.schema.Patterns = append(b.schema.Patterns, p)
}

func (b *Builder) setRule(r Rule) error {
	x := len(b.schema.Patterns) - 1
	if x < 0 {
		return fmt.Errorf("rule must be in a pattern element")
	}
	b.schema.Patterns[x].Rules = append(b.schema.Patterns[x].Rules, r)
	return nil
}

func (b *Builder) setAssert(a Assert) error {
	x := len(b.schema.Patterns) - 1
	if x < 0 {
		return fmt.Errorf("rule must be in a pattern element")
	}
	y := len(b.schema.Patterns[x].Rules) - 1
	if y < 0 {
		return fmt.Errorf("assert must be in a rule element")
	}
	b.schema.Patterns[x].Rules[y].Asserts = append(b.schema.Patterns[x].Rules[y].Asserts, a)
	return nil
}

func (b *Builder) setLet(ident, value string) error {
	var err error
	switch b.context {
	case ctxSchema:
		b.schema.Variables[ident] = value
	case ctxSchema | ctxPattern:
		err = b.setLetToPattern(ident, value)
	case ctxSchema | ctxPattern | ctxRule:
		err = b.setLetToRule(ident, value)
	default:
		err = fmt.Errorf("invalid let element")
	}
	return err
}

func (b *Builder) setFunction(fn Function) error {
	var err error
	switch b.context {
	case ctxSchema:
		b.schema.Functions[fn.QualifiedName()] = fn
	case ctxSchema | ctxPattern:
		err = b.setFuncToPattern(fn)
	case ctxSchema | ctxPattern | ctxRule:
		err = b.setFuncToRule(fn)
	default:
		err = fmt.Errorf("invalid function element")
	}
	return err
}

func (b *Builder) setLetToPattern(ident, value string) error {
	x := len(b.schema.Patterns) - 1
	if x < 0 {
		return fmt.Errorf("no pattern element found")
	}
	b.schema.Patterns[x].Variables[ident] = value
	return nil
}

func (b *Builder) setFuncToPattern(fn Function) error {
	x := len(b.schema.Patterns) - 1
	if x < 0 {
		return fmt.Errorf("no pattern element found")
	}
	b.schema.Patterns[x].Functions[fn.QualifiedName()] = fn
	return nil
}

func (b *Builder) setLetToRule(ident, value string) error {
	x := len(b.schema.Patterns) - 1
	if x < 0 {
		return fmt.Errorf("no pattern element found")
	}
	y := len(b.schema.Patterns[x].Rules) - 1
	if y < 0 {
		return fmt.Errorf("no rule element in pattern found")
	}
	b.schema.Patterns[x].Rules[y].Variables[ident] = value
	return nil
}

func (b *Builder) setFuncToRule(fn Function) error {
	x := len(b.schema.Patterns) - 1
	if x < 0 {
		return fmt.Errorf("no pattern element found")
	}
	y := len(b.schema.Patterns[x].Rules) - 1
	if y < 0 {
		return fmt.Errorf("no rule element in pattern found")
	}
	b.schema.Patterns[x].Rules[y].Functions[fn.QualifiedName()] = fn
	return nil
}

func (b *Builder) onSchema(rs *xml.Reader, el *xml.Element, closed bool) error {
	b.context = ctxSchema
	if closed {
		return nil
	}
	attr, err := getAttribute(el, "queryBinding")
	if err != nil {
		return err
	}
	b.schema.Mode = attr
	return nil
}

func (b *Builder) onTitle(rs *xml.Reader, el *xml.Element, closed bool) error {
	if closed {
		return nil
	}
	if b.context != ctxSchema {
		return fmt.Errorf("title element only allowed in schema")
	}
	str, err := getValue(rs)
	if err == nil {
		b.schema.Title = str
	}
	return err
}

func (b *Builder) onPhase(rs *xml.Reader, el *xml.Element, closed bool) error {
	if b.context != ctxSchema {
		return fmt.Errorf("phase element only allowed in schema")
	}
	var (
		ph  Phase
		err error
	)
	ph.Ident, err = getAttribute(el, "id")
	if err != nil {
		return err
	}

	sub := rs.Sub()
	sub.OnElement(xml.LocalName("active"), func(_ *xml.Reader, el *xml.Element, closed bool) error {
		if !closed {
			return fmt.Errorf("active element should be self closed")
		}
		pattern, err := getAttribute(el, "pattern")
		if err == nil {
			ph.Actives = append(ph.Actives, pattern)
		}
		return err
	})
	sub.OnElementClosed(xml.LocalName("phase"), func(_ *xml.Reader, el *xml.Element, _ bool) error {
		b.schema.Phases = append(b.schema.Phases, ph)
		return xml.ErrBreak
	})
	if err := sub.Start(); err != nil {
		return err
	}
	return xml.ErrBreak
}

func (b *Builder) onFunction(rs *xml.Reader, el *xml.Element, closed bool) error {
	var fn Function
	name, err := getAttribute(el, "name")
	if err != nil {
		return err
	}
	fn.QName, err = xml.ParseName(name)
	if err != nil {
		return err
	}
	sub := rs.Sub()
	sub.OnElementClosed(xml.LocalName("function"), func(_ *xml.Reader, el *xml.Element, closed bool) error {
		b.setFunction(fn)
		return xml.ErrBreak
	})
	sub.OnElement(xml.LocalName("param"), func(_ *xml.Reader, el *xml.Element, closed bool) error {
		if !closed {
			return fmt.Errorf("param should be self closed")
		}
		name, err := getAttribute(el, "name")
		if err != nil {
			return err
		}
		as, _ := getAttribute(el, "as")
		_ = as
		fn.Params = append(fn.Params, name)

		return nil
	})
	sub.OnElement(xml.LocalName("variable"), func(rs *xml.Reader, el *xml.Element, closed bool) error {
		if closed {
			return nil
		}
		name, err := getAttribute(el, "name")
		if err != nil {
			return err
		}
		value, err := getAttribute(el, "select")
		if err != nil && errors.Is(err, ErrMissing) {
			value, err = getValue(rs)
		}
		if err == nil {
			_ = name
			fn.Body = append(fn.Body, cleanString(value))
		}
		return err
	})
	sub.OnElement(xml.LocalName("value-of"), func(_ *xml.Reader, el *xml.Element, closed bool) error {
		code, err := getAttribute(el, "select")
		if err != nil {
			return err
		}
		fn.Body = append(fn.Body, cleanString(code))
		return nil
	})
	sub.OnElement(xml.LocalName("sequence"), func(_ *xml.Reader, el *xml.Element, closed bool) error {
		code, err := getAttribute(el, "select")
		if err != nil {
			return err
		}
		fn.Body = append(fn.Body, cleanString(code))
		return nil
	})
	sub.OnElementOpen(xml.LocalName("choose"), func(_ *xml.Reader, el *xml.Element, closed bool) error {
		return xml.ErrDiscard
	})
	return sub.Start()
}

func (b *Builder) onPattern(rs *xml.Reader, el *xml.Element, closed bool) error {
	if closed {
		b.context = ctxSchema
		return nil
	}
	b.context |= ctxPattern
	if b.context != ctxPattern|ctxSchema {
		return fmt.Errorf("invalid pattern element")
	}
	p := Pattern{
		Variables: make(map[string]string),
		Functions: make(map[string]Function),
	}
	p.Ident, _ = getAttribute(el, "id")
	b.setPattern(p)
	return nil
}

func (b *Builder) onRule(rs *xml.Reader, el *xml.Element, closed bool) error {
	if closed {
		b.context = ctxPattern | ctxSchema
		return nil
	}
	b.context |= ctxRule
	if b.context != ctxRule|ctxPattern|ctxSchema {
		return fmt.Errorf("invalid rule element")
	}
	ctx, err := getAttribute(el, "context")
	if err != nil {
		return err
	}
	r := Rule{
		Context:   ctx,
		Variables: make(map[string]string),
		Functions: make(map[string]Function),
	}
	return b.setRule(r)
}

func (b *Builder) onAssert(rs *xml.Reader, el *xml.Element, closed bool) error {
	if closed {
		b.context = ctxRule | ctxPattern | ctxSchema
		return nil
	}
	b.context |= ctxAssert
	if b.context != ctxAssert|ctxRule|ctxPattern|ctxSchema {
		return fmt.Errorf("invalid assert element")
	}
	var (
		ast Assert
		err error
	)
	if ast.Ident, err = getAttribute(el, "id"); err != nil {
		return err
	}
	if ast.Test, err = getAttribute(el, "test"); err != nil {
		return err
	}
	ast.Test = cleanString(ast.Test)
	if ast.Level, err = getAttribute(el, "flag"); err != nil {
		return err
	}
	if ast.Message, err = getValue(rs); err != nil {
		return err
	}
	return b.setAssert(ast)
}

func (b *Builder) onLet(rs *xml.Reader, el *xml.Element, closed bool) error {
	var (
		let Let
		err error
	)
	if !closed {
		return fmt.Errorf("let element should not have any children")
	}
	let.Ident, err = getAttribute(el, "name")
	if err != nil {
		return err
	}
	let.Value, err = getAttribute(el, "value")
	if err != nil {
		return err
	}
	let.Value = cleanString(let.Value)
	return b.setLet(let.Ident, let.Value)
}

type Schema struct {
	Title     string
	Mode      string
	Phases    []Phase
	Patterns  []Pattern
	Variables map[string]string
	Functions map[string]Function
}

func Default() *Schema {
	s := Schema{
		Mode:      "xpath",
		Variables: make(map[string]string),
		Functions: make(map[string]Function),
	}
	return &s
}

type Phase struct {
	Ident   string
	Actives []string
}

type Pattern struct {
	Ident     string
	Rules     []Rule
	Variables map[string]string
	Functions map[string]Function
}

type Rule struct {
	Context   string
	Asserts   []Assert
	Variables map[string]string
	Functions map[string]Function
}

type Assert struct {
	Ident   string
	Level   string
	Test    string
	Message string
}

type Let struct {
	Ident string
	Value string
}

type Function struct {
	xml.QName
	Params []string
	Body   []string
}

func getValue(rs *xml.Reader) (string, error) {
	text, err := rs.Read()
	if err != nil {
		return "", err
	}
	return text.Value(), nil
}

var ErrMissing = errors.New("missing")

func getAttribute(el *xml.Element, name string) (string, error) {
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.LocalName() == name
	})
	if ix < 0 {
		return "", fmt.Errorf("%s: %w %s", name, ErrMissing, "attribute")
	}
	return el.Attrs[ix].Value(), nil
}

func cleanString(str string) string {
	var prev rune
	str = strings.TrimSpace(str)
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			r = ' '
		}
		if unicode.IsSpace(r) && unicode.IsSpace(prev) {
			return -1
		}
		prev = r
		return r
	}, str)
}
