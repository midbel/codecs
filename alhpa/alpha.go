package alpha

import (
	"io"
	"strings"
	"unicode/utf8"
)

type Namer interface {
	Next() (string, error)
	Reset()
}

const (
	lowerA  = 'a'
	lowerZ  = 'z'
	upperA  = 'A'
	upperZ  = 'Z'
	number0 = '0'
	number9 = '9'
)

type Char struct {
	step int
	curr rune
	min  rune
	max  rune
}

func Create(min, max rune, step int) *Char {
	return &Char{
		step: step,
		curr: min,
		min:  min,
		max:  max,
	}
}

func Lower() *Char {
	return Create(lowerA, lowerZ, 1)
}

func Upper() *Char {
	return Create(upperA, upperZ, 1)
}

func Number() *Char {
	return Create(number0, number9, 1)
}

func (c *Char) Get() rune {
	return c.curr
}

func (c *Char) Next() rune {
	if c.Done() {
		return c.Get()
	}
	c.curr += rune(c.step)
	if c.curr > c.max {
		c.curr = utf8.RuneError
	}
	return c.curr
}

func (c *Char) Done() bool {
	return c.curr == utf8.RuneError
}

func (c *Char) Reset() {
	c.curr = c.min
}

type chain struct {
	list []*Char
}

func NewLowerString(size int) Namer {
	var c chain
	if size <= 0 {
		return &c
	}
	for i := 0; i < size; i++ {
		c.list = append(c.list, Lower())
	}
	return &c
}

func NewUpperString(size int) Namer {
	var c chain
	if size <= 0 {
		return &c
	}
	for i := 0; i < size; i++ {
		c.list = append(c.list, Upper())
	}
	return &c
}

func NewNumberString(size int) Namer {
	var c chain
	if size <= 0 {
		return &c
	}
	for i := 0; i < size; i++ {
		c.list = append(c.list, Number())
	}
	return &c
}

func (c *chain) Next() (string, error) {
	if len(c.list) == 0 || c.list[0].Done() {
		return "", io.EOF
	}
	return c.next()
}

func (c *chain) Reset() {
	for i := range c.list {
		c.list[i].Reset()
	}
}

func (c *chain) next() (string, error) {
	var chars []rune
	for _, a := range c.list {
		chars = append(chars, a.Get())
	}
	for i := len(c.list) - 1; i >= 0; i-- {
		c.list[i].Next()
		if !c.list[i].Done() {
			for j := i + 1; j < len(c.list); j++ {
				c.list[j].Reset()
			}
			break
		}
	}
	return string(chars), nil
}

type compose struct {
	list []String
	buf  []string
	sep  string
}

func Compose(part ...Namer) Namer {
	var c compose
	c.list = append(c.list, str...)
	c.sep = "-"
	for i := range c.list {
		str, _ := c.list[i].Next()
		c.buf = append(c.buf, str)
	}
	return &c
}

func (c *compose) Next() (string, error) {
	str := strings.Join(c.buf, c.sep)
	return str, c.next()
}

func (c *compose) next() error {
	var done bool
	for i := len(c.list) - 1; i >= 0; i-- {
		str, err := c.list[i].Next()
		if err == nil {
			c.buf[i] = str
			return nil
		}
		if errors.Is(err, io.EOF) {
			if i == 0 {
				done = true
			}
			for j := i; j < len(c.list); j++ {
				c.list[j].Reset()
			}
			str, _ = c.list[i].Next()
			c.buf[i] = str
			continue
		}
	}
	if done {
		return io.EOF
	}
	return nil
}

func (c *compose) Reset() {
	for i := range c.list {
		c.list[i].Reset()
	}
}
