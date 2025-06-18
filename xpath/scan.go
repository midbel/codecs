package xpath

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"unicode"
	"unicode/utf8"
)

type Position struct {
	Line   int
	Column int
}

const (
	kwLet       = "let"
	kwIf        = "if"
	kwElse      = "else"
	kwThen      = "then"
	kwFor       = "for"
	kwIn        = "in"
	kwTo        = "to"
	kwUnion     = "union"
	kwIntersect = "intersect"
	kwExcept    = "except"
	kwReturn    = "return"
	kwSome      = "some"
	kwEvery     = "every"
	kwSatisfies = "satisfies"
	kwAnd       = "and"
	kwOr        = "or"
	kwDiv       = "div"
	kwMod       = "mod"
	kwAs        = "as"
	kwIs        = "is"
	kwCast      = "cast"
	kwCastable  = "castable"
	kwMap       = "map"
	kwArray     = "array"
)

func isReserved(str string) bool {
	switch str {
	case kwLet:
	case kwIf:
	case kwElse:
	case kwThen:
	case kwFor:
	case kwIn:
	case kwTo:
	case kwUnion:
	case kwIntersect:
	case kwExcept:
	case kwReturn:
	case kwSome:
	case kwEvery:
	case kwSatisfies:
	case kwCast:
	case kwCastable:
	case kwAs:
	case kwIs:
	case kwMap:
	case kwArray:
	default:
		return false
	}
	return true
}

const (
	EOF rune = -(1 + iota)
	Name
	Namespace // name:
	Attr      // name=
	Literal
	Digit
	Invalid
)

const (
	currNode = -(iota + 1000)
	parentNode
	attrNode
	reserved
	variable
	currLevel
	anyLevel
	begPred
	endPred
	begGrp
	endGrp
	begCurl
	endCurl
	opAssign
	opArrow
	opRange
	opConcat
	opBefore
	opAfter
	opAdd
	opSub
	opMul
	opDiv
	opMod
	opEq
	opNe
	opGt
	opGe
	opLt
	opLe
	opAlt
	opAnd
	opOr
	opSeq
	opAxis
)

type Token struct {
	Literal string
	Type    rune
	Position
}

func (t Token) String() string {
	switch t.Type {
	case opAxis:
		return "<axis>"
	case currNode:
		return "<current-node>"
	case parentNode:
		return "<parent-node>"
	case attrNode:
		return fmt.Sprintf("attribute(%s)", t.Literal)
	case currLevel:
		return fmt.Sprintf("current-level(%s)", t.Literal)
	case anyLevel:
		return fmt.Sprintf("any-level(%s)", t.Literal)
	case begPred:
		return "<begin-predicate>"
	case endPred:
		return "<end-predicate>"
	case begGrp:
		return "<begin-group>"
	case endGrp:
		return "<end-group>"
	case begCurl:
		return "<begin-curly>"
	case endCurl:
		return "<end-curly>"
	case opAdd:
		return "<add>"
	case opSub:
		return "<subtract>"
	case opMul:
		return "<multiply>"
	case opDiv:
		return "<divide>"
	case opMod:
		return "<modulo>"
	case opAssign:
		return "<assignment>"
	case opArrow:
		return "<arrow>"
	case opRange:
		return "<range>"
	case opEq:
		return "<equal>"
	case opNe:
		return "<not-equal>"
	case opGt:
		return "<greater-than>"
	case opGe:
		return "<greater-eq>"
	case opLt:
		return "<lesser-than>"
	case opLe:
		return "<lesser-eq>"
	case opAlt:
		return "<alternative>"
	case opAnd:
		return "<and>"
	case opOr:
		return "<or>"
	case opSeq:
		return "<sequence>"
	case EOF:
		return "<eof>"
	case Digit:
		return fmt.Sprintf("number(%s)", t.Literal)
	case Name:
		return fmt.Sprintf("name(%s)", t.Literal)
	case Namespace:
		return fmt.Sprintf("namespace(%s)", t.Literal)
	case Attr:
		return fmt.Sprintf("attr(%s)", t.Literal)
	case Literal:
		return fmt.Sprintf("literal(%s)", t.Literal)
	case variable:
		return fmt.Sprintf("variable(%s)", t.Literal)
	case reserved:
		return fmt.Sprintf("reserved(%s)", t.Literal)
	case Invalid:
		return "<invalid>"
	default:
		return "<unknown>"
	}
}

type Scanner struct {
	input io.RuneScanner
	char  rune
	str   bytes.Buffer

	Position
	old Position

	predicate bool
}

func Scan(r io.Reader) *Scanner {
	scan := &Scanner{
		input: bufio.NewReader(r),
	}
	scan.Line = 1
	scan.read()
	return scan
}

func (s *Scanner) Scan() Token {
	var tok Token
	if s.done() {
		tok.Position = s.Position
		tok.Type = EOF
		return tok
	}
	s.str.Reset()

	s.skipBlank()
	tok.Position = s.Position
	switch {
	case isOperator(s.char):
		s.scanOperator(&tok)
	case isDelimiter(s.char):
		s.scanDelimiter(&tok)
	case s.char == arobase:
		s.scanAttr(&tok)
	case s.char == apos || s.char == quote:
		s.scanLiteral(&tok)
	case isVariable(s.char):
		s.scanVariable(&tok)
	case unicode.IsLetter(s.char):
		s.scanIdent(&tok)
	case unicode.IsDigit(s.char):
		s.scanNumber(&tok)
	default:
		tok.Type = Invalid
	}
	return tok
}

func (s *Scanner) scanOperator(tok *Token) {
	switch k := s.peek(); s.char {
	case plus:
		tok.Type = opAdd
	case dash:
		tok.Type = opSub
	case star:
		tok.Type = opMul
	case percent:
		tok.Type = opMod
	case equal:
		tok.Type = opEq
		if k := s.peek(); k == rangle {
			s.read()
			tok.Type = opArrow
		}
	case bang:
		tok.Type = Invalid
		if k == equal {
			s.read()
			tok.Type = opNe
		}
	case langle:
		tok.Type = opLt
		if k == equal {
			s.read()
			tok.Type = opLe
		} else if k == langle {
			s.read()
			tok.Type = opBefore
		}
	case rangle:
		tok.Type = opGt
		if k == equal {
			s.read()
			tok.Type = opGe
		} else if k == rangle {
			s.read()
			tok.Type = opAfter
		}
	case lparen:
		tok.Type = begGrp
	case rparen:
		tok.Type = endGrp
	default:
		tok.Type = Invalid
	}
	if tok.Type != Invalid {
		s.read()
		s.skipBlank()
	}
}

func (s *Scanner) scanDelimiter(tok *Token) {
	switch k := s.peek(); s.char {
	case colon:
		tok.Type = Namespace
		if k == colon {
			s.read()
			tok.Type = opAxis
		} else if k == equal {
			s.read()
			tok.Type = opAssign
		}
	case dot:
		tok.Type = currNode
		if k == s.char {
			s.read()
			tok.Type = parentNode
		}
	case comma:
		tok.Type = opSeq
	case pipe:
		tok.Type = opAlt
		if k == s.char {
			s.read()
			tok.Type = opConcat
		}
	case lcurly:
		tok.Type = begCurl
	case rcurly:
		tok.Type = endCurl
	case lsquare:
		tok.Type = begPred
		s.enterPredicate()
	case rsquare:
		tok.Type = endPred
		s.leavePredicate()
	case slash:
		tok.Type = currLevel
		if k := s.peek(); k == slash {
			s.read()
			tok.Type = anyLevel
		}
	default:
		tok.Type = Invalid
	}
	if tok.Type != Invalid {
		s.read()
		s.skipBlank()
	}
}

func (s *Scanner) scanLiteral(tok *Token) {
	quote := s.char
	s.read()
	for !s.done() && s.char != quote {
		s.write()
		s.read()
	}
	tok.Type = Literal
	tok.Literal = s.str.String()
	if s.char != quote {
		tok.Type = Invalid
	}
	s.read()
}

func (s *Scanner) scanAttr(tok *Token) {
	s.read()
	s.scanIdent(tok)
	tok.Type = attrNode
}

func (s *Scanner) scanNumber(tok *Token) {
	for !s.done() && unicode.IsDigit(s.char) {
		s.write()
		s.read()
	}
	tok.Type = Digit
	tok.Literal = s.str.String()
	if s.char != dot {
		return
	}
	s.write()
	s.read()
	for !s.done() && unicode.IsDigit(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	if s.char != 'e' && s.char != 'E' {
		return
	}
	s.write()
	s.read()
	if s.char == '-' || s.char == '+' {
		s.write()
		s.read()
	}
	for !s.done() && unicode.IsDigit(s.char) {
		s.write()
		s.read()
	}
}

func (s *Scanner) scanVariable(tok *Token) {
	s.read()
	for !s.done() && (unicode.IsLetter(s.char) || unicode.IsDigit(s.char) || s.char == underscore) {
		s.write()
		s.read()
	}
	tok.Type = variable
	tok.Literal = s.str.String()
}

func (s *Scanner) scanIdent(tok *Token) {
	accept := func() bool {
		return unicode.IsLetter(s.char) || unicode.IsDigit(s.char) ||
			s.char == dash || s.char == underscore
	}
	for !s.done() && accept() {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	switch tok.Literal {
	case kwAnd:
		tok.Type = opAnd
	case kwOr:
		tok.Type = opOr
	case kwTo:
		tok.Type = opRange
	case kwDiv:
		tok.Type = opDiv
	case kwMod:
		tok.Type = opMod
	case "eq":
		tok.Type = opEq
	case "ne":
		tok.Type = opNe
	case "lt":
		tok.Type = opLt
	case "le":
		tok.Type = opLe
	case "gt":
		tok.Type = opGt
	case "ge":
		tok.Type = opGe
	default:
		if isReserved(tok.Literal) {
			tok.Type = reserved
		} else {
			tok.Type = Name
		}
	}
	s.skipBlank()
}

func (s *Scanner) enterPredicate() {
	s.predicate = true
}

func (s *Scanner) leavePredicate() {
	s.predicate = false
}

func (s *Scanner) skipBlank() {
	s.skip(unicode.IsSpace)
}

func (s *Scanner) skip(accept func(r rune) bool) {
	for accept(s.char) {
		s.read()
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
	c, _, err := s.input.ReadRune()
	if err != nil {
		s.char = utf8.RuneError
	} else {
		s.char = c
	}
}

func (s *Scanner) peek() rune {
	defer s.input.UnreadRune()
	c, _, _ := s.input.ReadRune()
	return c
}

func (s *Scanner) done() bool {
	return s.char == utf8.RuneError
}

const (
	langle     = '<'
	rangle     = '>'
	lsquare    = '['
	rsquare    = ']'
	lparen     = '('
	rparen     = ')'
	lcurly     = '{'
	rcurly     = '}'
	colon      = ':'
	quote      = '"'
	apos       = '\''
	slash      = '/'
	question   = '?'
	bang       = '!'
	equal      = '='
	ampersand  = '&'
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

func isVariable(c rune) bool {
	return c == dollar
}

func isDelimiter(c rune) bool {
	return c == comma || c == dot || c == pipe || c == slash ||
		c == lsquare || c == rsquare || c == colon ||
		c == lcurly || c == rcurly
}

func isOperator(c rune) bool {
	return c == plus || c == dash || c == star || c == percent ||
		c == equal || c == bang || c == langle || c == rangle ||
		c == lparen || c == rparen
}
