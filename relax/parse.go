package relax

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
)

type ParseError struct {
	Position
	Element string
	Message string
}

func (p ParseError) Error() string {
	return fmt.Sprintf("%d:%d: %s: %s", p.Line, p.Column, p.Element, p.Message)
}

type Parser struct {
	scan *Scanner
	curr Token
	peek Token

	spaces map[string]string
	types  map[string]string
}

func Parse(r io.Reader) *Parser {
	p := Parser{
		scan:   Scan(r),
		spaces: make(map[string]string),
		types:  make(map[string]string),
	}
	p.next()
	p.next()
	return &p
}

func (p *Parser) Parse() (Pattern, error) {
	pattern, err := p.parse()
	return pattern, err
}

func (p *Parser) parse() (Pattern, error) {
	if err := p.parseDeclarations(); err != nil {
		return nil, err
	}
	p.skipEOL()
	p.skipComment()
	for p.isKeyword("include") {

	}
	switch {
	case p.isKeyword("element"):
		return p.parseElement()
	case p.isKeyword("start"):
		return p.parseDefinitions()
	default:
		msg := fmt.Sprintf("want element/start keyword but got %s", p.curr.Literal)
		return nil, p.createError("parser", msg)
	}
}

func (p *Parser) parseInclude() (Pattern, error) {
	p.next()
	if !p.is(Literal) {
		return nil, p.createError("include", "include URL should be in quoted string")
	}
	r, err := os.Open(p.curr.Literal)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	ps := Parse(r)
	return ps.Parse()
}

func (p *Parser) parseDeclarations() error {
	ok := func() bool {
		return p.isKeyword("include") ||
			p.isKeyword("start") ||
			p.isKeyword("element")
	}
	for !p.done() {
		p.skipEOL()
		p.skipComment()

		if !p.is(Keyword) || ok() {
			break
		}
		if err := p.parseNamespace(); err != nil {
			return err
		}
	}
	return nil
}

func (p *Parser) parseNamespace() error {
	switch {
	case p.isKeyword("default"):
		p.next()
		return p.parseNamespace()
	case p.isKeyword("namespace"):
		p.next()
	default:
		msg := fmt.Sprintf("want default/namespace keyword but got %s", p.curr.Literal)
		return p.createError("namespace", msg)
	}
	name := p.curr.Literal
	p.next()
	if !p.is(Assign) {
		return p.createError("namespace", "missing assignment operator (\"=\") after namespace")
	}
	p.next()
	if !p.is(Literal) {
		return p.createError("namespace", "namespace URL should be in quoted string")
	}

	p.spaces[name] = p.curr.Literal
	p.next()
	return nil
}

func (p *Parser) parseDefinitions() (Pattern, error) {
	var (
		start Pattern
		err   error
	)
	if start, err = p.parseStartPattern(); err != nil {
		return nil, err
	}

	register := func(name string, elem Pattern, patterns map[string]Pattern) map[string]Pattern {
		parent, ok := patterns[name]
		if !ok {
			patterns[name] = elem
			return patterns
		}
		if c, ok := parent.(Choice); ok {
			c.List = append(c.List, elem)
			parent = c
		} else {
			var c Choice
			c.List = append(c.List, parent, elem)
			parent = c
		}
		patterns[name] = parent
		return patterns
	}
	patterns := make(map[string]Pattern)
	for !p.done() {
		p.skipComment()
		if !p.is(Name) {
			return nil, p.createError("pattern", "pattern should be a name")
		}
		var (
			name  = p.curr.Literal
			merge bool
		)
		p.next()
		switch {
		case p.is(Assign):
		case p.is(MergeAlt):
			merge = true
		default:
			return nil, p.createError("pattern", "missing assignment operator (\"=\") after name")
		}
		p.next()
		elem, err := p.parseElement()
		if err != nil {
			return nil, err
		}
		if merge {
			patterns = register(name, elem, patterns)
		} else {
			patterns[name] = elem
		}
		p.skipEOL()
	}
	return reassemble(start, patterns)
}

func (p *Parser) parseStartPattern() (Pattern, error) {
	defer p.skipEOL()
	if !p.isKeyword("start") {
		msg := fmt.Sprintf("expected start keyword but got %s", p.curr.Literal)
		return nil, p.createError("start", msg)
	}
	p.next()
	if !p.is(Assign) {
		return nil, p.createError("start", "missing assignment operator (\"=\") after start")
	}
	p.next()
	if p.is(Name) {
		return p.parseLink()
	}
	return p.parseElement()
}

func (p *Parser) parseLink() (Pattern, error) {
	var ref Link
	ref.Ident = p.curr.Literal
	p.next()
	ref.cardinality = p.parseCardinality()
	return ref, nil
}

func (p *Parser) parseList() (Pattern, error) {
	var grp Group
	for p.is(Keyword) || p.is(Name) {
		var (
			pat Pattern
			err error
		)
		switch {
		case p.isKeyword("element"):
			pat, err = p.parseElement()
		case p.isKeyword("attribute"):
			pat, err = p.parseAttribute()
		case p.is(Name):
			pat, err = p.parseLink()
		default:
			msg := fmt.Sprintf("expected element/attribute keyword or a name but got %s", p.curr.Literal)
			return nil, p.createError("pattern", msg)
		}
		if err != nil {
			return nil, err
		}
		grp.List = append(grp.List, pat)
		if !p.is(Comma) {
			break
		}
		p.next()
	}
	if len(grp.List) == 1 {
		return grp.List[0], nil
	}
	return grp, nil
}

func (p *Parser) parseGroup() (Pattern, error) {
	p.next()
	var grp Group
	for !p.done() && !p.is(EndParen) {
		var (
			el  Pattern
			err error
		)
		switch {
		case p.isKeyword("element"):
			el, err = p.parseElement()
		case p.is(Name):
			el, err = p.parseLink()
		default:
			msg := fmt.Sprintf("expected element keyword or a name but got %s", p.curr.Literal)
			return nil, p.createError("group", msg)
		}
		if err != nil {
			return nil, err
		}
		grp.List = append(grp.List, el)
		switch {
		case p.is(Comma):
			p.next()
		case p.is(EndParen):
		default:
			return nil, p.createError("group", "only \")\" or \",\" after pattern is allowed")
		}
	}
	if !p.is(EndParen) {
		return nil, p.createError("group", "missing \")\" at end of pattern")
	}
	p.next()
	if len(grp.List) == 1 {
		return grp.List[0], nil
	}
	return grp, nil
}

func (p *Parser) parseChoice() (Pattern, error) {
	p.next()
	var ch Choice
	for !p.done() && !p.is(EndParen) {
		var (
			el  Pattern
			err error
		)
		switch {
		case p.is(Keyword) || p.is(Name):
			el, err = p.parseList()
		case p.is(BegParen):
			el, err = p.parseGroup()
		default:
			return nil, p.createError("choice", "expected one of keyword/name/group")
		}
		if err != nil {
			return nil, err
		}
		ch.List = append(ch.List, el)
		switch {
		case p.is(Alt):
			p.next()
		case p.is(EndParen):
		default:
			return nil, p.createError("choice", "only \")\" or \"|\" after pattern is allowed")
		}
	}
	if !p.is(EndParen) {
		return nil, p.createError("choice", "missing \")\" at end of pattern")
	}
	p.next()
	return ch, nil
}

func (p *Parser) parseElement() (Pattern, error) {
	p.next()
	var (
		el  Element
		err error
	)
	if el.QName, err = p.parseName(); err != nil {
		return nil, err
	}
	if !p.is(BegBrace) {
		return nil, p.createError("element", "missing \"{\" at beginning of pattern")
	}
	p.next()
	p.skipEOL()
	p.skipComment()
	for {
		var (
			pat Pattern
			err error
		)
		switch {
		case p.is(Name):
			pat, err = p.parseLink()
		case p.is(BegParen):
			pat, err = p.parseChoice()
		case p.isKeyword("attribute"):
			pat, err = p.parseAttribute()
		case p.isKeyword("element"):
			pat, err = p.parseElement()
		default:
			// msg := fmt.Sprintf("expected element/attribute keyword or a name but got %s", p.curr.Literal)
			// return nil, p.createError("element", msg)
		}
		if err != nil {
			return nil, err
		}
		if pat == nil {
			break
		}
		el.Patterns = append(el.Patterns, pat)

		switch {
		case p.is(Comma):
			p.next()
			if p.is(EndBrace) {
				return nil, p.createError("element", "\"}\" can not be used after \",\"")
			}
		case p.is(EndBrace):
		default:
			return nil, p.createError("element", "only \"}\" or \",\" after pattern is allowed")
		}
	}
	p.skipEOL()
	p.skipComment()
	switch {
	case p.isType():
		t, err := p.parseType()
		if err != nil {
			return nil, err
		}
		el.Value = t
	case p.isKeyword("text"):
		p.next()
		el.Value = Text{}
	case p.isKeyword("empty"):
		el.Value = Empty{}
	case p.is(Literal):
		el.Value, err = p.parseEnum()
		if err != nil {
			return nil, err
		}
	case p.is(EOL) || p.is(Comment):
		p.skipEOL()
		p.skipComment()
	case p.is(EndBrace):
	default:
		return nil, p.createError("element", "expected one of type/text/empty pattern")
	}
	if !p.is(EndBrace) {
		return nil, p.createError("element", "missing \"}\" at end of pattern")
	}
	p.next()
	el.cardinality = p.parseCardinality()
	return el, nil
}

func (p *Parser) parseAttribute() (Pattern, error) {
	p.next()
	var (
		at  Attribute
		err error
	)
	if at.QName, err = p.parseName(); err != nil {
		return nil, err
	}
	if !p.is(BegBrace) {
		return nil, p.createError("attribute", "missing \"{\" at beginning of pattern")
	}
	p.next()
	switch {
	case p.isKeyword("text"):
		p.next()
		at.Value = Text{}
	case p.is(Literal):
		at.Value, err = p.parseEnum()
		if err != nil {
			return nil, err
		}
	default:
		return nil, p.createError("attribute", "expected one of text/type/enum pattern")
	}
	if !p.is(EndBrace) {
		return nil, p.createError("attribute", "missing \"}\" at end of pattern")
	}
	p.next()
	at.cardinality = p.parseCardinality()
	if at.cardinality > 0 && at.cardinality != ZeroOrOne {
		return nil, p.createError("attribute", "arity for attribute can only be one of \"+\" or \"?\"")
	}
	if at.cardinality == 0 {
		at.cardinality = One
	}
	return at, nil
}

func (p *Parser) parseName() (QName, error) {
	var q QName
	if !p.is(Name) {
		return q, p.createError("name", "an element name is missing")
	}
	q.Local = p.curr.Literal
	p.next()
	if p.is(Colon) {
		p.next()
		if !p.is(Name) {
			return q, p.createError("name", "an element name is missing")
		}
		defer p.next()
		q.Space = q.Local
		q.Local = p.curr.Literal
	}
	return q, nil
}

func (p *Parser) parseCardinality() cardinality {
	var value cardinality
	switch {
	case p.is(Mandatory):
		value = OneOrMore
	case p.is(Optional):
		value = ZeroOrOne
	case p.is(Star):
		value = ZeroOrMore
	default:
	}
	if value != 0 {
		p.next()
	}
	return value
}

func (p *Parser) parseType() (Pattern, error) {
	defer p.skipEOL()
	t := Type{
		Name: p.curr.Literal,
	}
	p.next()
	if !p.is(BegBrace) {
		return t, nil
	}
	switch t.Name {
	case "int":
		return p.parseTypeInt(t)
	case "float", "decimal":
		return p.parseTypeFloat(t)
	case "date":
		return p.parseTypeDate(t)
	case "string":
		return p.parseTypeString(t)
	case "bool":
		return t, nil
	default:
		return nil, p.createError("type", fmt.Sprintf("%s is not a supported type", t.Name))
	}
}

func (p *Parser) parseTypeString(t Type) (Pattern, error) {
	res := StringType{
		Type: t,
	}
	err := p.parseParameters(func(name, value string) error {
		switch name {
		case "format":
			res.Format = value
		case "minLength":
			n, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			res.MinLength = n
		case "maxLength":
			n, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			res.MaxLength = n
		default:
			return p.createError("string", fmt.Sprintf("%s is not a supported string parameter", name))
		}
		return nil
	})
	return res, err
}

func (p *Parser) parseTypeInt(t Type) (Pattern, error) {
	res := IntType{
		Type: t,
	}
	err := p.parseParameters(func(name, value string) error {
		switch name {
		case "minValue":
			n, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			res.MinValue = int(n)
		case "maxValue":
			n, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			res.MaxValue = int(n)
		default:
			return p.createError("int", fmt.Sprintf("%s is not a supported integer parameter", name))
		}
		return nil
	})
	return res, err
}

func (p *Parser) parseTypeFloat(t Type) (Pattern, error) {
	res := FloatType{
		Type: t,
	}
	err := p.parseParameters(func(name, value string) error {
		switch name {
		case "minValue":
			n, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return err
			}
			res.MinValue = n
		case "maxValue":
			n, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return err
			}
			res.MaxValue = n
		default:
			return p.createError("float", fmt.Sprintf("%s is not a supported float parameter", name))
		}
		return nil
	})
	return res, err
}

func (p *Parser) parseTypeDate(t Type) (Pattern, error) {
	res := TimeType{
		Type: t,
	}
	err := p.parseParameters(func(name, value string) error {
		switch name {
		case "minValue":
			n, err := time.Parse("2006-01-02", value)
			if err != nil {
				return err
			}
			res.MinValue = n
		case "maxValue":
			n, err := time.Parse("2006-01-02", value)
			if err != nil {
				return err
			}
			res.MaxValue = n
		default:
			return p.createError("date", fmt.Sprintf("%s is not a supported date parameter", name))
		}
		return nil
	})
	return res, err
}

func (p *Parser) parseParameters(do func(name, value string) error) error {
	p.next()
	for !p.done() && !p.is(EndBrace) {
		if !p.is(Name) {
			return p.createError("parameter", "parameter name is missing")
		}
		name := p.curr.Literal
		p.next()
		if !p.is(Assign) {
			return p.createError("parameter", "missing assignment operator (\"=\") after parameter name")
		}
		p.next()
		if !p.is(Literal) {
			return p.createError("parameter", "parameter value should be given as a quoted string")
		}
		if err := do(name, p.curr.Literal); err != nil {
			return err
		}
		p.next()
	}
	if !p.is(EndBrace) {
		return p.createError("parameter", "missing \"}\" at end of pattern")
	}
	p.next()
	return nil
}

func (p *Parser) parseEnum() (Pattern, error) {
	var pt Enum
	for !p.done() && p.is(Literal) {
		pt.List = append(pt.List, p.curr.Literal)
		p.next()
		if p.is(Alt) {
			p.next()
		}
	}
	return pt, nil
}

func (p *Parser) skipComment() {
	for p.is(Comment) {
		p.next()
	}
}

func (p *Parser) skipEOL() {
	for p.is(EOL) {
		p.next()
	}
}

func (p *Parser) is(kind rune) bool {
	return p.curr.Type == kind
}

func (p *Parser) isKeyword(kw string) bool {
	return p.is(Keyword) && p.curr.Literal == kw
}

func (p *Parser) isType() bool {
	types := []string{"int", "float", "decimal", "bool", "string", "date"}
	for i := range types {
		if p.isKeyword(types[i]) {
			return true
		}
	}
	return false
}

func (p *Parser) done() bool {
	return p.is(EOF)
}

func (p *Parser) next() {
	p.curr = p.peek
	p.peek = p.scan.Scan()
}

func (p *Parser) createError(elem, message string) error {
	return ParseError{
		Position: p.curr.Position,
		Element:  elem,
		Message:  message,
	}
}
