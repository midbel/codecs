package xml

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"html"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	errSyntax      = errors.New("syntax error")
	errType        = errors.New("invalid type")
	errZero        = errors.New("division by zero")
	errDiscard     = errors.New("discard")
	errImplemented = errors.New("not implemented")
	errArgument    = errors.New("invalid number of argument(s)")
)

const (
	powLowest = iota
	powReserv
	powAlt
	powLevel
	powOr
	powAnd
	powNe
	powEq
	powCmp
	powAdd
	powMul
	powPrefix
	powCall
	powPred
)

var bindings = map[rune]int{
	reserved:  powReserv,
	currLevel: powLevel,
	anyLevel:  powLevel,
	opAlt:     powAlt,
	opEq:      powEq,
	opNe:      powNe,
	opGt:      powCmp,
	opGe:      powCmp,
	opLt:      powCmp,
	opLe:      powCmp,
	opAnd:     powAnd,
	opOr:      powOr,
	opAdd:     powAdd,
	opSub:     powAdd,
	opMul:     powMul,
	opDiv:     powMul,
	opMod:     powMul,
	begGrp:    powCall,
	begPred:   powPred,
}

type compiler struct {
	scan *QueryScanner
	curr Token
	peek Token

	infix  map[rune]func(Expr) (Expr, error)
	prefix map[rune]func() (Expr, error)
}

func Compile(r io.Reader) (Expr, error) {
	cp := compiler{
		scan: ScanQuery(r),
	}

	cp.infix = map[rune]func(Expr) (Expr, error){
		currLevel: cp.compileDescendant,
		anyLevel:  cp.compileDescendant,
		begPred:   cp.compileFilter,
		opAdd:     cp.compileBinary,
		opSub:     cp.compileBinary,
		opMul:     cp.compileBinary,
		opDiv:     cp.compileBinary,
		opMod:     cp.compileBinary,
		opEq:      cp.compileBinary,
		opNe:      cp.compileBinary,
		opGt:      cp.compileBinary,
		opGe:      cp.compileBinary,
		opLt:      cp.compileBinary,
		opLe:      cp.compileBinary,
		opAlt:     cp.compileAlt,
		begGrp:    cp.compileCall,
		reserved:  cp.compileReservedInfix,
	}
	cp.prefix = map[rune]func() (Expr, error){
		currLevel:  cp.compileRoot,
		anyLevel:   cp.compileDescendantFromRoot,
		Name:       cp.compileName,
		currNode:   cp.compileCurrent,
		parentNode: cp.compileParent,
		attrNode:   cp.compileAttr,
		Literal:    cp.compileLiteral,
		Digit:      cp.compileNumber,
		opSub:      cp.compileReverse,
		opMul:      cp.compileName,
		begGrp:     cp.compileSequence,
		reserved:   cp.compileReservedPrefix,
	}

	cp.next()
	cp.next()
	return cp.Compile()
}

func (c *compiler) Compile() (Expr, error) {
	expr, err := c.compile()
	if err == nil {
		q := query{
			expr: expr,
		}
		expr = q
	}
	return expr, err
}

func (c *compiler) compile2() (Expr, error) {
	var (
		expr union
		do   = func() (Expr, error) {
			if c.is(currLevel) {
				return c.compileRoot()
			}
			if c.is(anyLevel) {
				return c.compileDescendantFromRoot()
			}
			return c.compileExpr(powLowest)
		}
	)
	for !c.done() {
		e, err := do()
		if err != nil {
			return nil, err
		}
		expr.all = append(expr.all, e)
		switch {
		case c.is(opAlt):
			c.next()
			if c.done() {
				return nil, errSyntax
			}
		case c.done():
		default:
			return nil, errSyntax
		}
	}
	if len(expr.all) == 1 {
		return expr.all[0], nil
	}
	return expr, nil
}

func (c *compiler) compile() (Expr, error) {
	return c.compileExpr(powLowest)
}

func (c *compiler) compileReservedPrefix() (Expr, error) {
	switch c.curr.Literal {
	case kwIf:
		return c.compileIf()
	case kwFor:
		return c.compileFor()
	case kwSome:
		return c.compileSome()
	case kwEvery:
		return c.compileEvery()
	default:
		return nil, fmt.Errorf("%s: reserved word can not be used as prefix operator")
	}
}

func (c *compiler) compileIf() (Expr, error) {
	c.next()
	return nil, errImplemented
}

func (c *compiler) compileFor() (Expr, error) {
	c.next()
	return nil, errImplemented
}

func (c *compiler) compileSome() (Expr, error) {
	c.next()
	return nil, errImplemented
}

func (c *compiler) compileEvery() (Expr, error) {
	c.next()
	return nil, errImplemented
}

func (c *compiler) compileReservedInfix(left Expr) (Expr, error) {
	keyword := c.curr.Literal
	c.next()

	expr, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	switch keyword {
	case kwUnion:
		var res union
		res.all = []Expr{left, expr}

		expr = res
	case kwIntersect:
		var res intersect
		res.all = []Expr{left, expr}

		expr = res
	case kwExcept:
		var res except
		res.all = []Expr{left, expr}
		expr = res
	default:
		return nil, fmt.Errorf("%s: reserved word can not be used as infix operator")
	}
	return expr, nil
}

func (c *compiler) compileFilter(left Expr) (Expr, error) {
	c.next()
	expr, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	if !c.is(endPred) {
		return nil, fmt.Errorf("%w: missing ']' after filter", errSyntax)
	}
	c.next()

	f := filter{
		expr:  left,
		check: expr,
	}
	return f, nil
}

func (c *compiler) compileSequence() (Expr, error) {
	return nil, errImplemented
}

func (c *compiler) compileAlt(left Expr) (Expr, error) {
	c.next()
	expr, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	var res union
	res.all = []Expr{left, expr}
	return res, nil
}

func (c *compiler) compileBinary(left Expr) (Expr, error) {
	var (
		op  = c.curr.Type
		pow = bindings[op]
	)
	c.next()
	next, err := c.compileExpr(pow)
	if err != nil {
		return nil, err
	}
	b := binary{
		left:  left,
		right: next,
		op:    op,
	}
	return createNoop(b), nil
}

func (c *compiler) compileLiteral() (Expr, error) {
	defer c.next()
	i := literal{
		expr: c.curr.Literal,
	}
	return createNoop(i), nil
}

func (c *compiler) compileNumber() (Expr, error) {
	defer c.next()
	f, err := strconv.ParseFloat(c.curr.Literal, 64)
	if err != nil {
		return nil, err
	}
	n := number{
		expr: f,
	}
	return createNoop(n), nil
}

func (c *compiler) compileReverse() (Expr, error) {
	c.next()
	expr, err := c.compileExpr(powPrefix)
	if err != nil {
		return nil, err
	}
	r := reverse{
		expr: expr,
	}
	return createNoop(r), nil
}

func (c *compiler) compileAttr() (Expr, error) {
	defer c.next()
	a := attr{
		ident: c.curr.Literal,
	}
	return createNoop(a), nil
}

func (c *compiler) compileCall(left Expr) (Expr, error) {
	compile := func(left Expr) (call, error) {
		n, ok := left.(name)
		if !ok {
			return call{}, fmt.Errorf("invalid function identifier")
		}
		fn := call{
			ident: n.ident,
		}
		c.next()
		for !c.done() && !c.is(endGrp) {
			arg, err := c.compileExpr(powLowest)
			if err != nil {
				return fn, err
			}
			fn.args = append(fn.args, arg)
			switch {
			case c.is(opSeq):
				c.next()
				if c.is(endGrp) {
					return fn, errSyntax
				}
			case c.is(endGrp):
			default:
				return fn, errSyntax
			}
		}
		if !c.is(endGrp) {
			return fn, fmt.Errorf("%w: missing closing ')'", errSyntax)
		}
		c.next()
		return fn, nil
	}
	switch e := left.(type) {
	case axis:
		expr, err := compile(e.next)
		if err != nil {
			return nil, err
		}
		e.next = expr
		return e, nil
	default:
		fn, err := compile(left)
		if err != nil {
			return nil, err
		}
		return createNoop(fn), nil
	}
}

func (c *compiler) compileExpr(pow int) (Expr, error) {
	fn, ok := c.prefix[c.curr.Type]
	if !ok {
		return nil, fmt.Errorf("unexpected prefix expression")
	}
	left, err := fn()
	if err != nil {
		return nil, err
	}
	for !c.done() && pow < bindings[c.curr.Type] {
		fn, ok := c.infix[c.curr.Type]
		if !ok {
			return nil, fmt.Errorf("unexpected infix expression")
		}
		left, err = fn(left)
		if err != nil {
			return nil, err
		}
	}
	return left, nil
}

func (c *compiler) compileName() (Expr, error) {
	if c.peek.Type == opAxis {
		return c.compileAxis()
	}
	expr, err := c.compileNameBase()
	if err != nil {
		return nil, err
	}
	a := axis{
		ident: childAxis,
		next:  expr,
	}
	return a, nil
}

func (c *compiler) compileNameBase() (Expr, error) {
	if c.is(opMul) {
		c.next()
		var a all
		return a, nil
	}
	n := name{
		ident: c.curr.Literal,
	}
	c.next()
	if c.is(Namespace) {
		c.next()
		n.space = n.ident
		if !c.is(Name) {
			return nil, fmt.Errorf("name expected")
		}
		n.ident = c.curr.Literal
		c.next()
	}
	return n, nil
}

func (c *compiler) compileCurrent() (Expr, error) {
	c.next()
	return current{}, nil
}

func (c *compiler) compileParent() (Expr, error) {
	c.next()
	return parent{}, nil
}

func (c *compiler) compileDescendant(left Expr) (Expr, error) {
	d := descendant{
		curr: left,
		deep: c.is(anyLevel),
	}
	c.next()
	next, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	d.next = next
	return d, nil
}

func (c *compiler) compileDescendantFromRoot() (Expr, error) {
	var expr root
	return c.compileDescendant(expr)
}

func (c *compiler) compileRoot() (Expr, error) {
	c.next()
	if c.done() {
		return root{}, nil
	}
	next, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	a := absolute{
		expr: next,
	}
	return a, nil
}

func (c *compiler) compileAxis() (Expr, error) {
	a := axis{
		ident: c.curr.Literal,
	}
	c.next()
	c.next()
	expr, err := c.compileNameBase()
	if err != nil {
		return nil, err
	}
	a.next = expr
	return a, nil
}

func (c *compiler) is(kind rune) bool {
	return c.curr.Type == kind
}

func (c *compiler) done() bool {
	return c.is(EOF)
}

func (c *compiler) next() {
	c.curr = c.peek
	c.peek = c.scan.Scan()
}

const (
	kwIf        = "if"
	kwElse      = "else"
	kwThen      = "then"
	kwFor       = "for"
	kwIn        = "in"
	kwUnion     = "union"
	kwIntersect = "intersect"
	kwExcept    = "except"
	kwReturn    = "return"
	kwSome      = "some"
	kwEvery     = "every"
	kwAnd       = "and"
	kwOr        = "or"
	kwDiv       = "div"
	kwMod       = "mod"
)

func isReserved(str string) bool {
	switch str {
	case kwIf:
	case kwElse:
	case kwThen:
	case kwFor:
	case kwIn:
	case kwUnion:
	case kwIntersect:
	case kwExcept:
	case kwReturn:
	case kwSome:
	case kwEvery:
	default:
		return false
	}
	return true
}

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

type QueryScanner struct {
	input io.RuneScanner
	char  rune
	str   bytes.Buffer

	Position
	old Position

	predicate bool
}

func ScanQuery(r io.Reader) *QueryScanner {
	scan := &QueryScanner{
		input: bufio.NewReader(r),
	}
	scan.Line = 1
	scan.read()
	return scan
}

func (s *QueryScanner) Scan() Token {
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

func (s *QueryScanner) scanOperator(tok *Token) {
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
		tok.Type = Invalid
	}
	if tok.Type != Invalid {
		s.read()
		s.skipBlank()
	}
}

func (s *QueryScanner) scanDelimiter(tok *Token) {
	switch k := s.peek(); s.char {
	case colon:
		tok.Type = Namespace
		if k == colon {
			s.read()
			tok.Type = opAxis
		}
	case dot:
		tok.Type = currNode
		s.read()
		if s.char == dot {
			tok.Type = parentNode
		}
	case comma:
		tok.Type = opSeq
	case pipe:
		tok.Type = opAlt
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

func (s *QueryScanner) scanLiteral(tok *Token) {
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

func (s *QueryScanner) scanAttr(tok *Token) {
	s.read()
	s.scanIdent(tok)
	tok.Type = attrNode
}

func (s *QueryScanner) scanNumber(tok *Token) {
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

func (s *QueryScanner) scanVariable(tok *Token) {
	s.read()
	for !s.done() && (unicode.IsLetter(s.char) || unicode.IsDigit(s.char)) {
		s.write()
		s.read()
	}
	tok.Type = variable
	tok.Literal = s.str.String()
}

func (s *QueryScanner) scanIdent(tok *Token) {
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
	case kwDiv:
		tok.Type = opDiv
	case kwMod:
		tok.Type = opMod
	default:
		tok.Type = Name
		if isReserved(tok.Literal) {
			tok.Type = reserved
		}
	}
	s.skipBlank()
}

func (s *QueryScanner) enterPredicate() {
	s.predicate = true
}

func (s *QueryScanner) leavePredicate() {
	s.predicate = false
}

func (s *QueryScanner) skipBlank() {
	s.skip(unicode.IsSpace)
}

func (s *QueryScanner) skip(accept func(r rune) bool) {
	for accept(s.char) {
		s.read()
	}
}

func (s *QueryScanner) write() {
	s.str.WriteRune(s.char)
}

func (s *QueryScanner) read() {
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

func (s *QueryScanner) peek() rune {
	defer s.input.UnreadRune()
	c, _, _ := s.input.ReadRune()
	return c
}

func (s *QueryScanner) done() bool {
	return s.char == utf8.RuneError
}

func isVariable(c rune) bool {
	return c == dollar
}

func isDelimiter(c rune) bool {
	return c == comma || c == dot || c == pipe || c == slash ||
		c == lsquare || c == rsquare || c == colon
}

func isOperator(c rune) bool {
	return c == plus || c == dash || c == star || c == percent ||
		c == equal || c == bang || c == langle || c == rangle ||
		c == lparen || c == rparen
}

const MaxDepth = 512

const (
	SupportedVersion  = "1.0"
	SupportedEncoding = "UTF-8"
)

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
	MaxDepth   int

	piFuncs map[string]PiFunc
}

func NewParser(r io.Reader) *Parser {
	p := Parser{
		scan:      Scan(r),
		TrimSpace: true,
		MaxDepth:  MaxDepth,
		piFuncs:   make(map[string]PiFunc),
	}
	p.next()
	p.next()
	return &p
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
	doc.root, err = p.parseNode()
	return &doc, err
}

func (p *Parser) parseProlog() (Node, error) {
	if !p.is(ProcInstTag) {
		if !p.OmitProlog {
			return nil, p.createError("document", "xml prolog missing")
		}
		return nil, nil
	}
	node, err := p.parseProcessingInstr()
	if err != nil {
		return nil, err
	}
	pi, ok := node.(*Instruction)
	if !ok {
		return nil, p.createError("document", "expected xml prolog")
	}
	ok = slices.ContainsFunc(pi.Attrs, func(a Attribute) bool {
		return a.LocalName() == "version" && a.Value == SupportedVersion
	})
	if !ok {
		return nil, p.createError("document", "xml version not supported")
	}
	ix := slices.IndexFunc(pi.Attrs, func(a Attribute) bool {
		return a.LocalName() == "encoding"
	})
	if ix >= 0 && pi.Attrs[ix].Value != SupportedEncoding {
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
		node, err = p.parseProcessingInstr()
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
	p.next()
	var (
		elem Element
		err  error
	)
	if p.is(Namespace) {
		elem.Space = p.curr.Literal
		p.next()
	}
	if !p.is(Name) {
		return nil, p.createError("element", "name is missing")
	}
	elem.Name = p.curr.Literal
	p.next()

	elem.Attrs, err = p.parseAttributes(func() bool {
		return p.is(EndTag) || p.is(EmptyElemTag)
	})
	if err != nil {
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
	if p.is(Namespace) {
		if elem.Space != p.curr.Literal {
			return p.createError("element", "namespace mismatched with opening element")
		}
		p.next()
	}
	if !p.is(Name) {
		return p.createError("element", "name is missing")
	}
	if p.curr.Literal != elem.Name {
		return p.createError("element", "name mismatched with opening element")
	}
	p.next()
	if !p.is(EndTag) {
		return p.createError("element", "end of element expected")
	}
	p.next()
	return nil
}

func (p *Parser) parseProcessingInstr() (Node, error) {
	p.next()
	if !p.is(Name) {
		return nil, p.createError("processing instruction", "name is missing")
	}
	var elem Instruction
	elem.Name = p.curr.Literal
	p.next()
	var err error
	elem.Attrs, err = p.parseAttributes(func() bool {
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

func (p *Parser) parseAttributes(done func() bool) ([]Attribute, error) {
	var attrs []Attribute
	for !p.done() && !done() {
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
		attrs = append(attrs, attr)
	}
	return attrs, nil
}

func (p *Parser) parseAttr() (Attribute, error) {
	var attr Attribute
	if p.is(Namespace) {
		attr.Space = p.curr.Literal
		p.next()
	}
	if !p.is(Attr) {
		return attr, p.createError("attribute", "name is expected")
	}
	attr.Name = p.curr.Literal
	p.next()
	if !p.is(Literal) {
		return attr, p.createError("attribute", "value is missing")
	}
	attr.Value = p.curr.Literal
	p.next()
	return attr, nil
}

func (p *Parser) parseComment() (Node, error) {
	defer p.next()
	node := Comment{
		Content: p.curr.Literal,
	}
	return &node, nil
}

func (p *Parser) parseCharData() (Node, error) {
	defer p.next()
	char := CharData{
		Content: p.curr.Literal,
	}
	return &char, nil
}

func (p *Parser) parseLiteral() (Node, error) {
	text := Text{
		Content: p.curr.Literal,
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
	case variable:
		return fmt.Sprintf("variable(%s)", t.Literal)
	case reserved:
		return fmt.Sprintf("reserved(%s)", t.Literal)
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
	scan := &Scanner{
		input: bufio.NewReader(r),
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
