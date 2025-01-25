package sch

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"

	"github.com/midbel/codecs/xml"
)

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

func NewBuilder() *Builder {
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
	rs.OnElementOpen(xml.LocalName("ns"), b.onNs)
	rs.OnElementOpen(xml.LocalName("function"), b.onFunction)
	return b.schema, rs.Start()
}

func (b *Builder) createEnv() xml.Environ[xml.Expr] {
	switch b.context {
	case ctxSchema:
		return xml.Empty[xml.Expr]()
	case ctxSchema | ctxPattern:
		return xml.Enclosed[xml.Expr](b.schema)
	case ctxSchema | ctxPattern | ctxRule:
		x := len(b.schema.Patterns) - 1 
		if x < 0 {
			return xml.Empty[xml.Expr]()
		}
		return xml.Enclosed[xml.Expr](b.schema.Patterns[x])
	default:
		return xml.Empty[xml.Expr]()
	}
}

func (b *Builder) setPattern(p *Pattern) {
	b.schema.Patterns = append(b.schema.Patterns, p)
}

func (b *Builder) setRule(r *Rule) error {
	x := len(b.schema.Patterns) - 1
	if x < 0 {
		return fmt.Errorf("rule must be in a pattern element")
	}
	b.schema.Patterns[x].Rules = append(b.schema.Patterns[x].Rules, r)
	return nil
}

func (b *Builder) setAssert(a *Assert) error {
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

func (b *Builder) setLet(let Let) error {
	expr, err := compileExpr(let.Value)
	if err != nil {
		return nil
	}
	switch b.context {
	case ctxSchema:
		b.schema.Define(let.Ident, expr)
	case ctxSchema | ctxPattern:
		err = b.setLetToPattern(let.Ident, expr)
	case ctxSchema | ctxPattern | ctxRule:
		err = b.setLetToRule(let.Ident, expr)
	default:
		err = fmt.Errorf("invalid let element")
	}
	return err
}

func (b *Builder) setFunction(fn Function) error {
	var err error
	switch b.context {
	case ctxSchema:
		b.schema.Funcs.Define(fn.QualifiedName(), fn)
	case ctxSchema | ctxPattern:
		err = b.setFuncToPattern(fn)
	case ctxSchema | ctxPattern | ctxRule:
		err = b.setFuncToRule(fn)
	default:
		err = fmt.Errorf("invalid function element")
	}
	return err
}

func (b *Builder) setLetToPattern(ident string, value xml.Expr) error {
	x := len(b.schema.Patterns) - 1
	if x < 0 {
		return fmt.Errorf("no pattern element found")
	}
	b.schema.Patterns[x].Define(ident, value)
	return nil
}

func (b *Builder) setFuncToPattern(fn Function) error {
	x := len(b.schema.Patterns) - 1
	if x < 0 {
		return fmt.Errorf("no pattern element found")
	}
	b.schema.Patterns[x].Funcs.Define(fn.QualifiedName(), fn)
	return nil
}

func (b *Builder) setLetToRule(ident string, value xml.Expr) error {
	x := len(b.schema.Patterns) - 1
	if x < 0 {
		return fmt.Errorf("no pattern element found")
	}
	y := len(b.schema.Patterns[x].Rules) - 1
	if y < 0 {
		return fmt.Errorf("no rule element in pattern found")
	}
	b.schema.Patterns[x].Rules[y].Define(ident, value)
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
	b.schema.Patterns[x].Rules[y].Funcs.Define(fn.QualifiedName(), fn)
	return nil
}

func (b *Builder) onSchema(rs *xml.Reader, el *xml.Element, closed bool) error {
	b.context = ctxSchema
	if closed {
		return nil
	}
	bind, err := getAttribute(el, "queryBinding")
	if err != nil {
		return err
	}
	switch bind {
	case "xslt2", "xslt3":
		b.schema.Mode = xml.ModeXsl
	case "xpath2", "xpath3":
		b.schema.Mode = xml.ModeXpath
	default:
		return fmt.Errorf("%s: unsupported query binding value", bind)
	}
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

func (b *Builder) onNs(rs *xml.Reader, el *xml.Element, closed bool) error {
	if b.context != ctxSchema {
		return fmt.Errorf("namespace element only allowed in schema")
	}
	if !closed {
		return fmt.Errorf("ns element should be self closed")
	}
	return nil
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
		b.schema.Phases = append(b.schema.Phases, &ph)
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
		fn.args = append(fn.args, name)

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
			expr, err := compileExpr(normalizeSpace(value))
			if err != nil {
				return err
			}
			_, _ = name, expr
		}
		return err
	})
	sub.OnElement(xml.LocalName("value-of"), func(_ *xml.Reader, el *xml.Element, closed bool) error {
		code, err := getAttribute(el, "select")
		if err != nil {
			return err
		}
		expr, err := compileExpr(normalizeSpace(code))
		if err != nil {
			fmt.Println("valuf-of error", normalizeSpace(code))
			return err
		}
		fn.body = append(fn.body, expr)
		return nil
	})
	sub.OnElement(xml.LocalName("sequence"), func(_ *xml.Reader, el *xml.Element, closed bool) error {
		code, err := getAttribute(el, "select")
		if err != nil {
			return err
		}
		expr, err := compileExpr(normalizeSpace(code))
		if err != nil {
			fmt.Println("sequence error", normalizeSpace(code))
			return err
		}
		fn.body = append(fn.body, expr)
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
		Environ: b.createEnv(),
		Funcs:   xml.Empty[xml.Callable](),
	}
	p.Ident, _ = getAttribute(el, "id")
	b.setPattern(&p)
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
		Context: ctx,
		Environ: b.createEnv(),
		Funcs:   xml.Empty[xml.Callable](),
	}
	return b.setRule(&r)
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
	ast.Test = normalizeSpace(ast.Test)
	if ast.Flag, err = getAttribute(el, "flag"); err != nil {
		return err
	}
	if ast.Message, err = getValue(rs); err != nil {
		return err
	}
	return b.setAssert(&ast)
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
	let.Value = normalizeSpace(let.Value)
	return b.setLet(let)
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

func normalizeSpace(str string) string {
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
