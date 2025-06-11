package main

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

type String interface {
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

func LowerString(size int) String {
	var c chain
	if size <= 0 {
		return &c
	}
	for i := 0; i < size; i++ {
		c.list = append(c.list, Lower())
	}
	return &c
}

func NumberString(size int) String {
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

type Compose struct {
	list []String
	buf  []string
	sep  string
}

func ComposeString(str ...String) String {
	var c Compose
	c.list = append(c.list, str...)
	c.sep = "-"
	for i := range c.list {
		str, _ := c.list[i].Next()
		c.buf = append(c.buf, str)
	}
	return &c
}

func (c *Compose) Next() (string, error) {
	str := strings.Join(c.buf, c.sep)
	return str, c.next()
}

func (c *Compose) next() error {
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

func (c *Compose) Reset() {
	for i := range c.list {
		c.list[i].Reset()
	}
}

type Direction int

const (
	Forward Direction = 1 << iota
	Reverse
)

type Settings struct {
	Size   int
	Sep    string
	Prefix string
	Suffix string
	Dir    Direction
}

func main() {
	c := LowerString(2)

	x := NumberString(2)

	c.Reset()
	x.Reset()
	n := ComposeString(c, x)
	for {
		str, err := n.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		fmt.Println(">>>", str)
	}
}
