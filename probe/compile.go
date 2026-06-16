package probe

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"unicode/utf8"
)

var (
	errSyntax  = errors.New("syntax error")
	errConvert = errors.New("conversion error")
)

func syntaxError(msg string) error {
	return fmt.Errorf("%w: %s", errSyntax, msg)
}

func convertNumberError(str string) error {
	return fmt.Errorf("%w: %s can not be converted to number", errConvert, str)
}

func convertBoolError(str string) error {
	return fmt.Errorf("%w: %s can not be converted to boolean", errConvert, str)
}

type compiler struct {
	scan *scanner
	curr token
	peek token
}

func CompilePath(str string) (*Query, error) {
	c := compile(str)
	return c.Compile()
}

func compile(str string) *compiler {
	c := compiler{
		scan: createScanner(str),
	}
	c.next()
	c.next()
	return &c
}

func (c *compiler) Compile() (*Query, error) {
	if c.is(BegGrp) {
		ps, err := c.compileGroups()
		if err != nil {
			return nil, err
		}
		q := &Query{
			paths: ps,
		}
		return q, nil
	}
	p, err := c.compileRootedPath(Eof)
	if err != nil {
		return nil, err
	}
	q := &Query{
		paths: []Path{p},
	}
	return q, nil
}

func (c *compiler) compileRootedPath(stop rune) (Path, error) {
	base, err := c.compilePath(Arrow, stop)
	if err != nil {
		return nil, err
	}
	if !c.is(Arrow) {
		return base, nil
	}
	c.next()
	if c.is(BegGrp) {
		ps, err := c.compileGroups()
		if err != nil {
			return nil, err
		}
		rs := root{
			base: base,
			next: ps,
		}
		return rs, nil
	}
	next, err := c.compilePath(stop)
	if err != nil {
		return nil, err
	}
	base = root{
		base: base,
		next: []Path{next},
	}
	return base, nil
}

func (c *compiler) compileGroups() ([]Path, error) {
	var list []Path
	for !c.done() {
		c.next()
		p, err := c.compileRootedPath(EndGrp)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
		if !c.at(EndGrp) {
			return nil, syntaxError("')' expected at end of query")
		}
		c.next()
		switch {
		case c.is(Comma):
			c.next()
			if !c.is(BegGrp) {
				return nil, syntaxError("'(' expected at begining of path")
			}
		case c.done():
		default:
			return nil, syntaxError("',' expected after query")
		}
	}
	return list, nil
}

func (c *compiler) compilePath(stop ...rune) (Path, error) {
	var paths []Path
	for !c.at(stop...) {
		pth, err := c.compileAlternative()
		if err != nil {
			return nil, err
		}
		switch {
		case c.at(stop...):
		case c.is(Comma):
			c.next()
			if c.at(stop...) {
				return nil, syntaxError("',' not allowed at end of path")
			}
		default:
			return nil, syntaxError("',' expected after path")
		}
		paths = append(paths, pth)

	}
	switch len(paths) {
	case 0:
		return nil, syntaxError("no path could be parsed from input string")
	case 1:
		return paths[0], nil
	default:
		mp := multi{
			paths: paths,
		}
		return mp, nil
	}
}

func (c *compiler) compileAlternative() (Path, error) {
	var paths []Path
	for !c.done() {
		var (
			pth Path
			err error
		)
		if c.isLiteral() {
			pth, err = c.compileValue()
		} else {
			pth, err = c.compilePart()
		}
		if err != nil {
			return nil, err
		}
		paths = append(paths, pth)
		if !c.is(Pipe) {
			break
		}
		c.next()
	}
	switch len(paths) {
	case 0:
		return nil, syntaxError("no path could be parsed from input string")
	case 1:
		return paths[0], nil
	default:
		mp := alternative{
			paths: paths,
		}
		return mp, nil
	}
}

func (c *compiler) compileValue() (Path, error) {
	var value any
	switch {
	case c.is(Null):
	case c.is(String):
		value = c.currentLiteral()
	case c.is(Number):
		x, err := strconv.ParseFloat(c.currentLiteral(), 64)
		if err != nil {
			return nil, convertNumberError(c.currentLiteral())
		}
		value = x
	case c.is(Boolean):
		x, err := strconv.ParseBool(c.currentLiteral())
		if err != nil {
			return nil, convertBoolError(c.currentLiteral())
		}
		value = x
	default:
		return nil, syntaxError("literal value expected")
	}
	c.next()
	lit := literal{
		value: value,
	}
	return lit, nil
}

func (c *compiler) compilePart() (Path, error) {
	var (
		pth single
		err error
	)
	pth.Anchored, err = c.compileAnchored()
	if err != nil {
		return nil, err
	}
	pth.Start, err = c.compileExpr()
	if err != nil {
		return nil, err
	}
	return pth, nil
}

func (c *compiler) compileAnchored() (bool, error) {
	var anchored bool
	switch {
	case c.is(Root):
		c.next()
		if !c.is(Dot) {
			return false, syntaxError("'.' expected after '$'!")
		}
		anchored = true
		c.next()
	case c.is(Dot):
		c.next()
	default:
		return false, syntaxError("path should start with '$'' or '.'!")
	}
	return anchored, nil
}

func (c *compiler) compileExpr() (Expr, error) {
	var (
		step field
		err  error
	)
	if !c.isIdentifier() {
		return nil, syntaxError("identifier expected")
	}
	step.Name = c.currentLiteral()
	c.next()
	if c.is(Call) {
		step.Apply, err = c.compileCall()
		if err != nil {
			return nil, err
		}
	}
	switch {
	case c.is(Dot):
		c.next()
		step.Next, err = c.compileExpr()
		if err != nil {
			return nil, err
		}
	case c.is(Pipe):
		if !c.aheadLiteral() {
			break
		}
		c.next()
		v, err := c.compileValue()
		if err != nil {
			return nil, err
		}
		if !c.is(Comma) && !c.is(Pipe) && !c.is(Eof) && !c.is(EndGrp) {
			return nil, syntaxError("alternative value only allow at end of path")
		}
		step.Alt = v.(Expr)
	default:
	}
	return step, nil
}

func (c *compiler) compileCall() (Expr, error) {
	c.next()
	if !c.is(Ident) {
		return nil, syntaxError("selector name expected")
	}
	apply := call{
		Ident: c.currentLiteral(),
	}
	c.next()
	if !c.is(BegGrp) {
		return nil, syntaxError("expected '(' at beginning of selector")
	}
	c.next()
	for !c.done() && !c.is(EndGrp) {
		v, err := c.compileValue()
		if err != nil {
			return nil, err
		}
		apply.Args = append(apply.Args, v.(Expr))
		switch {
		case c.is(Comma):
			c.next()
			if c.is(EndGrp) {
				return nil, syntaxError("')' is not allowed after ','")
			}
		case c.is(EndGrp):
		default:
			return nil, syntaxError("',' or ')' after selector argument")
		}
	}
	if !c.is(EndGrp) {
		return nil, syntaxError("expected ')' at end of selector")
	}
	c.next()
	return apply, nil
}

func (c *compiler) next() {
	c.curr = c.peek
	c.peek = c.scan.Scan()
}

func (c *compiler) done() bool {
	return c.is(Eof)
}

func (c *compiler) at(stop ...rune) bool {
	// return c.done() || c.is(stop)
	if c.done() {
		return true
	}
	for i := range stop {
		if c.is(stop[i]) {
			return true
		}
	}
	return false
}

func (c *compiler) is(kind rune) bool {
	return c.curr.Type == kind
}

func (c *compiler) isLiteral() bool {
	return c.is(String) || c.is(Number) || c.is(Boolean) || c.is(Null)
}

func (c *compiler) aheadLiteral() bool {
	return c.aheadIs(String) || c.aheadIs(Number) || c.aheadIs(Boolean) || c.is(Null)
}

func (c *compiler) aheadIs(kind rune) bool {
	return c.peek.Type == kind
}

func (c *compiler) isIdentifier() bool {
	return c.is(Ident) || c.is(String) || c.is(Number)
}

func (c *compiler) currentLiteral() string {
	return c.curr.Literal
}

const (
	Invalid rune = iota
	Ident
	Number
	String
	Boolean
	Null
	Dot
	Deep
	Arrow
	Root
	Call
	Comma
	Pipe
	BegGrp
	EndGrp
	Eof
)

type token struct {
	Literal string
	Type    rune
}

type scanner struct {
	input []byte
	curr  int
	next  int
	char  rune

	buf bytes.Buffer
}

func createScanner(str string) *scanner {
	s := scanner{
		input: []byte(str),
	}
	s.read()
	return &s
}

func (s *scanner) Scan() token {
	defer s.reset()
	s.skipBlanks()

	return s.scanDefault()
}

func (s *scanner) scanDefault() token {
	var tok token
	if s.done() {
		tok.Type = Eof
		return tok
	}
	s.skipBlanks()
	switch {
	case s.char == '=':
		s.read()
		if s.char == '>' {
			tok.Type = Arrow
			s.read()
		}
	case s.char == '.':
		tok.Type = Dot
		s.read()
		if s.char == '.' {
			tok.Type = Deep
			s.read()
		}
	case s.char == ',':
		tok.Type = Comma
		s.read()
	case s.char == '(':
		tok.Type = BegGrp
		s.read()
	case s.char == ')':
		tok.Type = EndGrp
		s.read()
	case s.char == '$':
		tok.Type = Root
		s.read()
	case s.char == ':':
		tok.Type = Call
		s.read()
	case s.char == '|':
		tok.Type = Pipe
		s.read()
	case s.char == '"':
		s.scanString(&tok)
	case isNumber(s.char) || s.char == '-':
		s.scanNumber(&tok)
	default:
		s.scanIdent(&tok)
	}
	return tok
}

func (s *scanner) scanString(tok *token) {
	s.read()
	for !s.done() && s.char != '"' {
		s.write()
		s.read()
	}
	tok.Type = String
	tok.Literal = s.literal()
	if s.char != '"' {
		tok.Type = Invalid
	} else {
		s.read()
	}
}

func (s *scanner) scanNumber(tok *token) {
	if s.char == '-' {
		s.write()
		s.read()
	}
	for !s.done() && isNumber(s.char) {
		s.write()
		s.read()
	}
	tok.Type = Number
	tok.Literal = s.literal()
	if s.char != '.' {
		return
	}
	s.write()
	s.read()
	for !s.done() && isNumber(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.literal()
}

func (s *scanner) scanIdent(tok *token) {
	for !s.done() && isAlpha(s.char) {
		s.write()
		s.read()
	}
	tok.Type = Ident
	tok.Literal = s.literal()
	switch tok.Literal {
	case "true", "false":
		tok.Type = Boolean
	case "null":
		tok.Type = Null
	default:
	}
}

func (s *scanner) done() bool {
	return s.char == utf8.RuneError || s.curr >= len(s.input)
}

func (s *scanner) read() {
	if s.char != utf8.RuneError && s.next >= len(s.input) {
		s.char = utf8.RuneError
		return
	}
	c, z := utf8.DecodeRune(s.input[s.next:])
	s.curr = s.next
	s.next = s.next + z
	s.char = c
}

func (s *scanner) write() {
	s.buf.WriteRune(s.char)
}

func (s *scanner) reset() {
	s.buf.Reset()
}

func (s *scanner) skipBlanks() {
	for !s.done() && isBlank(s.char) {
		s.read()
	}
}

func (s *scanner) literal() string {
	return s.buf.String()
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

func isBlank(c rune) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}
