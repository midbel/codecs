package xml

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/midbel/codecs/environ"
)

const MaxDepth = 512

const (
	SupportedVersion  = "1.0"
	SupportedEncoding = "UTF-8"
)

const AttrXmlNS = "xmlns"

type ParseError struct {
	Position
	Element string
	Message string
}

func createParseError(elem, msg string, pos Position) error {
	return ParseError{
		Position: pos,
		Element:  elem,
		Message:  msg,
	}
}

func (p ParseError) Error() string {
	return fmt.Sprintf("%d:%d: %s: %s", p.Line, p.Column, p.Element, p.Message)
}

type PiFunc func(string, []Attribute) (Node, error)

type Parser struct {
	scan *Scanner
	curr Token
	peek Token

	depth int

	TrimSpace  bool
	KeepEmpty  bool
	OmitProlog bool
	StrictNS   bool
	MaxDepth   int

	namespaces environ.Environ[string]

	piFuncs map[string]PiFunc
}

func NewParser(r io.Reader) *Parser {
	p := Parser{
		scan:       Scan(r),
		TrimSpace:  true,
		MaxDepth:   MaxDepth,
		piFuncs:    make(map[string]PiFunc),
		namespaces: environ.Empty[string](),
	}
	p.next()
	p.next()
	return &p
}

func ParseFile(file string) (*Document, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	return ParseReader(r)
}

func ParseString(xml string) (*Document, error) {
	return ParseReader(strings.NewReader(xml))
}

func ParseReader(r io.Reader) (*Document, error) {
	p := NewParser(r)
	return p.Parse()
}

func (p *Parser) RegisterPI(name string, fn PiFunc) {
	p.piFuncs[name] = fn
}

func (p *Parser) UnregisterPI(name string) {
	delete(p.piFuncs, name)
}

func (p *Parser) Parse() (*Document, error) {
	if _, err := p.parseProlog(); err != nil {
		return nil, err
	}
	for p.is(Literal) {
		p.next()
	}
	var (
		doc Document
		err error
	)
	doc.Version = SupportedVersion
	doc.Encoding = SupportedEncoding
	for !p.done() {
		node, err := p.parseNode()
		if err != nil {
			return nil, err
		}
		if node == nil {
			continue
		}
		switch node.Type() {
		case TypeComment, TypeElement:
		case TypeText:
			continue
		default:
			return nil, p.createError("document", "invalid node type")
		}
		doc.attach(node)
		if node.Type() == TypeElement {
			break
		}
	}
	if doc.Root() == nil {
		return nil, p.createError("document", "missing root element")
	}
	return &doc, err
}

func (p *Parser) parseProlog() (Node, error) {
	if !p.is(ProcInstTag) {
		if !p.OmitProlog {
			return nil, p.createError("document", "xml prolog missing")
		}
		return nil, nil
	}
	node, err := p.parsePI()
	if err != nil {
		return nil, err
	}
	pi, ok := node.(*Instruction)
	if !ok {
		return nil, p.createError("document", "expected xml prolog")
	}
	ok = slices.ContainsFunc(pi.Attrs, func(a Attribute) bool {
		return a.LocalName() == "version" && a.Value() == SupportedVersion
	})
	if !ok {
		return nil, p.createError("document", "xml version not supported")
	}
	ix := slices.IndexFunc(pi.Attrs, func(a Attribute) bool {
		return a.LocalName() == "encoding"
	})
	if ix >= 0 && strings.ToUpper(pi.Attrs[ix].Value()) != SupportedEncoding {
		return nil, p.createError("document", "xml encoding not supported")
	}
	return pi, nil
}

func (p *Parser) parseNode() (Node, error) {
	p.enter()
	defer p.leave()
	if p.depth >= p.MaxDepth {
		return nil, p.createError("document", "maximum depth reached")
	}
	var (
		node Node
		err  error
	)
	switch p.curr.Type {
	case OpenTag:
		node, err = p.parseElement()
	case CommentTag:
		node, err = p.parseComment()
	case ProcInstTag:
		node, err = p.parsePI()
	case Cdata:
		node, _ = p.parseCharData()
	case Literal:
		node, _ = p.parseLiteral()
	default:
		return nil, p.createError("document", "unsupported element type")
	}
	if err != nil {
		return nil, err
	}
	return node, nil
}

func (p *Parser) parseElement() (Node, error) {
	p.namespaces = environ.Enclosed[string](p.namespaces)
	defer func() {
		u, ok := p.namespaces.(interface {
			Unwrap() environ.Environ[string]
		})
		if !ok {
			return
		}
		p.namespaces = u.Unwrap()
	}()
	p.next()
	var (
		elem Element
		err  error
	)
	if p.is(Namespace) {
		elem.Space = p.getCurrentLiteral()
		p.next()
	}
	if !p.is(Name) {
		return nil, p.createError("element", "name is missing")
	}
	elem.Name = p.getCurrentLiteral()
	p.next()

	elem.Attrs, err = p.parseAttributes(&elem, func() bool {
		return p.is(EndTag) || p.is(EmptyElemTag)
	})
	if err != nil {
		return nil, err
	}

	if elem.Uri, err = p.isDefined(elem.QName); err != nil {
		return nil, err
	}

	switch p.curr.Type {
	case EmptyElemTag:
		p.next()
		return &elem, nil
	case EndTag:
		p.next()
		var pos int
		for !p.done() && !p.is(CloseTag) {
			child, err := p.parseNode()
			if err != nil {
				return nil, err
			}
			if child != nil {
				child.setPosition(pos)
				child.setParent(&elem)
				elem.Nodes = append(elem.Nodes, child)
				pos++
			}
		}
		if !p.is(CloseTag) {
			return nil, p.createError("element", "closing element is missing")
		}
		p.next()
		return &elem, p.parseCloseElement(elem)
	default:
		return nil, p.createError("element", "end of element expected")
	}
}

func (p *Parser) parseCloseElement(elem Element) error {
	if elem.Space != "" && !p.is(Namespace) {
		return p.createError("element", "closing element without namespace")
	}
	if p.is(Namespace) {
		if _, err := p.isDefined(elem.QName); err != nil {
			return err
		}
		if elem.Space != p.getCurrentLiteral() {
			return p.createError("element", "namespace mismatched with opening element")
		}
		p.next()
	}
	if !p.is(Name) {
		return p.createError("element", "name is missing")
	}
	if p.getCurrentLiteral() != elem.Name {
		return p.createError("element", "name mismatched with opening element")
	}
	p.next()
	if !p.is(EndTag) {
		return p.createError("element", "end of element expected")
	}
	p.next()
	return nil
}

func (p *Parser) parsePI() (Node, error) {
	p.next()
	if !p.is(Name) {
		return nil, p.createError("processing instruction", "name is missing")
	}
	var elem Instruction
	elem.Name = p.getCurrentLiteral()
	p.next()
	var err error
	elem.Attrs, err = p.parseAttributes(&elem, func() bool {
		return p.is(ProcInstTag)
	})
	if err != nil {
		return nil, err
	}
	if !p.is(ProcInstTag) {
		return nil, p.createError("processing instruction", "end of element expected")
	}
	p.next()
	fn, ok := p.piFuncs[elem.Name]
	if ok {
		return fn(elem.Name, elem.Attrs)
	}
	return &elem, nil
}

func (p *Parser) parseAttributes(parent Node, done func() bool) ([]Attribute, error) {
	var attrs []Attribute
	for i := 0; !p.done() && !done(); i++ {
		attr, err := p.parseAttr()
		if err != nil {
			return nil, err
		}
		ok := slices.ContainsFunc(attrs, func(a Attribute) bool {
			return attr.QualifiedName() == a.QualifiedName()
		})
		if ok {
			return nil, p.createError("attribute", "attribute is already defined")
		}
		attr.setParent(parent)
		attr.setPosition(i)
		attrs = append(attrs, attr)
	}
	return attrs, nil
}

func (p *Parser) parseAttr() (Attribute, error) {
	var (
		attr Attribute
		err  error
	)
	if p.is(Namespace) {
		attr.Space = p.getCurrentLiteral()
		p.next()
	}
	if !p.is(Attr) {
		return attr, p.createError("attribute", "name is expected")
	}
	attr.Name = p.getCurrentLiteral()
	p.next()
	if !p.is(Literal) {
		return attr, p.createError("attribute", "value is missing")
	}
	attr.Datum = p.getCurrentLiteral()
	p.next()
	if attr.Name == AttrXmlNS {
		p.defineNS("", attr.Datum)
	} else if attr.Space == AttrXmlNS {
		p.defineNS(attr.Name, attr.Datum)
	}
	if attr.Uri, err = p.isDefined(attr.QName); err != nil {
		return attr, err
	}
	return attr, nil
}

func (p *Parser) parseComment() (Node, error) {
	defer p.next()
	node := Comment{
		Content: p.getCurrentLiteral(),
	}
	return &node, nil
}

func (p *Parser) parseCharData() (Node, error) {
	defer p.next()
	char := CharData{
		Content: p.getCurrentLiteral(),
	}
	return &char, nil
}

func (p *Parser) parseLiteral() (Node, error) {
	text := Text{
		Content: p.getCurrentLiteral(),
	}
	if p.TrimSpace {
		text.Content = strings.TrimSpace(text.Content)
	}
	p.next()
	if !p.KeepEmpty && text.Content == "" {
		return nil, nil
	}
	return &text, nil
}

func (p *Parser) isDefined(qn QName) (string, error) {
	if qn.Name == AttrXmlNS {
		return "", nil
	}
	uri, err := p.namespaces.Resolve(qn.Space)
	if err != nil && p.StrictNS {
		return "", fmt.Errorf("%s: namespace is not defined", qn.Space)
	}
	return uri, nil
}

func (p *Parser) defineNS(ident, uri string) {
	p.namespaces.Define(ident, uri)
}

func (p *Parser) getCurrentLiteral() string {
	return p.curr.Literal
}

func (p *Parser) createError(elem, msg string) error {
	return createParseError(elem, msg, p.curr.Position)
}

func (p *Parser) is(kind rune) bool {
	return p.curr.Type == kind
}

func (p *Parser) done() bool {
	return p.is(EOF)
}

func (p *Parser) enter() {
	p.depth++
}

func (p *Parser) leave() {
	p.depth--
}

func (p *Parser) next() {
	p.curr = p.peek
	p.peek = p.scan.Scan()
}

const (
	EOF rune = -(1 + iota)
	Name
	Namespace // name:
	Attr      // name=
	Literal
	Digit
	Cdata
	CommentTag   // <!--
	OpenTag      // <
	EndTag       // >
	CloseTag     // </
	EmptyElemTag // />
	ProcInstTag  // <?, ?>
	Invalid
)

type Position struct {
	Line   int
	Column int
}

type Token struct {
	Literal string
	Type    rune
	Position
}

func (t Token) String() string {
	switch t.Type {
	case EOF:
		return "<eof>"
	case Digit:
		return fmt.Sprintf("number(%s)", t.Literal)
	case CommentTag:
		return fmt.Sprintf("comment(%s)", t.Literal)
	case Name:
		return fmt.Sprintf("name(%s)", t.Literal)
	case Namespace:
		return fmt.Sprintf("namespace(%s)", t.Literal)
	case Attr:
		return fmt.Sprintf("attr(%s)", t.Literal)
	case Cdata:
		return fmt.Sprintf("chardata(%s)", t.Literal)
	case Literal:
		return fmt.Sprintf("literal(%s)", t.Literal)
	case OpenTag:
		return "<open-elem-tag>"
	case EndTag:
		return "<end-elem-tag>"
	case CloseTag:
		return "<close-elem-tag>"
	case EmptyElemTag:
		return "<empty-elem-tag>"
	case ProcInstTag:
		return "<processing-instruction>"
	case Invalid:
		return "<invalid>"
	default:
		return "<unknown>"
	}
}

const (
	langle     = '<'
	rangle     = '>'
	lsquare    = '['
	rsquare    = ']'
	lparen     = '('
	rparen     = ')'
	colon      = ':'
	quote      = '"'
	apos       = '\''
	slash      = '/'
	question   = '?'
	bang       = '!'
	equal      = '='
	ampersand  = '&'
	semicolon  = ';'
	dash       = '-'
	underscore = '_'
	dot        = '.'
	arobase    = '@'
	comma      = ','
	plus       = '+'
	star       = '*'
	percent    = '%'
	pipe       = '|'
	dollar     = '$'
)

type state int8

const (
	literalState state = 1 << iota
)

type Scanner struct {
	input io.RuneScanner
	char  rune
	str   bytes.Buffer

	Position
	old Position

	state
}

func Scan(r io.Reader) *Scanner {
	var (
		rs    = bufio.NewReader(r)
		pk, _ = rs.Peek(3)
	)
	if bytes.Equal(pk, []byte{0xEF, 0xBB, 0xBF}) {
		rs.Discard(3)
	}

	scan := &Scanner{
		input: rs,
	}
	scan.Position.Line = 1
	scan.read()
	return scan
}

func (s *Scanner) Scan() Token {
	var tok Token
	tok.Position = s.Position
	if s.done() {
		tok.Type = EOF
		return tok
	}

	if s.state == literalState {
		s.scanLiteral(&tok)
		return tok
	}
	s.str.Reset()
	switch {
	case s.char == langle:
		s.scanOpeningTag(&tok)
	case s.char == rangle:
		s.scanEndTag(&tok)
	case s.char == slash || s.char == question:
		s.scanClosingTag(&tok)
	case s.char == quote:
		s.scanValue(&tok)
	case unicode.IsLetter(s.char):
		s.scanName(&tok)
	default:
		s.scanLiteral(&tok)
	}
	return tok
}

func (s *Scanner) scanOpeningTag(tok *Token) {
	s.read()
	tok.Type = OpenTag
	switch s.char {
	case bang:
		s.read()
		if s.char == lsquare {
			s.scanCharData(tok)
			return
		}
		if s.char == dash {
			s.scanComment(tok)
			return
		}
		tok.Type = Invalid
	case question:
		tok.Type = ProcInstTag
	case slash:
		tok.Type = CloseTag
	default:
	}
	if tok.Type == ProcInstTag || tok.Type == CloseTag {
		s.read()
	}
}

func (s *Scanner) scanComment(tok *Token) {
	s.read()
	if s.char != dash {
		tok.Type = Invalid
		return
	}
	s.read()
	var done bool
	for !s.done() {
		if s.char == dash && s.peek() == s.char {
			s.read()
			s.read()
			if done = s.char == rangle; done {
				s.read()
				break
			}
			s.str.WriteRune(dash)
			s.str.WriteRune(dash)
			continue
		}
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	tok.Type = CommentTag
	if !done {
		tok.Type = Invalid
	}
}

func (s *Scanner) scanCharData(tok *Token) {
	s.read()
	for !s.done() && s.char != lsquare {
		s.write()
		s.read()
	}
	s.read()
	if s.str.String() != "CDATA" {
		tok.Type = Invalid
		return
	}
	s.str.Reset()
	var done bool
	for !s.done() {
		if s.char == rsquare && s.peek() == s.char {
			s.read()
			s.read()
			if done = s.char == rangle; done {
				s.read()
				break
			}
			s.str.WriteRune(rsquare)
			s.str.WriteRune(rsquare)
			continue
		}
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	tok.Type = Cdata
	if !done {
		tok.Type = Invalid
	}
}

func (s *Scanner) scanEndTag(tok *Token) {
	tok.Type = EndTag
	s.state = literalState
	s.read()
}

func (s *Scanner) scanClosingTag(tok *Token) {
	tok.Type = Invalid
	if s.char == question {
		tok.Type = ProcInstTag
	} else if s.char == slash {
		tok.Type = EmptyElemTag
	}
	s.read()
	if s.char != rangle {
		tok.Type = Invalid
	} else {
		s.read()
	}
}

func (s *Scanner) scanValue(tok *Token) {
	s.read()
	for !s.done() && s.char != quote {
		s.write()
		s.read()
		if s.char == ampersand {
			str := s.scanEntity()
			if str == "" {
				break
			}
			s.str.WriteString(str)
			continue
		}
	}
	tok.Type = Literal
	tok.Literal = s.str.String()
	if s.char != quote {
		tok.Type = Invalid
	}
	s.read()
	s.skipBlank()

}

func (s *Scanner) scanEntity() string {
	s.read()
	var str bytes.Buffer
	str.WriteRune('&')
	for !s.done() && s.char != semicolon {
		str.WriteRune(s.char)
		s.read()
	}
	if s.char != semicolon {
		return ""
	}
	str.WriteRune(semicolon)
	s.read()
	return html.UnescapeString(str.String())
}

func (s *Scanner) scanLiteral(tok *Token) {
	for !s.done() && s.char != langle {
		s.write()
		s.read()
		if s.char == ampersand {
			str := s.scanEntity()
			if str == "" {
				break
			}
			s.str.WriteString(str)
		}
	}
	tok.Type = Literal
	tok.Literal = s.str.String()
	if s.char == langle {
		s.state = 0
	}
}

func (s *Scanner) scanName(tok *Token) {
	accept := func() bool {
		return unicode.IsLetter(s.char) || unicode.IsDigit(s.char) ||
			s.char == dash || s.char == underscore || s.char == dot
	}
	for !s.done() && accept() {
		s.write()
		s.read()
	}
	tok.Type = Name
	tok.Literal = s.str.String()
	if s.char == equal {
		tok.Type = Attr
		s.read()
	} else if s.char == colon {
		tok.Type = Namespace
		s.read()
	} else {
		s.skipBlank()
	}
}

func (s *Scanner) write() {
	s.str.WriteRune(s.char)
}

func (s *Scanner) read() {
	s.old = s.Position
	if s.char == '\n' {
		s.Column = 0
		s.Line++
	}
	s.Column++
	char, _, err := s.input.ReadRune()
	if errors.Is(err, io.EOF) {
		char = utf8.RuneError
	}
	s.char = char
}

func (s *Scanner) peek() rune {
	defer s.input.UnreadRune()
	r, _, _ := s.input.ReadRune()
	return r
}

func (s *Scanner) done() bool {
	return s.char == utf8.RuneError
}

func (s *Scanner) skipBlank() {
	for !s.done() && unicode.IsSpace(s.char) {
		s.read()
	}
}
