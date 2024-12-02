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
	return p.parse()
}

func (p *Parser) parse() (Pattern, error) {
	if err := p.parseDeclarations(); err != nil {
		return nil, err
	}
	p.skipEOL()
	switch p.curr.Literal {
	case "element":
		return p.parseElement()
	case "start":
		return p.parseDefinitions()
	default:
		return nil, fmt.Errorf("unexpected keyword")
	}
}

func (p *Parser) parseDeclarations() error {
	for !p.done() {
		p.skipEOL()
		p.skipComment()

		if !p.is(Keyword) || (p.curr.Literal == "start" || p.curr.Literal == "element") {
			break
		}
		if err := p.parseNamespace(); err != nil {
			return err
		}
	}
	return nil
}

func (p *Parser) parseNamespace() error {
	switch p.curr.Literal {
	case "default":
		p.next()
		return p.parseNamespace()
	case "namespace":
		p.next()
	default:
		return fmt.Errorf("unexpected keyword")
	}
	name := p.curr.Literal
	p.next()
	if !p.is(Assign) {
		return p.unexpected()
	}
	p.next()
	if !p.is(Literal) {
		return p.unexpected()
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
	patterns := make(map[string]Pattern)
	for !p.done() {
		p.skipComment()
		if !p.is(Name) {
			return nil, fmt.Errorf("missing name")
		}
		name := p.curr.Literal
		p.next()
		if !p.is(Assign) {
			return nil, fmt.Errorf("missing assignment after name")
		}
		p.next()
		if patterns[name], err = p.parseElement(); err != nil {
			return nil, err
		}
		p.skipEOL()
	}
	return reassemble(start, patterns)
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

func (p *Parser) parseList() (Pattern, error) {
	var grp Group
	for p.is(Keyword) && p.curr.Literal == "element" {
		el, err := p.parseElement()
		if err != nil {
			return nil, err
		}
		grp.List = append(grp.List, el)
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

func (p *Parser) parseChoice() (Pattern, error) {
	p.next()
	var ch Choice
	for !p.done() && !p.is(EndParen) {
		var (
			el  Pattern
			err error
		)
		switch {
		case p.is(Keyword) && p.curr.Literal == "element":
			el, err = p.parseList()
		case p.is(BegParen):
			el, err = p.parseChoice()
		default:
			return nil, p.unexpected()
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
			return nil, p.unexpected()
		}
	}
	if !p.is(EndParen) {
		return nil, p.unexpected()
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
		return nil, p.unexpected()
	}
	p.next()
	p.skipEOL()
	p.skipComment()
	for p.is(Keyword) && p.curr.Literal == "attribute" {
		at, err := p.parseAttribute()
		if err != nil {
			return nil, err
		}
		el.Attributes = append(el.Attributes, at)
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
	p.skipEOL()
	p.skipComment()
	for p.is(Name) || p.is(BegParen) || (p.is(Keyword) && p.curr.Literal == "element") {
		if p.is(Name) {
			var ref Link
			ref.Ident = p.curr.Literal
			p.next()
			ref.Arity = p.parseArity()
			el.Elements = append(el.Elements, ref)
		} else if p.is(BegParen) {
			elem, err := p.parseChoice()
			if err != nil {
				return nil, err
			}
			el.Elements = append(el.Elements, elem)
		} else {
			elem, err := p.parseElement()
			if err != nil {
				return nil, err
			}
			el.Elements = append(el.Elements, elem)
		}
		switch {
		case p.is(Comma):
			p.next()
			if p.is(EndBrace) {
				return nil, p.unexpected()
			}
		case p.is(EndBrace):
		default:
			fmt.Println(p.curr, p.peek)
			return nil, p.unexpected()
		}
	}
	p.skipEOL()
	p.skipComment()
	switch {
	case p.is(Keyword) && p.curr.Literal == "text":
		p.next()
		el.Value = Text{}
		if len(el.Elements) != 0 {
			return nil, fmt.Errorf("invalid text pattern! no empty elements")
		}
	case p.is(Keyword) && p.curr.Literal == "empty":
		el.Value = Empty{}
		if len(el.Elements) != 0 {
			return nil, fmt.Errorf("invalid empty pattern! no empty elements")
		}
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
		return nil, fmt.Errorf("unexpected pattern type")
	}
	if !p.is(EndBrace) {
		return nil, p.unexpected()
	}
	p.next()
	el.Arity = p.parseArity()
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
		return nil, p.unexpected()
	}
	p.next()
	if !p.is(Keyword) && p.curr.Literal != "text" {
		return nil, fmt.Errorf("unexpected pattern type for attribute")
	}
	p.next()
	at.Value = Text{}
	if !p.is(EndBrace) {
		return nil, p.unexpected()
	}
	p.next()
	at.Arity = p.parseArity()
	if at.Arity > 0 && at.Arity != ZeroOrOne {
		return nil, fmt.Errorf("unexpected value for attribute")
	}
	return at, nil
}

func (p *Parser) parseName() (QName, error) {
	var q QName
	if !p.is(Name) {
		return q, fmt.Errorf("name expected")
	}
	q.Local = p.curr.Literal
	p.next()
	if p.is(Colon) {
		p.next()
		if !p.is(Name) {
			return q, fmt.Errorf("local name expected")
		}
		defer p.next()
		q.Space = q.Local
		q.Local = p.curr.Literal
	}
	return q, nil
}

func (p *Parser) parseArity() Arity {
	var arity Arity
	switch {
	case p.is(Mandatory):
		arity = OneOrMore
	case p.is(Optional):
		arity = ZeroOrOne
	case p.is(Star):
		arity = ZeroOrMore
	default:
	}
	if arity != 0 {
		p.next()
	}
	return arity
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
