package relax

import "io"

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

func (p *Parser) Parse() error {
	return nil
}

func (p *Parser) parse() error {
	return nil
}

func (p *Parser) parseDatatypes() error {
	return nil
}

func (p *Parser) parseNamespaces() error {
	return nil
}

func (p *Parser) parseGrammar() error {
	return nil
}

func (p *Parser) parseElement() (*Element, error) {
	return nil, nil
}

func (p *Parser) parseAttribute() (*Attribute, error) {
	return nil, nil
}

func (p *Parser) done() bool {
	return p.curr.Type == EOF
}

func (p *Parser) next() {
	p.curr = p.peek
	p.peek = p.scan.Scan()
}
