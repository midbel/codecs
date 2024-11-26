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

func (p *Parser) Parse() (*Element, error) {
	return p.parse()
}

func (p *Parser) parse() (*Element, error) {
	p.skipComment()
	if !p.is(Keyword) {
		return nil, fmt.Errorf("keyword expected")
	}
	switch p.curr.Literal {
	case "element":
		return p.parseElement()
	default:
		return nil, fmt.Errorf("pattern not yet supported")
	}
}

func (p *Parser) parsePatternForElement(el *Element) error {
	p.skipComment()
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
		_, err := p.parseEnum()
		if err != nil {
			return nil, err
		}
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

func (p *Parser) parsePatternForAttribute() error {
	if !p.is(Keyword) {
		return fmt.Errorf("pattern: keyword expected")
	}
	switch p.curr.Literal {
	case "text":
		p.next()
	default:
		return fmt.Errorf("%s: pattern not supported for attribute", p.curr.Literal)
	}
	return nil
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
	if p.is(Literal) {
		_, err := p.parseEnum()
		if err != nil {
			return nil, err
		}
	} else {
		err := p.parsePatternForAttribute()
		if err != nil {
			return nil, err
		}
	}
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

func (p *Parser) parseEnum() ([]string, error) {
	var list []string
	for !p.done() && p.is(Literal) {
		list = append(list, p.curr.Literal)
		p.next()
		if p.is(Choice) {
			p.next()
		}
	}
	return list, nil
}

func (p *Parser) skipComment() {
	for p.is(Comment) {
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
