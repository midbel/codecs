package relax

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
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
	var prefix string
	switch t.Type {
	case EOF:
		return "<eof>"
	case EOL:
		return "<eol>"
	case Comment:
		prefix = "comment"
	case Literal:
		prefix = "literal"
	case Name:
		prefix = "name"
	case Keyword:
		prefix = "keyword"
	case BegBrace:
		return "<beg-brace>"
	case EndBrace:
		return "<end-brace>"
	case BegParen:
		return "<beg-paren>"
	case EndParen:
		return "<end-paren>"
	case Comma:
		return "<comma>"
	case Alt:
		return "<alternative>"
	case MergeAlt:
		return "<merge-alt>"
	case Interleave:
		return "<interleave>"
	case MergeInter:
		return "<merge-interleave>"
	case Optional:
		return "<optional>"
	case Mandatory:
		return "<mandatory>"
	case Star:
		return "<star>"
	case Assign:
		return "<assignment>"
	case Colon:
		return "<colon>"
	case Invalid:
		prefix = "invalid"
	}
	return fmt.Sprintf("%s(%s)", prefix, t.Literal)
}

const (
	EOF = -(iota + 1)
	EOL
	Comment
	Literal
	Name
	Keyword
	BegBrace
	EndBrace
	BegParen
	EndParen
	Comma
	Interleave
	MergeInter
	Alt
	MergeAlt
	Optional  // ?
	Mandatory // +
	Star      // *
	Assign
	Colon
	Invalid
)

type Scanner struct {
	input *bufio.Reader
	char  rune
	Position
	old Position

	str    bytes.Buffer
	nested int
}

func Scan(r io.Reader) *Scanner {
	scan := Scanner{
		input: bufio.NewReader(r),
	}
	scan.Line++
	scan.read()
	return &scan
}

func (s *Scanner) Scan() Token {
	defer s.reset()

	var tok Token
	tok.Position = s.Position
	if s.char == utf8.RuneError {
		tok.Type = EOF
		return tok
	}
	s.skip(isSpace)
	if s.nested > 0 {
		s.skip(isBlank)
	}
	switch {
	case isNL(s.char):
		s.scanNL(&tok)
	case isComment(s.char):
		s.scanComment(&tok)
	case isLetter(s.char):
		s.scanName(&tok)
	case isArity(s.char):
		s.scanArity(&tok)
	case isPunct(s.char):
		s.scanPunct(&tok)
	case isQuote(s.char):
		s.scanQuote(&tok)
	default:
		tok.Type = Invalid
	}
	return tok
}

func (s *Scanner) scanNL(tok *Token) {
	tok.Type = EOL
	s.skip(isBlank)
}

func (s *Scanner) scanQuote(tok *Token) {
	s.read()
	for !s.done() && !isQuote(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.literal()
	if isQuote(s.char) {
		tok.Type = Literal
		s.read()
	} else {
		tok.Type = Invalid
	}
}

func (s *Scanner) scanComment(tok *Token) {
	s.read()
	s.skipBlank()
	for !s.done() && !isNL(s.char) {
		s.write()
		s.read()
	}
	s.read()
	tok.Type = Comment
	tok.Literal = s.literal()
}

func (s *Scanner) scanName(tok *Token) {
	for !s.done() && isAlpha(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.literal()
	tok.Type = Keyword
	switch tok.Literal {
	case "default":
	case "element":
	case "attribute":
	case "grammar":
	case "namespace":
	case "datatypes":
	case "include":
	case "external":
	case "empty":
	case "start":
	case "text":
	case "int", "float", "decimal", "bool", "string", "date":
	default:
		tok.Type = Name
	}
}

func (s *Scanner) scanPunct(tok *Token) {
	switch k := s.peek(); s.char {
	case ':':
		tok.Type = Colon
	case '=':
		tok.Type = Assign
	case '|':
		tok.Type = Alt
		if k == '=' {
			s.read()
			tok.Type = MergeAlt
		}
	case '&':
		tok.Type = Interleave
		if k == '=' {
			s.read()
			tok.Type = MergeInter
		}
	case ',':
		tok.Type = Comma
	case '{':
		tok.Type = BegBrace
		s.nested++
	case '}':
		tok.Type = EndBrace
		s.nested--
	case '(':
		tok.Type = BegParen
	case ')':
		tok.Type = EndParen
	default:
		tok.Type = Invalid
	}
	if tok.Type != Invalid {
		s.read()
	}
}

func (s *Scanner) scanArity(tok *Token) {
	switch s.char {
	case '*':
		tok.Type = Star
	case '?':
		tok.Type = Optional
	case '+':
		tok.Type = Mandatory
	default:
		tok.Type = Invalid
	}
	if tok.Type != Invalid {
		s.read()
	}
}

func (s *Scanner) read() {
	char, _, err := s.input.ReadRune()
	if errors.Is(err, io.EOF) {
		char = utf8.RuneError
	}
	s.char = char
}

func (s *Scanner) peek() rune {
	defer s.input.UnreadRune()
	c, _, _ := s.input.ReadRune()
	return c
}

func (s *Scanner) write() {
	s.str.WriteRune(s.char)
}

func (s *Scanner) done() bool {
	return s.char == utf8.RuneError
}

func (s *Scanner) literal() string {
	return s.str.String()
}

func (s *Scanner) reset() {
	s.str.Reset()
}

func (s *Scanner) skipBlank() {
	for isSpace(s.char) {
		s.read()
	}
}

func (s *Scanner) skip(accept func(rune) bool) {
	for accept(s.char) {
		s.read()
	}
}

func isComment(r rune) bool {
	return r == '#'
}

func isArity(r rune) bool {
	return r == '?' || r == '*' || r == '+'
}

func isPunct(r rune) bool {
	return r == ',' || r == '|' || r == '=' || r == ':' || r == '&' ||
		r == '(' || r == ')' || r == '{' || r == '}'
}

func isAlpha(r rune) bool {
	return isLetter(r) || isNumber(r) || r == '_'
}

func isLetter(r rune) bool {
	return isLower(r) || isUpper(r)
}

func isLower(r rune) bool {
	return r >= 'a' && r <= 'z'
}

func isUpper(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

func isNumber(r rune) bool {
	return r >= '0' && r <= '9'
}

func isQuote(r rune) bool {
	return r == '"'
}

func isBlank(r rune) bool {
	return isSpace(r) || isNL(r)
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

func isNL(r rune) bool {
	return r == '\n' || r == '\r'
}
