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

type Path struct {
	Relative bool
	Steps    []Step
}

type Step struct {
	Field string
	Cast  string
}

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
	case c.is(Dot):
		p.Relative = true
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
	}
	if c.is(Select) {
		c.next()
	}
	return step, nil
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
	Dot
	Root
	Cast
	Select
	BegGrp
	EndGrap
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

	var tok token
	if s.done() {
		tok.Type = Eof
		return tok
	}
	switch {
	case s.char == '.':
		tok.Type = Dot
		s.read()
	case s.char == '$':
		tok.Type = Root
		s.read()
	default:
		s.scanIdent(&tok)
	}
	return tok
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
