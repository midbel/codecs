package relax

type Parser struct {
	scan *Scanner
	curr Token
	peek Token

	namespaces map[string]string
	datatypes  map[string]string
}

func Parse(r io.Reader) *Parser {
	p := Parser{
		scan: Scan(r),
	}
	p.next()
	p.next()
	return &p
}

func (p *Parser) Parse() error {
	return nil
}

func (p *Parser) done() bool {
	return p.curr.Type == EOF
}

func (p *Parser) next() {
	p.curr = p.peek
	p.peek = p.scan.Scan()
}
