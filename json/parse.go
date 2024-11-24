package json

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode"
	"unicode/utf8"
)

var (
	errUndefined   = errors.New("undefined")
	errDiscard     = errors.New("discard")
	errType        = errors.New("type")
	errArgument    = errors.New("argument")
	errImplemented = errors.New("not implemented")
)

type Parser struct {
	scan *Scanner
	curr Token
	peek Token

	mode
}

func Parse(r io.Reader) (any, error) {
	p := &Parser{
		scan: Scan(r, stdMode),
		mode: stdMode,
	}
	p.next()
	p.next()
	return p.Parse()
}

func Parse5(r io.Reader) (any, error) {
	p := &Parser{
		scan: Scan(r, json5Mode),
		mode: json5Mode,
	}
	p.next()
	p.next()
	return p.Parse()
}

func (p *Parser) Parse() (any, error) {
	return p.parse()
}

func (p *Parser) parse() (any, error) {
	switch p.curr.Type {
	case BegArr:
		return p.parseArray()
	case BegObj:
		return p.parseObject()
	case String:
		return p.parseString(), nil
	case Number:
		return p.parseNumber(), nil
	case Boolean:
		return p.parseBool(), nil
	case Null:
		return p.parseNull(), nil
	default:
		return nil, fmt.Errorf("syntax error")
	}
}

func (p *Parser) parseKey() (string, error) {
	switch {
	case p.is(String):
	case p.is(Ident) && p.mode.isExtended():
	default:
		return "", fmt.Errorf("syntax error: object key should be string")
	}
	key := p.curr.Literal
	p.next()
	if !p.is(Colon) {
		return "", fmt.Errorf("syntax error: missing ':'")
	}
	p.next()
	return key, nil
}

func (p *Parser) parseObject() (any, error) {
	p.next()
	obj := make(map[string]any)
	for !p.done() && !p.is(EndObj) {
		k, err := p.parseKey()
		if err != nil {
			return nil, err
		}
		a, err := p.parse()
		if err != nil {
			return nil, err
		}

		obj[k] = a
		switch {
		case p.is(Comma):
			p.next()
			if p.is(EndObj) && !p.mode.isExtended() {
				return nil, fmt.Errorf("syntax error: trailing comma not allowed")
			}
		case p.is(EndObj):
		default:
			return nil, fmt.Errorf("syntax error: expected ',' or '}'")
		}
	}
	if !p.is(EndObj) {
		return nil, fmt.Errorf("array: missing '}'")
	}
	p.next()
	return obj, nil
}

func (p *Parser) parseArray() (any, error) {
	p.next()
	var arr []any
	for !p.done() && !p.is(EndArr) {
		a, err := p.parse()
		if err != nil {
			return nil, err
		}
		arr = append(arr, a)
		switch {
		case p.is(Comma):
			p.next()
			if p.is(EndArr) && !p.mode.isExtended() {
				return nil, fmt.Errorf("syntax error: trailing comma not allowed")
			}
		case p.is(EndArr):
		default:
			return nil, fmt.Errorf("syntax error: expected ',' or ']'")
		}
	}
	if !p.is(EndArr) {
		return nil, fmt.Errorf("array: missing ']'")
	}
	p.next()
	return arr, nil
}

func (p *Parser) parseNumber() any {
	defer p.next()
	n, err := strconv.ParseFloat(p.curr.Literal, 64)
	if err != nil {
		n, _ := strconv.ParseInt(p.curr.Literal, 0, 64)
		return float64(n)
	}
	return n
}

func (p *Parser) parseBool() any {
	defer p.next()
	if p.curr.Literal == "true" {
		return true
	}
	return false
}

func (p *Parser) parseString() any {
	defer p.next()
	return p.curr.Literal
}

func (p *Parser) parseNull() any {
	defer p.next()
	return nil
}

func (p *Parser) done() bool {
	return p.is(EOF)
}

func (p *Parser) is(kind rune) bool {
	return p.curr.Type == kind
}

func (p *Parser) next() {
	p.curr = p.peek
	p.peek = p.scan.Scan()
}

type mode int8

const (
	stdMode mode = 1 << iota
	json5Mode
)

func (m mode) isStd() bool {
	return m == stdMode
}

func (m mode) isExtended() bool {
	return m == json5Mode
}

type Scanner struct {
	input io.RuneScanner
	char  rune
	mode

	Position
	old Position

	str bytes.Buffer
}

func Scan(r io.Reader, mode mode) *Scanner {
	scan := Scanner{
		input: bufio.NewReader(r),
		mode:  mode,
	}
	scan.Line = 1
	scan.read()
	return &scan
}

func (s *Scanner) Scan() Token {
	defer s.str.Reset()
	s.skipBlank()

	var tok Token
	if s.done() {
		tok.Type = EOF
		return tok
	}
	switch {
	case s.mode.isExtended() && isComment(s.char, s.peek()):
		s.scanComment(&tok)
	case s.mode.isExtended() && isLetter(s.char):
		s.scanLiteral(&tok)
	case isLower(s.char):
		s.scanIdent(&tok)
	case isQuote(s.char) || (s.mode.isExtended() && isApos(s.char)):
		s.scanString(&tok)
	case isNumber(s.char) || s.char == '-':
		s.scanNumber(&tok)
	case s.mode.isExtended() && (s.char == '+' || s.char == '.'):
		s.scanNumber(&tok)
	case isDelim(s.char):
		s.scanDelimiter(&tok)
	default:
		tok.Type = Invalid
	}
	return tok
}

func (s *Scanner) scanLiteral(tok *Token) {
	for !s.done() && isAlpha(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	switch tok.Literal {
	case "true", "false":
		tok.Type = Boolean
	case "null":
		tok.Type = Null
	default:
		tok.Type = Ident
	}
}

func (s *Scanner) scanComment(tok *Token) {
	s.read()
	s.read()
	s.skipBlank()
	for !s.done() && !isNL(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	tok.Type = Comment
}

func (s *Scanner) scanIdent(tok *Token) {
	for !s.done() && isAlpha(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	switch tok.Literal {
	case "true", "false":
		tok.Type = Boolean
	case "null":
		tok.Type = Null
	default:
		tok.Type = Invalid
	}
}

func (s *Scanner) scanString(tok *Token) {
	quote := s.char
	s.read()
	for !s.done() && s.char != quote {
		if s.char == '\\' {
			s.read()
			if isNL(s.char) {
				s.write()
				s.read()
				continue
			}
			if ok := s.scanEscape(quote); !ok {
				tok.Type = Invalid
				return
			}
		}
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	tok.Type = String
	if s.char != quote {
		tok.Type = Invalid
	} else {
		s.read()
	}
}

func (s *Scanner) scanEscape(quote rune) bool {
	switch s.char {
	case quote:
		s.char = quote
	case '\\':
		s.char = '\\'
	case '/':
		s.char = '/'
	case 'b':
		s.char = '\b'
	case 'f':
		s.char = '\f'
	case 'n':
		s.char = '\n'
	case 'r':
		s.char = '\r'
	case 't':
		s.char = '\t'
	case 'u':
		s.read()
		buf := make([]rune, 4)
		for i := 1; i <= 4; i++ {
			if !isHex(s.char) {
				return false
			}
			buf[i-1] = s.char
			if i < 4 {
				s.read()
			}
		}
		char, _ := strconv.ParseInt(string(buf), 16, 32)
		s.char = rune(char)
	default:
		return false
	}
	return true
}

func (s *Scanner) scanHexa(tok *Token) {
	s.read()
	s.read()
	s.writeRune('0')
	s.writeRune('x')
	for !s.done() && isHex(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
}

func (s *Scanner) scanNumber(tok *Token) {
	tok.Type = Number
	if s.mode.isExtended() && s.char == '0' && s.peek() == 'x' {
		s.scanHexa(tok)
		return
	}
	if s.mode.isExtended() && s.char == '.' {
		s.writeRune('0')
		s.writeRune('.')
		s.read()
	}
	if s.char == '-' || s.char == '+' {
		s.write()
		s.read()
	}
	for !s.done() && isNumber(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	if s.char == '.' {
		s.write()
		s.read()
		if !isNumber(s.char) {
			if !s.mode.isExtended() {
				tok.Type = Invalid
			}
			return
		}
		for !s.done() && isNumber(s.char) {
			s.write()
			s.read()
		}
		tok.Literal = s.str.String()
	}
	if s.char == 'e' || s.char == 'E' {
		s.write()
		s.read()
		if s.char == '-' || s.char == '+' {
			s.write()
			s.read()
		}
		if !isNumber(s.char) {
			tok.Type = Invalid
			return
		}
		for !s.done() && isNumber(s.char) {
			s.write()
			s.read()
		}
		tok.Literal = s.str.String()
	}
}

func (s *Scanner) scanDelimiter(tok *Token) {
	switch s.char {
	case '[':
		tok.Type = BegArr
	case ']':
		tok.Type = EndArr
	case '{':
		tok.Type = BegObj
	case '}':
		tok.Type = EndObj
	case ',':
		tok.Type = Comma
	case ':':
		tok.Type = Colon
	default:
		tok.Type = Invalid
	}
	if tok.Type != Invalid {
		s.read()
	}
}

func (s *Scanner) writeRune(c rune) {
	s.str.WriteRune(c)
}

func (s *Scanner) write() {
	s.writeRune(s.char)
}

func (s *Scanner) read() {
	s.old = s.Position
	if s.char == '\n' {
		s.Line++
		s.Column = 0
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

func isComment(c, k rune) bool {
	return c == '/' && c == k
}

func isHex(c rune) bool {
	return isNumber(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isNumber(c rune) bool {
	return c >= '0' && c <= '9'
}

func isLower(c rune) bool {
	return c >= 'a' && c <= 'z'
}

func isUpper(c rune) bool {
	return c >= 'A' && c <= 'Z'
}

func isLetter(c rune) bool {
	return isLower(c) || isUpper(c)
}

func isAlpha(c rune) bool {
	return isLetter(c) || isNumber(c) || c == '_'
}

func isApos(c rune) bool {
	return c == '\''
}

func isQuote(c rune) bool {
	return c == '"'
}

func isBackQuote(c rune) bool {
	return c == '`'
}

func isDelim(c rune) bool {
	return c == '{' || c == '}' || c == '[' || c == ']' || c == ',' || c == ':'
}

func isNL(c rune) bool {
	return c == '\n' || c == '\r'
}

func isOperator(c rune) bool {
	return c == '!' || c == '=' || c == '<' || c == '>' ||
		c == '&' || c == '*' || c == '/' || c == '%' || c == '-' ||
		c == '+' || c == '.' || c == '?' || c == ':'
}

func isTransform(c rune) bool {
	return c == '|'
}

func isDollar(c rune) bool {
	return c == '$'
}
