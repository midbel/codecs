package traversal

import (
	"bytes"
	"errors"
	"fmt"
	"unicode/utf8"
)

var (
	ErrType = errors.New("array or object expected")
	ErrEnd  = errors.New("unexpected end of path")
	ErrProp = errors.New("property not found")
)

func Collect(in any, paths []string) ([]any, error) {
	st := make([]Step, 0, len(paths))
	for _, p := range paths {
		s := Step{
			Field: p,
		}
		st = append(st, s)
	}
	return traverse(in, st)
}

type Path struct {
	Anchored bool
	Steps    []Step
}

func (p Path) Collect(in any) ([]any, error) {
	return traverse(in, p.Steps)
}

type Step struct {
	Field string
	Cast  string
}

func traverse(in any, paths []Step) ([]any, error) {
	switch in := in.(type) {
	case []any:
		return traverseArray(in, paths)
	case map[string]any:
		return traverseObject(in, paths)
	default:
		return nil, ErrType
	}
}

func traverseArray(in []any, paths []Step) ([]any, error) {
	var result []any
	for i := range in {
		tmp, err := traverse(in[i], paths)
		if err != nil {
			return nil, err
		}
		result = append(result, tmp...)
	}
	return result, nil
}

func traverseObject(in map[string]any, paths []Step) ([]any, error) {
	if len(paths) == 0 {
		return nil, ErrEnd
	}
	p, ok := in[paths[0].Field]
	if !ok {
		return nil, fmt.Errorf("%s: %w", paths[0].Field, ErrProp)
	}
	if len(paths) == 1 {
		return []any{p}, nil
	}
	return traverse(p, paths[1:])
}

var errSyntax = errors.New("syntax error")

func syntaxError(msg string) error {
	return fmt.Errorf("%w: %s", errSyntax, msg)
}

type compiler struct {
	scan *scanner
	curr token
	peek token
}

func compile(str string) *compiler {
	c := compiler{
		scan: createScanner(str),
	}
	c.next()
	c.next()
	return &c
}

func (c *compiler) Compile() (Path, error) {
	var (
		p   Path
		err error
	)
	switch {
	case c.is(Root):
		c.next()
		if !c.is(Dot) {
			return p, syntaxError("'.' expected after '$'!")
		}
		p.Anchored = true
	case c.is(Dot):
		c.next()
	default:
		return p, syntaxError("path should start with '$'' or '.'!")
	}
	p.Steps, err = c.compile()
	return p, err
}

func (c *compiler) compile() ([]Step, error) {
	var steps []Step
	for !c.done() {
		step, err := c.compileStep()
		if err != nil {
			return nil, err
		}
		switch {
		case c.is(Dot):
			c.next()
			if c.is(Eof) {
				return nil, syntaxError("'.' not allowed at end of path")
			}
		case c.is(Eof):
		default:
			return nil, syntaxError("'.' expected after step")
		}
		steps = append(steps, step)
	}
	return steps, nil
}

func (c *compiler) compileStep() (Step, error) {
	var step Step
	if !c.is(Ident) {
		return step, syntaxError("identifier expected")
	}
	step.Field = c.currentLiteral()
	if c.is(Cast) {
		c.next()
		if !c.is(Ident) {
			return step, syntaxError("cast type expected")
		}
		step.Cast = c.currentLiteral()
		c.next()
	}
	if c.is(Select) {
		c.next()
		err := c.compileSelector()
		if err != nil {
			return step, err
		}
	}
	return step, nil
}

func (c *compiler) compileValue() error {
	switch {
	case c.is(String):
	case c.is(Number):
	case c.is(Boolean):
	case c.is(Dot):
	default:
		return syntaxError("primitive value of relative path expected")
	}
	return nil
}

func (c *compiler) compileSelector() error {
	c.next()
	if !c.is(Ident) {
		return syntaxError("selector name expected")
	}
	c.next()
	if !c.is(BegGrp) {
		return syntaxError("expected '(' at beginning of selector")
	}
	c.next()
	for !c.done() && !c.is(EndGrp) {
		err := c.compileValue()
		if err != nil {
			return err
		}
		switch {
		case c.is(Comma):
			c.next()
			if c.is(EndGrp) {
				return syntaxError("')' is not allowed after ','")
			}
		case c.is(EndGrp):
		default:
			return syntaxError("',' or ')' after selector argument")
		}
	}
	if !c.is(BegGrp) {
		return syntaxError("expected ')' at end of selector")
	}
	c.next()
	return nil
}

func (c *compiler) next() {
	c.curr = c.peek
	c.peek = c.scan.Scan()
}

func (c *compiler) done() bool {
	return c.is(Eof)
}

func (c *compiler) is(kind rune) bool {
	return c.curr.Type == kind
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
	Dot
	Root
	Cast
	Select
	Comma
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

	allowedValues int

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

	if s.allowedValues >= 1 {
		tok := s.scanValue()
		if tok.Type != Invalid {
			return tok
		}
	}
	return s.scanDefault()
}

func (s *scanner) scanValue() token {
	var tok token
	switch {
	case s.char == '"':
		s.scanString(&tok)
	case isNumber(s.char) || s.char == '-':
		s.scanNumber(&tok)
	case isLetter(s.char):
		s.scanBool(&tok)
	default:
		tok.Type = Invalid
	}
	return tok
}

func (s *scanner) scanDefault() token {
	var tok token
	if s.done() {
		tok.Type = Eof
		return tok
	}
	switch {
	case s.char == '.':
		tok.Type = Dot
		s.read()
	case s.char == ',':
		tok.Type = Comma
		s.read()
	case s.char == '(':
		tok.Type = Comma
		s.read()
	case s.char == ')':
		tok.Type = Comma
		s.read()
	case s.char == '$':
		tok.Type = Root
		s.read()
	case s.char == ':':
		tok.Type = Select
		s.read()
		if s.char == ':' {
			tok.Type = Cast
			s.read()
		}
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
	for !s.done() && !isNumber(s.char) {
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
	for !s.done() && !isNumber(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.literal()
}

func (s *scanner) scanBool(tok *token) {
	tok.Type = Boolean
	for !s.done() && isAlpha(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.literal()
	switch tok.Literal {
	case "true", "false":
	default:
		tok.Type = Invalid
	}
}

func (s *scanner) scanIdent(tok *token) {
	for !s.done() && isAlpha(s.char) {
		s.write()
		s.read()
	}
	tok.Type = Ident
	tok.Literal = s.literal()
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
