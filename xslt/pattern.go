package xslt

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"unicode"
	"unicode/utf8"

	"github.com/midbel/codecs/environ"
	"github.com/midbel/codecs/xml"
)

type Matcher interface {
	Match(xml.Node) bool
	Priority() float64
}

type currentMatcher struct{}

func (m currentMatcher) Match(node xml.Node) bool {
	return true
}

func (m currentMatcher) Priority() float64 {
	return 0
}

type rootMatcher struct {
	next Matcher
}

func (m rootMatcher) Match(node xml.Node) bool {
	return true
}

func (m rootMatcher) Priority() float64 {
	return 0
}

type axisMatcher struct {
	axis string
	next Matcher
}

func (m axisMatcher) Match(node xml.Node) bool {
	return false
}

func (m axisMatcher) Priority() float64 {
	return 0
}

type nameMatcher struct {
	name xml.QName
}

func (m nameMatcher) Match(node xml.Node) bool {
	return false
}

func (m nameMatcher) Priority() float64 {
	return 0
}

type attributeMatcher struct {
	name xml.QName
}

func (m attributeMatcher) Match(node xml.Node) bool {
	return false
}

func (m attributeMatcher) Priority() float64 {
	return 0
}

type wildcardMatcher struct{}

func (m wildcardMatcher) Match(node xml.Node) bool {
	return true
}

func (m wildcardMatcher) Priority() float64 {
	return 0
}

type pathMatcher struct {
	matchers []Matcher
}

func (m pathMatcher) Match(node xml.Node) bool {
	return false
}

func (m pathMatcher) Priority() float64 {
	return 0
}

type nodeMatcher struct{}

func (m nodeMatcher) Match(node xml.Node) bool {
	return false
}

func (m nodeMatcher) Priority() float64 {
	return 0
}

type textMatcher struct{}

func (m textMatcher) Match(node xml.Node) bool {
	return false
}

func (m textMatcher) Priority() float64 {
	return 0
}

type callMatcher struct {
	name xml.QName
	args []Matcher
}

func (m callMatcher) Match(node xml.Node) bool {
	return false
}

func (m callMatcher) Priority() float64 {
	return 0
}

type unionMatcher struct {
	left  Matcher
	right Matcher
}

func (m unionMatcher) Match(node xml.Node) bool {
	return m.left.Match(node) || m.right.Match(node)
}

func (m unionMatcher) Priority() float64 {
	return 0
}

type predicateMatcher struct {
	curr   Matcher
	filter Matcher
}

func (m predicateMatcher) Match(node xml.Node) bool {
	return m.curr.Match(node) && m.filter.Match(node)
}

func (m predicateMatcher) Priority() float64 {
	return m.curr.Priority() + m.filter.Priority()
}

type Compiler struct {
	scan *Scanner
	curr Token
	peek Token

	namespaces environ.Environ[string]
}

func NewCompiler() *Compiler {
	var cp Compiler
	cp.namespaces = environ.Empty[string]()
	return &cp
}

func (c *Compiler) Compile(r io.Reader) (Matcher, error) {
	c.scan = Scan(r)
	c.next()
	c.next()
	return c.compile()
}

func (c *Compiler) compile() (Matcher, error) {
	var paths []Matcher
	switch {
	case c.is(opCurrentLevel):
		return c.compileFromRoot()
	case c.is(opAnyLevel):
		return c.compileFromAny()
	default:
	}
	for {
		m, err := c.compilePath()
		if err != nil {
			return nil, err
		}
		paths = append(paths, m)
		if c.done() || c.is(opUnion) || c.is(opIntersect) || c.is(opExcept) {
			break
		}
		if !c.is(opCurrentLevel) && !c.is(opAnyLevel) {
			return nil, fmt.Errorf("\"/\" or \"//\" expected")
		}
		if len(paths) > 2 {
			return nil, fmt.Errorf("only one step allowed in pattern")
		}
		c.next()
	}
	m := pathMatcher{
		matchers: paths,
	}
	if c.is(opUnion) || c.is(opIntersect) || c.is(opExcept) {
		c.next()
		u := unionMatcher{
			left: m,
		}
		n, err := c.compile()
		if err != nil {
			return nil, err
		}
		u.right = n
		return u, nil
	}
	return m, nil
}

func (c *Compiler) compileFromRoot() (Matcher, error) {
	c.next()
	var (
		root rootMatcher
		err  error
	)
	if c.done() || c.is(opUnion) || c.is(opIntersect) || c.is(opExcept) {
		return root, nil
	}
	if root.next, err = c.compile(); err != nil {
		return nil, err
	}
	return root, nil
}

func (c *Compiler) compileFromAny() (Matcher, error) {
	c.next()
	m, err := c.compile()
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (c *Compiler) compilePath() (Matcher, error) {
	if c.peekIs(opAxis) {
		if !c.is(opName) {
			return nil, fmt.Errorf("name expected")
		}
		axis := c.getCurrentLiteral()
		switch axis {
		case "child":
		case "descendant":
		case "attribute":
		case "self":
		case "descendant-or-self":
		case "namespace":
		default:
			return nil, fmt.Errorf("invalid axis")
		}
		c.next()
		c.next()
		next, err := c.compilePath()
		if err != nil {
			return nil, err
		}
		a := axisMatcher{
			axis: axis,
			next: next,
		}
		return a, nil
	}
	m, err := c.compileName()
	if err != nil {
		return nil, err
	}
	if c.is(begPred) {
		return c.compilePredicate(m)
	}
	return m, nil
}

func (c *Compiler) compilePredicate(match Matcher) (Matcher, error) {
	m := predicateMatcher{
		curr: match,
	}
	c.next()
	return m, nil
}

func (c *Compiler) compileCall(qn xml.QName) (Matcher, error) {
	c.next()
	call := callMatcher{
		name: qn,
	}
	for !c.done() && !c.is(endGrp) {
		arg, err := c.compilePath()
		if err != nil {
			return nil, err
		}
		call.args = append(call.args, arg)
		switch {
		case c.is(opSeq):
			c.next()
		case c.is(endGrp):
		default:
			return nil, fmt.Errorf("call: unexpected token")
		}
	}
	if !c.is(endGrp) {
		return nil, fmt.Errorf("missing \")\"")
	}
	c.next()
	return call, nil
}

func (c *Compiler) compileLiteral() (Matcher, error) {
	switch {
	case c.is(opLiteral):
	case c.is(opDigit):
	default:
		return nil, fmt.Errorf("literal: unexpected token")
	}
}

func (c *Compiler) compileAttribute() (Matcher, error) {
	c.next()
	if c.is(opStar) && !c.peekIs(opNamespace) {
		c.next()
		var m wildcardMatcher
		return m, nil
	}
	qn, err := c.compileQN()
	if err != nil {
		return nil, err
	}
	a := attributeMatcher{
		name: qn,
	}
	return a, nil
}

func (c *Compiler) compileName() (Matcher, error) {
	if c.is(opCurrent) {
		c.next()
		var m currentMatcher
		return m, nil
	}
	if c.is(opAttribute) {
		return c.compileAttribute()
	}
	if c.is(opStar) && !c.peekIs(opNamespace) {
		c.next()
		var m wildcardMatcher
		return m, nil
	}
	qn, err := c.compileQN()
	if err != nil {
		return nil, err
	}
	if c.is(begGrp) {
		return c.compileCall(qn)
	}
	m := nameMatcher{
		name: qn,
	}
	return m, nil
}

func (c *Compiler) compileQN() (xml.QName, error) {
	var qn xml.QName

	if !c.is(opName) && !c.is(opStar) {
		return qn, fmt.Errorf("name/* expected")
	}

	qn.Name = c.getCurrentLiteral()
	c.next()

	if c.is(opNamespace) {
		c.next()
		if !c.is(opName) && !c.is(opStar) {
			return qn, fmt.Errorf("name/* expected")
		}
		qn.Space = qn.Name
		qn.Name = c.getCurrentLiteral()
		c.next()
	}
	return qn, nil
}

func (c *Compiler) getCurrentLiteral() string {
	return c.curr.Literal
}

func (c *Compiler) next() {
	c.curr = c.peek
	c.peek = c.scan.Scan()
}

func (c *Compiler) is(kind rune) bool {
	return c.curr.Type == kind
}

func (c *Compiler) peekIs(kind rune) bool {
	return c.peek.Type == kind
}

func (c *Compiler) done() bool {
	return c.is(opEOF)
}

const (
	opEOF rune = -(1 + iota)
	opName
	opVariable
	opAttribute
	opLiteral
	opDigit
	opInvalid
	opCurrent
	opCurrentLevel
	opAnyLevel
	begPred
	endPred
	begGrp
	endGrp
	opNamespace
	opSeq
	opUnion
	opExcept
	opIntersect
	opAxis
	opStar
	opEq
	opNe
	opLt
	opLe
	opGt
	opGe
)

type Token struct {
	Literal string
	Type    rune
}

func (t Token) Invalid() bool {
	return t.Type == opInvalid
}

func (t Token) Done() bool {
	return t.Type == opEOF
}

func (t Token) String() string {
	switch t.Type {
	case opIntersect:
		return "<intersect>"
	case opUnion:
		return "<union>"
	case opExcept:
		return "<except>"
	case opAxis:
		return "<axis>"
	case opStar:
		return "<star>"
	case opCurrentLevel:
		return fmt.Sprintf("current-level(%s)", t.Literal)
	case opAnyLevel:
		return fmt.Sprintf("any-level(%s)", t.Literal)
	case begPred:
		return "<begin-predicate>"
	case endPred:
		return "<end-predicate>"
	case begGrp:
		return "<begin-group>"
	case endGrp:
		return "<end-group>"
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
	case opSeq:
		return "<sequence>"
	case opEOF:
		return "<eof>"
	case opAttribute:
		return fmt.Sprintf("<attribute>")
	case opDigit:
		return fmt.Sprintf("number(%s)", t.Literal)
	case opCurrent:
		return "<current>"
	case opVariable:
		return fmt.Sprintf("variable(%s)", t.Literal)
	case opName:
		return fmt.Sprintf("name(%s)", t.Literal)
	case opLiteral:
		return fmt.Sprintf("literal(%s)", t.Literal)
	case opInvalid:
		return "<invalid>"
	default:
		return "<unknown>"
	}
}

type Scanner struct {
	input *bufio.Reader
	char  rune
	str   bytes.Buffer
}

func Scan(r io.Reader) *Scanner {
	scan := &Scanner{
		input: bufio.NewReader(r),
	}
	scan.read()
	return scan
}

func (s *Scanner) Scan() Token {
	var tok Token
	if s.done() {
		tok.Type = opEOF
		return tok
	}
	s.str.Reset()

	s.skipBlank()
	switch {
	case isOperator(s.char):
		s.scanOperator(&tok)
	case isDelimiter(s.char):
		s.scanDelimiter(&tok)
	case s.char == arobase:
		s.scanAttr(&tok)
	case s.char == apos || s.char == quote:
		s.scanLiteral(&tok)
	case s.char == dollar:
		s.scanVariable(&tok)
	case unicode.IsLetter(s.char):
		s.scanIdent(&tok)
	case unicode.IsDigit(s.char):
		s.scanNumber(&tok)
	default:
		tok.Type = opInvalid
	}
	return tok
}

func (s *Scanner) scanOperator(tok *Token) {
	switch k := s.peek(); s.char {
	case star:
		tok.Type = opStar
	case equal:
		tok.Type = opEq
	case bang:
		tok.Type = opInvalid
		if k == equal {
			s.read()
			tok.Type = opNe
		}
	case langle:
		tok.Type = opLt
		if k == equal {
			s.read()
			tok.Type = opLe
		}
	case rangle:
		tok.Type = opGt
		if k == equal {
			s.read()
			tok.Type = opGe
		}
	case lparen:
		tok.Type = begGrp
	case rparen:
		tok.Type = endGrp
	default:
		tok.Type = opInvalid
	}
	if tok.Type != opInvalid {
		s.read()
		s.skipBlank()
	}
}

func (s *Scanner) scanDelimiter(tok *Token) {
	switch k := s.peek(); s.char {
	case colon:
		tok.Type = opNamespace
		if k == colon {
			s.read()
			tok.Type = opAxis
		}
	case dot:
		tok.Type = opCurrent
	case comma:
		tok.Type = opSeq
	case pipe:
		tok.Type = opUnion
	case lsquare:
		tok.Type = begPred
	case rsquare:
		tok.Type = endPred
	case lparen:
		tok.Type = begGrp
	case rparen:
		tok.Type = endGrp
	case slash:
		tok.Type = opCurrentLevel
		if k == slash {
			s.read()
			tok.Type = opAnyLevel
		}
	default:
		tok.Type = opInvalid
	}
	if tok.Type != opInvalid {
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
	tok.Type = opLiteral
	tok.Literal = s.str.String()
	if s.char != quote {
		tok.Type = opInvalid
	}
	s.read()
}

func (s *Scanner) scanVariable(tok *Token) {
	s.read()
	s.scanIdent(tok)
	tok.Type = opVariable
}

func (s *Scanner) scanAttr(tok *Token) {
	s.read()
	tok.Type = opAttribute
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
	tok.Type = opName
	switch tok.Literal {
	case "union":
		tok.Type = opUnion
	case "intersect":
		tok.Type = opIntersect
	case "except":
		tok.Type = opExcept
	default:
	}
	s.skipBlank()
}

func (s *Scanner) scanNumber(tok *Token) {
	for !s.done() && unicode.IsDigit(s.char) {
		s.write()
		s.read()
	}
	tok.Type = opDigit
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
	space      = ' '
	tab        = '\t'
)

func isBlank(c rune) bool {
	return c == space || c == tab
}

func isDelimiter(c rune) bool {
	return c == dot || c == pipe || c == slash ||
		c == comma || c == lsquare || c == rsquare ||
		c == lparen || c == rparen || c == colon
}

func isOperator(c rune) bool {
	return c == star || c == equal || c == bang
}
