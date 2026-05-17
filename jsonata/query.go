package jsonata

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/midbel/codecs/internal/jsonkit"
	"github.com/midbel/codecs/json"
)

func Find(r io.Reader, q string) (any, error) {
	doc, err := json.Parse(r)
	if err != nil {
		return nil, err
	}
	if q == "" {
		return doc, nil
	}
	query, err := Compile(q)
	if err != nil {
		return nil, err
	}
	return query.Get(doc)
}

const (
	powLowest = iota
	powComma
	powTernary
	powOr
	powAnd
	powCmp
	powEq
	powAdd
	powMul
	powPrefix
	powCall
	powGrp
	powMap
	powFilter
	powTransform
)

var bindings = map[rune]int{
	jsonkit.BegGrp:    powCall,
	jsonkit.BegArr:    powFilter,
	jsonkit.BegObj:    powFilter,
	jsonkit.Ternary:   powTernary,
	jsonkit.And:       powAnd,
	jsonkit.Or:        powOr,
	jsonkit.Add:       powAdd,
	jsonkit.Sub:       powAdd,
	jsonkit.Mul:       powMul,
	jsonkit.Div:       powMul,
	jsonkit.Mod:       powMul,
	jsonkit.Wildcard:  powMul,
	jsonkit.Parent:    powMul,
	jsonkit.Eq:        powEq,
	jsonkit.Ne:        powEq,
	jsonkit.In:        powCmp,
	jsonkit.Lt:        powCmp,
	jsonkit.Le:        powCmp,
	jsonkit.Gt:        powCmp,
	jsonkit.Ge:        powCmp,
	jsonkit.Concat:    powAdd,
	jsonkit.Map:       powMap,
	jsonkit.Transform: powTransform,
}

type compiler struct {
	scan *QueryScanner
	curr jsonkit.Token
	peek jsonkit.Token

	prefix map[rune]func() (Expr, error)
	infix  map[rune]func(Expr) (Expr, error)
}

func Compile(query string) (Query, error) {
	cp := compiler{
		scan: ScanQuery(strings.NewReader(query)),
	}
	cp.prefix = map[rune]func() (Expr, error){
		jsonkit.Ident:    cp.compileIdent,
		jsonkit.Func:     cp.compileIdent,
		jsonkit.Number:   cp.compileNumber,
		jsonkit.String:   cp.compileString,
		jsonkit.Boolean:  cp.compileBool,
		jsonkit.BegGrp:   cp.compileGroup,
		jsonkit.Wildcard: cp.compileWildcard,
		jsonkit.Descent:  cp.compileDescent,
		jsonkit.Sub:      cp.compileReverse,
		jsonkit.BegArr:   cp.compileArray,
		jsonkit.BegObj:   cp.compileObjectPrefix,
	}

	cp.infix = map[rune]func(Expr) (Expr, error){
		jsonkit.BegGrp:    cp.compileCall,
		jsonkit.BegArr:    cp.compileFilter,
		jsonkit.BegObj:    cp.compileObject,
		jsonkit.And:       cp.compileBinary,
		jsonkit.Or:        cp.compileBinary,
		jsonkit.Add:       cp.compileBinary,
		jsonkit.Sub:       cp.compileBinary,
		jsonkit.Mul:       cp.compileBinary,
		jsonkit.Wildcard:  cp.compileBinary,
		jsonkit.Div:       cp.compileBinary,
		jsonkit.Mod:       cp.compileBinary,
		jsonkit.Parent:    cp.compileBinary,
		jsonkit.Eq:        cp.compileBinary,
		jsonkit.Ne:        cp.compileBinary,
		jsonkit.Lt:        cp.compileBinary,
		jsonkit.Le:        cp.compileBinary,
		jsonkit.Gt:        cp.compileBinary,
		jsonkit.Ge:        cp.compileBinary,
		jsonkit.Concat:    cp.compileBinary,
		jsonkit.In:        cp.compileBinary,
		jsonkit.Map:       cp.compileMap,
		jsonkit.Ternary:   cp.compileTernary,
		jsonkit.Transform: cp.compileTransform,
	}

	cp.next()
	cp.next()
	return cp.Compile()
}

func (c *compiler) Compile() (Query, error) {
	return c.compile()
}

func (c *compiler) compile() (Query, error) {
	e, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	q := query{
		expr: e,
	}
	return q, nil
}

func (c *compiler) compileTransform(left Expr) (Expr, error) {
	expr := transform{
		expr: left,
	}
	c.next()
	next, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	expr.next = next
	return expr, nil
}

func (c *compiler) compileMap(left Expr) (Expr, error) {
	c.next()
	q := path{
		expr: left,
	}
	next, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	q.next = next
	return q, nil
}

func (c *compiler) compileFilter(left Expr) (Expr, error) {
	c.next()
	if c.is(jsonkit.EndArr) {
		c.next()
		a := arrayTransform{
			expr: left,
		}
		if c.is(jsonkit.BegArr) {
			left, err := c.compileFilter(left)
			if err != nil {
				return nil, err
			}
			a.expr = left
		}
		return a, nil
	}
	expr, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	if !c.is(jsonkit.EndArr) {
		return nil, fmt.Errorf("syntax error: missing ]")
	}
	c.next()

	f := filter{
		expr:  left,
		check: expr,
	}
	return f, nil
}

func (c *compiler) getString() string {
	defer c.next()
	return c.curr.Literal
}

func (c *compiler) getNumber() float64 {
	defer c.next()
	n, _ := strconv.ParseFloat(c.curr.Literal, 64)
	return n
}

func (c *compiler) getBool() bool {
	defer c.next()
	b, _ := strconv.ParseBool(c.curr.Literal)
	return b
}

func (c *compiler) compileExpr(pow int) (Expr, error) {
	fn, ok := c.prefix[c.curr.Type]
	if !ok {
		return nil, fmt.Errorf("syntax error: invalid prefix expression")
	}
	left, err := fn()
	if err != nil {
		return nil, err
	}
	for !c.is(jsonkit.EndArr) && pow < bindings[c.curr.Type] {
		fn, ok := c.infix[c.curr.Type]
		if !ok {
			return nil, fmt.Errorf("syntax error: invalid infix expression")
		}
		left, err = fn(left)
		if err != nil {
			return nil, err
		}
	}
	return left, nil
}

func (c *compiler) compileArray() (Expr, error) {
	c.next()
	var b arrayBuilder
	for !c.done() && !c.is(jsonkit.EndArr) {
		expr, err := c.compileExpr(powComma)
		if err != nil {
			return nil, err
		}
		b.expr = append(b.expr, expr)
		switch {
		case c.is(jsonkit.Comma):
			c.next()
		case c.is(jsonkit.EndArr):
		default:
			return nil, fmt.Errorf("syntax error: expected ',' or ']")
		}
	}
	if !c.is(jsonkit.EndArr) {
		return nil, fmt.Errorf("syntax error: missing ']")
	}
	c.next()
	return b, nil
}

func (c *compiler) compileObjectPrefix() (Expr, error) {
	return c.compileObject(nil)
}

func (c *compiler) compileObject(left Expr) (Expr, error) {
	c.next()
	b := objectBuilder{
		expr: left,
		list: make(map[Expr]Expr),
	}
	for !c.done() && !c.is(jsonkit.EndObj) {
		key, err := c.compileExpr(powLowest)
		if err != nil {
			return nil, err
		}
		if !c.is(jsonkit.Colon) {
			return nil, fmt.Errorf("syntax error: expected ':'")
		}
		c.next()
		val, err := c.compileExpr(powLowest)
		if err != nil {
			return nil, err
		}
		b.list[key] = val
		switch {
		case c.is(jsonkit.Comma):
			c.next()
		case c.is(jsonkit.EndObj):
		default:
			return nil, fmt.Errorf("syntax error: expected ',' or '}")
		}
	}
	if !c.is(jsonkit.EndObj) {
		return nil, fmt.Errorf("syntax error: expected '}")
	}
	c.next()
	return b, nil
}

func (c *compiler) compileWildcard() (Expr, error) {
	defer c.next()
	return wildcard{}, nil
}

func (c *compiler) compileDescent() (Expr, error) {
	defer c.next()
	return descent{}, nil
}

func (c *compiler) compileIdent() (Expr, error) {
	i := identifier{
		ident: c.getString(),
	}
	return i, nil
}

func (c *compiler) compileNumber() (Expr, error) {
	i := literal[float64]{
		value: c.getNumber(),
	}
	return i, nil
}

func (c *compiler) compileString() (Expr, error) {
	i := literal[string]{
		value: c.getString(),
	}
	return i, nil
}

func (c *compiler) compileBool() (Expr, error) {
	i := literal[bool]{
		value: c.getBool(),
	}
	return i, nil
}

func (c *compiler) compileGroup() (Expr, error) {
	c.next()
	expr, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	if !c.is(jsonkit.EndGrp) {
		return nil, fmt.Errorf("syntax error: missing ')'")
	}
	c.next()
	return expr, nil
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
	return r, nil
}

func (c *compiler) compileTernary(left Expr) (Expr, error) {
	c.next()
	t := ternary{
		cdt: left,
	}
	csq, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	if !c.is(jsonkit.Colon) {
		return nil, fmt.Errorf("syntax error: missing ':'")
	}
	c.next()
	alt, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	t.csq = csq
	t.alt = alt
	return t, nil
}

func (c *compiler) compileBinary(left Expr) (Expr, error) {
	if c.is(jsonkit.Wildcard) {
		c.curr.Type = jsonkit.Mul
	} else if c.is(jsonkit.Parent) {
		c.curr.Type = jsonkit.Mod
	}
	var (
		pow = bindings[c.curr.Type]
		err error
	)
	bin := binary{
		left: left,
		op:   c.curr.Type,
	}
	c.next()
	bin.right, err = c.compileExpr(pow)
	return bin, err
}

func (c *compiler) compileCall(left Expr) (Expr, error) {
	ident, ok := left.(identifier)
	if !ok {
		return nil, fmt.Errorf("syntax error: identifier expected")
	}
	expr := call{
		ident: ident.ident,
	}
	c.next()
	for !c.done() && !c.is(jsonkit.EndGrp) {
		a, err := c.compileExpr(powLowest)
		if err != nil {
			return nil, err
		}
		expr.args = append(expr.args, a)
		switch {
		case c.is(jsonkit.Comma):
			c.next()
			if c.is(jsonkit.EndGrp) {
				return nil, fmt.Errorf("syntax error: trailing comma")
			}
		case c.is(jsonkit.EndGrp):
		default:
			return nil, fmt.Errorf("syntax error: unexpected token")
		}
	}
	if !c.is(jsonkit.EndGrp) {
		return nil, fmt.Errorf("syntax error: missing ')'")
	}
	c.next()
	return expr, nil
}

func (c *compiler) done() bool {
	return c.is(jsonkit.EOF)
}

func (c *compiler) is(kind rune) bool {
	return c.curr.Type == kind
}

func (c *compiler) next() {
	c.curr = c.peek
	c.peek = c.scan.Scan()
}

type queryMode int8

const (
	pathMode queryMode = 1 << iota
	filterMode
)

type QueryScanner struct {
	input *bufio.Reader
	char  rune

	mode queryMode

	str bytes.Buffer
}

func ScanQuery(r io.Reader) *QueryScanner {
	scan := QueryScanner{
		input: bufio.NewReader(r),
		mode:  pathMode,
	}
	scan.read()
	return &scan
}

func (s *QueryScanner) Scan() jsonkit.Token {
	defer s.str.Reset()
	s.skipBlank()

	var tok jsonkit.Token
	if s.done() {
		tok.Type = jsonkit.EOF
		return tok
	}
	switch {
	case jsonkit.IsLetter(s.char):
		s.scanIdent(&tok)
	case jsonkit.IsBackQuote(s.char):
		s.scanQuotedIdent(&tok)
	case jsonkit.IsNumber(s.char):
		s.scanNumber(&tok)
	case jsonkit.IsQuote(s.char):
		s.scanString(&tok)
	case jsonkit.IsDelim(s.char) || s.char == '(' || s.char == ')':
		s.scanDelimiter(&tok)
	case jsonkit.IsOperator(s.char):
		s.scanOperator(&tok)
	case jsonkit.IsDollar(s.char):
		s.scanDollar(&tok)
	case jsonkit.IsTransform(s.char):
		s.scanTransform(&tok)
	default:
		tok.Type = jsonkit.Invalid
	}
	s.setMode(tok)
	return tok
}

func (s *QueryScanner) setMode(tok jsonkit.Token) {
	if tok.Type == jsonkit.BegArr {
		s.mode = filterMode
	} else if tok.Type == jsonkit.EndArr {
		s.mode = pathMode
	}
}

func (s *QueryScanner) scanQuotedIdent(tok *jsonkit.Token) {
	for !s.done() && !jsonkit.IsBackQuote(s.char) {
		s.write()
		s.read()
	}
	tok.Type = jsonkit.Ident
	tok.Literal = s.str.String()
	if !jsonkit.IsBackQuote(s.char) {
		tok.Type = jsonkit.Invalid
	} else {
		s.read()
	}
}

func (s *QueryScanner) scanTransform(tok *jsonkit.Token) {
	s.read()
	tok.Type = jsonkit.Transform
}

func (s *QueryScanner) scanDollar(tok *jsonkit.Token) {
	s.read()
	if !jsonkit.IsLetter(s.char) {
		tok.Type = jsonkit.Invalid
		return
	}
	s.scanIdent(tok)
	if tok.Type == jsonkit.Ident {
		tok.Type = jsonkit.Func
	}
}

func (s *QueryScanner) scanIdent(tok *jsonkit.Token) {
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
	case "and":
		tok.Type = jsonkit.And
	case "or":
		tok.Type = jsonkit.Or
	case "in":
		tok.Type = jsonkit.In
	default:
		tok.Type = jsonkit.Ident
	}
}

func (s *QueryScanner) scanString(tok *jsonkit.Token) {
	s.read()
	for !s.done() && s.char != '"' {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	tok.Type = jsonkit.String
	if s.char != '"' {
		tok.Type = jsonkit.Invalid
	} else {
		s.read()
	}
}

func (s *QueryScanner) scanNumber(tok *jsonkit.Token) {
	tok.Type = jsonkit.Number
	for !s.done() && jsonkit.IsNumber(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	if s.char == '.' {
		s.write()
		s.read()
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

func (s *QueryScanner) scanOperator(tok *jsonkit.Token) {
	switch s.char {
	case '+':
		tok.Type = jsonkit.Add
	case '-':
		tok.Type = jsonkit.Sub
	case '*':
		if s.mode == pathMode {
			tok.Type = jsonkit.Wildcard
			if k := s.peek(); k == s.char {
				s.read()
				tok.Type = jsonkit.Descent
			}
		} else {
			tok.Type = jsonkit.Mul
		}
	case '/':
		tok.Type = jsonkit.Div
	case '%':
		if s.mode == pathMode {
			tok.Type = jsonkit.Parent
		} else {
			tok.Type = jsonkit.Mod
		}
	case '?':
		tok.Type = jsonkit.Ternary
	case ':':
	case '!':
		tok.Type = jsonkit.Invalid
		if k := s.peek(); k == '=' {
			s.read()
			tok.Type = jsonkit.Ne
		}
	case '=':
		tok.Type = jsonkit.Eq
	case '<':
		tok.Type = jsonkit.Lt
		if k := s.peek(); k == '=' {
			s.read()
			tok.Type = jsonkit.Le
		}
	case '>':
		tok.Type = jsonkit.Gt
		if k := s.peek(); k == '=' {
			s.read()
			tok.Type = jsonkit.Ge
		}
	case '.':
		tok.Type = jsonkit.Map
		if k := s.peek(); k == s.char {
			s.read()
			tok.Type = jsonkit.Range
		}
	case '&':
		tok.Type = jsonkit.Concat
	default:
		tok.Type = jsonkit.Invalid
	}
	if tok.Type != jsonkit.Invalid {
		s.read()
	}
}

func (s *QueryScanner) scanDelimiter(tok *jsonkit.Token) {
	switch s.char {
	case '(':
		tok.Type = jsonkit.BegGrp
	case ')':
		tok.Type = jsonkit.EndGrp
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

func (s *QueryScanner) write() {
	s.str.WriteRune(s.char)
}

func (s *QueryScanner) read() {
	char, _, err := s.input.ReadRune()
	if errors.Is(err, io.EOF) {
		char = utf8.RuneError
	}
	s.char = char
}

func (s *QueryScanner) peek() rune {
	defer s.input.UnreadRune()
	r, _, _ := s.input.ReadRune()
	return r
}

func (s *QueryScanner) done() bool {
	return s.char == utf8.RuneError
}

func (s *QueryScanner) skipBlank() {
	for !s.done() && unicode.IsSpace(s.char) {
		s.read()
	}
}
