package relax

import (
	"github.com/midbel/codecs/xml"
)

type Arity int8

const (
	ZeroOrMore Arity = 1 << iota
	ZeroOrOne
	OneOrMore
)

type Pattern interface {
	// Validate(xml.Node) error
}

type Grammar struct {
	Start Pattern
	List  map[string]Pattern
}

func (g Grammar) Validate(root xml.Node) error {
	return nil
}

type QName struct {
	Space string
	Local string
}

type Link struct {
	Ident string
	Arity
}

type Attribute struct {
	QName
	Arity
	Value Pattern
}

func (a Attribute) Validate(node xml.Node) error {
	return nil
}

type Element struct {
	QName
	Arity
	Value    Pattern
	Patterns []Pattern
}

func (e Element) Validate(node xml.Node) error {
	return nil
}

type Text struct{}

func (_ Text) Validate(node xml.Node) error {
	return nil
}

type Empty struct{}

func (_ Empty) Validate(node xml.Node) error {
	return nil
}

type Enum struct {
	List []string
}
