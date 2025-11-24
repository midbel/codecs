package xslt

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/midbel/codecs/environ"
	"github.com/midbel/codecs/xml"
	"github.com/midbel/codecs/xpath"
)

type Matcher interface {
	Match(xml.Node) bool
	Priority() float64
}

type wildcardMatcher struct {
	kind xml.NodeType
}

func (m wildcardMatcher) Match(node xml.Node) bool {
	if m.kind != 0 {
		return node.Type() == m.kind
	}
	return true
}

func (m wildcardMatcher) Priority() float64 {
	return 0
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
	if m.next != nil {
		ok := m.next.Match(node)
		if !ok {
			return ok
		}
	}
	return node.Type() == xml.TypeDocument
}

func (m rootMatcher) Priority() float64 {
	if m.next != nil {
		return m.next.Priority()
	}
	return 0
}

type axisMatcher struct {
	axis string
	next Matcher
}

func (m axisMatcher) Match(node xml.Node) bool {
	return m.next.Match(node)
}

func (m axisMatcher) Priority() float64 {
	return 0
}

type nameMatcher struct {
	name xml.QName
}

func (m nameMatcher) Match(node xml.Node) bool {
	var qn xml.QName
	switch n := node.(type) {
	case *xml.Element:
		qn = n.QName
	case *xml.Attribute:
		qn = n.QName
	case *xml.Instruction:
		qn = n.QName
	default:
		return false
	}
	return m.name.Equal(qn)
}

func (m nameMatcher) Priority() float64 {
	return 0.5
}

type attributeMatcher struct {
	Matcher
}

func (m attributeMatcher) Match(node xml.Node) bool {
	if node.Type() != xml.TypeAttribute {
		return false
	}
	return m.Matcher.Match(node)
}

func (m attributeMatcher) Priority() float64 {
	return 0
}

type pathMatcher struct {
	matchers []Matcher
}

func (m pathMatcher) Match(node xml.Node) bool {
	for i := len(m.matchers) - 1; i >= 0; i-- {
		if node == nil {
			return false
		}
		ok := m.matchers[i].Match(node)
		if !ok {
			return ok
		}
		node = node.Parent()
	}
	return true
}

func (m pathMatcher) Priority() float64 {
	return 0
}

type nodeMatcher struct{}

func (m nodeMatcher) Match(node xml.Node) bool {
	return xml.TypeNode&node.Type() != 0
}

func (m nodeMatcher) Priority() float64 {
	return 0
}

type textMatcher struct{}

func (m textMatcher) Match(node xml.Node) bool {
	return node.Type() == xml.TypeText
}

func (m textMatcher) Priority() float64 {
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
	return max(m.left.Priority(), m.right.Priority())
}

type predicateMatcher struct {
	curr   Matcher
	filter xpath.Expr
}

func (m predicateMatcher) Match(node xml.Node) bool {
	ok := m.curr.Match(node)
	if !ok {
		return ok
	}
	seq, _ := m.filter.Find(node)
	return seq.True()
}

func (m predicateMatcher) Priority() float64 {
	return m.curr.Priority()
}

func isTest(n string) bool {
	switch n {
	case "text", "attribute", "node", "document-node":
	default:
		return false
	}
	return true
}

type Compiler struct {
	scan *Scanner
	curr Token
	peek Token

	engine     *xpath.Evaluator
	namespaces environ.Environ[string]
}

func CompileMatch(query string) (Matcher, error) {
	return compileMatchWithEnv(nil, query)
}

func compileMatchWithEnv(env *xpath.Evaluator, query string) (Matcher, error) {
	cp := NewCompiler()
	if env != nil {
		cp.engine = env.Clone()
	}
	return cp.Compile(strings.NewReader(query))
}

func NewCompiler() *Compiler {
	var cp Compiler
	cp.engine = xpath.NewEvaluator()
	cp.namespaces = environ.Empty[string]()
	return &cp
}

func (c *Compiler) Compile(r io.Reader) (Matcher, error) {
	c.scan = Scan(r)
	c.next()
	c.next()
	return c.compile()
}

func (c *Compiler) RegisterNS(prefix, uri string) {
	c.namespaces.Define(prefix, uri)
	c.engine.RegisterNS(prefix, uri)
}

func (c *Compiler) Define(ident, value string) {
	c.engine.Define(ident, value)
}

func (c *Compiler) SetElemNS(ns string) {
	c.engine.SetElemNS(ns)
}

func (c *Compiler) SetFuncNS(ns string) {
	c.engine.SetFuncNS(ns)
}

func (c *Compiler) SetTypeNS(ns string) {
	c.engine.SetTypeNS(ns)
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
	var m Matcher
	if len(paths) == 1 {
		m = paths[0]
	} else {
		m = pathMatcher{
			matchers: paths,
		}
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
	if c.is(opPredicate) {
		return c.compilePredicate(m)
	}
	return m, nil
}

func (c *Compiler) compilePredicate(match Matcher) (Matcher, error) {
	defer c.next()

	m := predicateMatcher{
		curr: match,
	}
	expr, err := c.engine.Create(c.getCurrentLiteral())
	if err != nil {
		return nil, err
	}
	m.filter = expr
	return m, nil
}

func (c *Compiler) compileCall(qn xml.QName) (Matcher, error) {
	c.next()
	if isTest(qn.Name) {
		return c.compileTest(qn)
	}
	return nil, fmt.Errorf("call not yet supported")
}

func (c *Compiler) compileTest(qn xml.QName) (Matcher, error) {
	if !c.is(endGrp) {
		return nil, fmt.Errorf("expected \")\"")
	}
	c.next()
	var m Matcher
	if qn.Name == "text" {
		m = textMatcher{}
	} else if qn.Name == "document-node" {
		m = rootMatcher{}
	} else if qn.Name == "attribute" {
		m = attributeMatcher{
			Matcher: wildcardMatcher{},
		}
	} else {
		m = nodeMatcher{}
	}
	return m, nil
}

func (c *Compiler) compileAttribute() (Matcher, error) {
	c.next()
	if c.is(opStar) && !c.peekIs(opNamespace) {
		c.next()
		m := wildcardMatcher{
			kind: xml.TypeAttribute,
		}
		return m, nil
	}
	qn, err := c.compileQN()
	if err != nil {
		return nil, err
	}
	m := nameMatcher{
		name: qn,
	}
	a := attributeMatcher{
		Matcher: m,
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
	opPredicate
	opDigit
	opInvalid
	opCurrent
	opCurrentLevel
	opAnyLevel
	begGrp
	endGrp
	opNamespace
	opSeq
	opUnion
	opExcept
	opIntersect
	opAxis
	opStar
	opRev
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
	case opRev:
		return "<reverse>"
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
	case opPredicate:
		return fmt.Sprintf("predicate(%s)", t.Literal)
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
	case s.char == lsquare:
		s.scanPredicate(&tok)
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
	switch s.char {
	case star:
		tok.Type = opStar
	case comma:
		tok.Type = opSeq
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

func (s *Scanner) scanPredicate(tok *Token) {
	s.read()
	var count int
	count++
	for !s.done() || (s.char == rsquare && count == 0) {
		if s.char == lsquare {
			count++
		} else if s.char == rsquare {
			count--
			if count == 0 {
				break
			}
		}
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	tok.Type = opPredicate
	if s.done() {
		tok.Type = opInvalid
	}
	if s.char == rsquare {
		s.read()
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
		c == comma || c == lparen || c == rparen ||
		c == colon
}

func isOperator(c rune) bool {
	return c == star || c == comma
}
