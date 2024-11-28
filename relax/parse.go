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
	p.next()
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
	var (
		grm Grammar
		err error
	)
	if grm.Start, err = p.parseStartPattern(); err != nil {
		return nil, err
	}
	grm.List = make(map[string]Pattern)
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
		if grm.List[name], err = p.parseElement(); err != nil {
			return nil, err
		}
		p.skipEOL()
	}
	return grm, nil
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

func (p *Parser) parsePatternForElement() (Pattern, error) {
	p.skipComment()
	if p.is(Name) {
		var ref Link
		ref.Ident = p.curr.Literal
		p.next()
		ref.Arity = p.parseArity()
		return ref, nil
	}
	if !p.is(Keyword) {
		return nil, fmt.Errorf("pattern: keyword expected")
	}
	switch p.curr.Literal {
	case "element":
		return p.parseElement()
	case "attribute":
		return p.parseAttribute()
	case "text":
		p.next()
		var pat Text
		return pat, nil
	case "empty":
		p.next()
		var pat Empty
		return pat, nil
	default:
		return nil, fmt.Errorf("%s: pattern not supported for element", p.curr.Literal)
	}
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
	if p.is(Literal) {
		el.Value, err = p.parseEnum()
		if err != nil {
			return nil, err
		}
	} else if p.is(Keyword) && p.curr.Literal == "text" {
		p.next()
		el.Value = Text{}
	} else if p.is(Keyword) && p.curr.Literal == "empty" {
		p.next()
		el.Value = Empty{}
	} else {
		for !p.done() && !p.is(EndBrace) {
			pat, err := p.parsePatternForElement()
			if err != nil {
				return nil, err
			}
			el.Patterns = append(el.Patterns, pat)
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
	return el, nil
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
	at.Value, err = p.parsePatternForAttribute()
	if err != nil {
		return nil, err
	}
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
