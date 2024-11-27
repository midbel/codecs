package relax

import (
	"fmt"
	"io"
)

type Parser struct {
	scan *Scanner
	curr Token
	peek Token

	spaces map[string]string
	types  map[string]string
	names  map[string]Pattern
}

func Parse(r io.Reader) *Parser {
	p := Parser{
		scan:   Scan(r),
		spaces: make(map[string]string),
		types:  make(map[string]string),
		names:  make(map[string]Pattern),
	}
	p.next()
	p.next()
	return &p
}

func (p *Parser) Parse() (Pattern, error) {
	return p.parse()
}

func (p *Parser) parse() (Pattern, error) {
	if err := p.parseDeclarations(); err != nil {
		return nil, err
	}
	p.skipEOL()
	switch p.curr.Literal {
	case "grammar":
		return p.parseGrammar()
	case "element":
		return p.parseElement()
	case "start":
		return p.parseDefinitions()
	default:
		return nil, fmt.Errorf("unexpected keyword")
	}
}

func (p *Parser) parseDeclarations() error {
	return nil
}

func (p *Parser) parseGrammar() (Pattern, error) {
	p.next()
	if !p.is(BegBrace) {
		return nil, p.unexpected()
	}
	_, err := p.parseDefinitions()
	if err != nil {
		return nil, err
	}
	p.next()
	if !p.is(EndBrace) {
		return nil, p.unexpected()
	}
	p.next()
	return nil, nil
}

func (p *Parser) parseDefinitions() (Pattern, error) {
	if _, err := p.parseStartPattern(); err != nil {
		return nil, err
	}
	for !p.done() {
		p.skipComment()
		if !p.is(Name) {
			return nil, fmt.Errorf("missing name")
		}
		pat, err := p.parseName()
		if err != nil {
			return nil, err
		}
		_ = pat
	}
	return nil, nil
}

func (p *Parser) parseName() (Pattern, error) {
	defer p.skipEOL()
	name := p.curr.Literal
	p.next()
	if !p.is(Assign) {
		return nil, fmt.Errorf("missing assignment after name")
	}
	p.next()
	pattern, err := p.parseElement()
	if err != nil {
		return nil, err
	}
	ref := Reference{
		Ident:   name,
		Pattern: pattern,
	}
	return ref, err
}

func (p *Parser) parseStartPattern() (Pattern, error) {
	defer p.skipEOL()
	if !p.is(Keyword) && p.curr.Literal != "start" {
		return nil, fmt.Errorf("start keyword expected")
	}
	p.next()
	if !p.is(Assign) {
		return nil, fmt.Errorf("missing assignlent after start")
	}
	p.next()
	if p.is(Name) {
		var ref Link
		ref.Ident = p.curr.Literal
		p.next()
		ref.Arity = p.parseArity()
		return ref, nil
	}
	return p.parseElement()
}

func (p *Parser) parsePatternForElement(el *Element) error {
	p.skipComment()
	if p.is(Name) {
		var ref Link
		ref.Ident = p.curr.Literal
		p.next()
		ref.Arity = p.parseArity()
		el.Elements = append(el.Elements, ref)
		return nil
	}
	if !p.is(Keyword) {
		return fmt.Errorf("pattern: keyword expected")
	}
	switch p.curr.Literal {
	case "element":
		ch, err := p.parseElement()
		if err != nil {
			return err
		}
		el.Elements = append(el.Elements, ch)
	case "attribute":
		at, err := p.parseAttribute()
		if err != nil {
			return err
		}
		el.Attributes = append(el.Attributes, at)
	case "text":
		p.next()
	case "empty":
		p.next()
	default:
		return fmt.Errorf("%s: pattern not supported for element", p.curr.Literal)
	}
	return nil
}

func (p *Parser) parseElement() (*Element, error) {
	p.next()
	if !p.is(Name) {
		return nil, fmt.Errorf("element name expected")
	}
	var el Element
	el.Local = p.curr.Literal
	p.next()
	if !p.is(BegBrace) {
		return nil, p.unexpected()
	}
	p.next()
	if p.is(Literal) {
		pattern, err := p.parseEnum()
		if err != nil {
			return nil, err
		}
		el.Value = pattern
	} else if p.is(Keyword) && p.curr.Literal == "text" {
		p.next()
		el.Value = Text{}
	} else if p.is(Keyword) && p.curr.Literal == "empty" {
		p.next()
		el.Value = Empty{}
	} else {
		for !p.done() && !p.is(EndBrace) {
			if err := p.parsePatternForElement(&el); err != nil {
				return nil, err
			}
			switch {
			case p.is(Comma):
				p.next()
				if p.is(EndBrace) {
					return nil, p.unexpected()
				}
			case p.is(EndBrace):
			default:
				return nil, p.unexpected()
			}
		}
	}
	if !p.is(EndBrace) {
		return nil, p.unexpected()
	}
	p.next()
	el.Arity = p.parseArity()
	return &el, nil
}

func (p *Parser) parsePatternForAttribute() (Pattern, error) {
	if p.is(Literal) {
		return p.parseEnum()
	}
	if !p.is(Keyword) {
		return nil, fmt.Errorf("pattern: keyword expected")
	}
	switch p.curr.Literal {
	case "text":
		defer p.next()
		var pattern Text
		return pattern, nil
	default:
		return nil, fmt.Errorf("%s: pattern not supported for attribute", p.curr.Literal)
	}
}

func (p *Parser) parseAttribute() (*Attribute, error) {
	p.next()
	if !p.is(Name) {
		return nil, fmt.Errorf("expected attribute name")
	}
	var at Attribute
	at.Local = p.curr.Literal
	p.next()
	if !p.is(BegBrace) {
		return nil, p.unexpected()
	}
	p.next()
	pattern, err := p.parsePatternForAttribute()
	if err != nil {
		return nil, err
	}
	at.Value = pattern
	if !p.is(EndBrace) {
		return nil, p.unexpected()
	}
	p.next()
	at.Arity = p.parseArity()
	if at.Arity > 0 && at.Arity != ZeroOrOne {
		return nil, fmt.Errorf("unexpected value for attribute")
	}
	return &at, nil
}

func (p *Parser) parseArity() Arity {
	switch {
	case p.is(Mandatory):
		p.next()
		return OneOrMore
	case p.is(Optional):
		p.next()
		return ZeroOrOne
	case p.is(Star):
		p.next()
		return ZeroOrMore
	default:
		return 0
	}
}

func (p *Parser) parseEnum() (Pattern, error) {
	var pt Enum
	for !p.done() && p.is(Literal) {
		pt.List = append(pt.List, p.curr.Literal)
		p.next()
		if p.is(Choice) {
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

func (p *Parser) done() bool {
	return p.is(EOF)
}

func (p *Parser) next() {
	p.curr = p.peek
	p.peek = p.scan.Scan()
}

func (p *Parser) unexpected() error {
	return fmt.Errorf("unexpected token: %s", p.curr)
}
