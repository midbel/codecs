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

	"github.com/midbel/codecs/internal/jsonkit"
)

var errSyntax = errors.New("syntax error")

func Decode(r io.Reader) (any, error) {
	p := createParser(r, stdMode)
	return p.Parse()
}

func Decode5(r io.Reader) (any, error) {
	p := createParser(r, json5Mode)
	return p.Parse()
}

type Parser struct {
	scan *Scanner
	curr jsonkit.Token
	peek jsonkit.Token

	mode
}

func createParser(r io.Reader, jm mode) *Parser {
	p := &Parser{
		scan: Scan(r, jm),
		mode: stdMode,
	}
	p.next()
	p.next()
	return p
}

func (p *Parser) Parse() (any, error) {
	return p.parse()
}

func (p *Parser) parse() (any, error) {
	switch p.curr.Type {
	case jsonkit.BegArr:
		return p.parseArray()
	case jsonkit.BegObj:
		return p.parseObject()
	case jsonkit.String:
		return p.parseString(), nil
	case jsonkit.Number:
		return p.parseNumber(), nil
	case jsonkit.Boolean:
		return p.parseBool(), nil
	case jsonkit.Null:
		return p.parseNull(), nil
	default:
		return nil, fmt.Errorf("syntax error")
	}
}

func (p *Parser) parseKey() (string, error) {
	switch {
	case p.is(jsonkit.String):
	case p.is(jsonkit.Ident) && p.mode.isExtended():
	default:
		return "", p.syntaxError("object key should be string")
	}
	key := p.currentLiteral()
	p.next()
	if !p.is(jsonkit.Colon) {
		return "", p.syntaxError("missing colon after key")
	}
	p.next()
	return key, nil
}

func (p *Parser) parseObject() (any, error) {
	p.next()
	obj := make(map[string]any)
	for !p.done() && !p.is(jsonkit.EndObj) {
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
		case p.is(jsonkit.Comma):
			p.next()
			if p.is(jsonkit.EndObj) && !p.mode.isExtended() {
				return nil, p.syntaxError("trailing comma not allowed")
			}
		case p.is(jsonkit.EndObj):
		default:
			return nil, p.syntaxError("expected ',' or '}'")
		}
	}
	if !p.is(jsonkit.EndObj) {
		return nil, p.syntaxError("missing '}' at end of object")
	}
	p.next()
	return obj, nil
}

func (p *Parser) parseArray() (any, error) {
	p.next()
	var arr []any
	for !p.done() && !p.is(jsonkit.EndArr) {
		a, err := p.parse()
		if err != nil {
			return nil, err
		}
		arr = append(arr, a)
		switch {
		case p.is(jsonkit.Comma):
			p.next()
			if p.is(jsonkit.EndArr) && !p.mode.isExtended() {
				return nil, p.syntaxError("trailing comma not allowed")
			}
		case p.is(jsonkit.EndArr):
		default:
			return nil, p.syntaxError("expected ',' or ']'")
		}
	}
	if !p.is(jsonkit.EndArr) {
		return nil, p.syntaxError("missing ']' at end of array")
	}
	p.next()
	return arr, nil
}

func (p *Parser) parseNumber() any {
	defer p.next()
	n, err := strconv.ParseFloat(p.currentLiteral(), 64)
	if err != nil {
		n, _ := strconv.ParseInt(p.currentLiteral(), 0, 64)
		return float64(n)
	}
	return n
}

func (p *Parser) parseBool() any {
	defer p.next()
	if p.currentLiteral() == "true" {
		return true
	}
	return false
}

func (p *Parser) parseString() any {
	defer p.next()
	return p.currentLiteral()
}

func (p *Parser) parseNull() any {
	defer p.next()
	return nil
}

func (p *Parser) done() bool {
	return p.is(jsonkit.EOF)
}

func (p *Parser) is(kind rune) bool {
	return p.curr.Type == kind
}

func (p *Parser) next() {
	p.curr = p.peek
	p.peek = p.scan.Scan()
}

func (p *Parser) currentLiteral() string {
	return p.curr.Literal
}

func (p *Parser) syntaxError(msg string) error {
	return fmt.Errorf("%w: %s", errSyntax, msg)
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

	jsonkit.Position
	old jsonkit.Position

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

func (s *Scanner) Scan() jsonkit.Token {
	defer s.str.Reset()
	s.skipBlank()

	var tok jsonkit.Token
	if s.done() {
		tok.Type = jsonkit.EOF
		return tok
	}
	switch {
	case s.mode.isExtended() && jsonkit.IsComment(s.char, s.peek()):
		s.scanComment(&tok)
	case s.mode.isExtended() && jsonkit.IsLetter(s.char):
		s.scanLiteral(&tok)
	case jsonkit.IsLower(s.char):
		s.scanIdent(&tok)
	case jsonkit.IsQuote(s.char) || (s.mode.isExtended() && jsonkit.IsApos(s.char)):
		s.scanString(&tok)
	case jsonkit.IsNumber(s.char) || s.char == '-':
		s.scanNumber(&tok)
	case s.mode.isExtended() && (s.char == '+' || s.char == '.'):
		s.scanNumber(&tok)
	case jsonkit.IsDelim(s.char):
		s.scanDelimiter(&tok)
	default:
		tok.Type = jsonkit.Invalid
	}
	return tok
}

func (s *Scanner) scanLiteral(tok *jsonkit.Token) {
	for !s.done() && jsonkit.IsAlpha(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	switch tok.Literal {
	case "true", "false":
		tok.Type = jsonkit.Boolean
	case "null":
		tok.Type = jsonkit.Null
	default:
		tok.Type = jsonkit.Ident
	}
}

func (s *Scanner) scanComment(tok *jsonkit.Token) {
	s.read()
	s.read()
	s.skipBlank()
	for !s.done() && !jsonkit.IsNL(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	tok.Type = jsonkit.Comment
}

func (s *Scanner) scanIdent(tok *jsonkit.Token) {
	for !s.done() && jsonkit.IsAlpha(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	switch tok.Literal {
	case "true", "false":
		tok.Type = jsonkit.Boolean
	case "null":
		tok.Type = jsonkit.Null
	default:
		tok.Type = jsonkit.Invalid
	}
}

func (s *Scanner) scanString(tok *jsonkit.Token) {
	quote := s.char
	s.read()
	for !s.done() && s.char != quote {
		if s.char == '\\' {
			s.read()
			if jsonkit.IsNL(s.char) {
				s.write()
				s.read()
				continue
			}
			if ok := s.scanEscape(quote); !ok {
				tok.Type = jsonkit.Invalid
				return
			}
		}
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	tok.Type = jsonkit.String
	if s.char != quote {
		tok.Type = jsonkit.Invalid
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
			if !jsonkit.IsHex(s.char) {
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

func (s *Scanner) scanHexa(tok *jsonkit.Token) {
	s.read()
	s.read()
	s.writeRune('0')
	s.writeRune('x')
	for !s.done() && jsonkit.IsHex(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
}

func (s *Scanner) scanNumber(tok *jsonkit.Token) {
	tok.Type = jsonkit.Number
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
	for !s.done() && jsonkit.IsNumber(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	if s.char == '.' {
		s.write()
		s.read()
		if !jsonkit.IsNumber(s.char) {
			if !s.mode.isExtended() {
				tok.Type = jsonkit.Invalid
			}
			return
		}
		for !s.done() && jsonkit.IsNumber(s.char) {
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
		if !jsonkit.IsNumber(s.char) {
			tok.Type = jsonkit.Invalid
			return
		}
		for !s.done() && jsonkit.IsNumber(s.char) {
			s.write()
			s.read()
		}
		tok.Literal = s.str.String()
	}
}

func (s *Scanner) scanDelimiter(tok *jsonkit.Token) {
	switch s.char {
	case '[':
		tok.Type = jsonkit.BegArr
	case ']':
		tok.Type = jsonkit.EndArr
	case '{':
		tok.Type = jsonkit.BegObj
	case '}':
		tok.Type = jsonkit.EndObj
	case ',':
		tok.Type = jsonkit.Comma
	case ':':
		tok.Type = jsonkit.Colon
	default:
		tok.Type = jsonkit.Invalid
	}
	if tok.Type != jsonkit.Invalid {
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
