package relax

import (
	"fmt"
	"time"

	"github.com/midbel/codecs/xml"
)

type Arity int8

const (
	ZeroOrMore Arity = 1 << iota
	ZeroOrOne
	OneOrMore
	One
)

func (a Arity) Zero() bool {
	return a == ZeroOrMore || a == ZeroOrOne
}

func (a Arity) One() bool {
	return a == 0 || a == ZeroOrOne || a == OneOrMore || a == One
}

func (a Arity) More() bool {
	return a == ZeroOrMore || a == OneOrMore
}

type Pattern interface {
	Validate(xml.Node) error
}

type QName struct {
	Space string
	Local string
}

func (q QName) QualifiedName() string {
	if q.Space == "" {
		return q.Local
	}
	return fmt.Sprintf("%s:%s", q.Space, q.Local)
}

func (q QName) LocalName() string {
	return q.Local
}

type Link struct {
	Ident string
	Arity
	Pattern
}

type Attribute struct {
	QName
	Arity
	Value Pattern
}

func (a Attribute) Validate(node xml.Node) error {
	return nil
}

type Group struct {
	List []Pattern
}

func (g Group) Validate(node xml.Node) error {
	return nil
}

type Choice struct {
	List []Pattern
}

func (c Choice) Validate(node xml.Node) error {
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

type Type struct {
	Name    string
	Pattern string
}

func (t Type) Validate(node xml.Node) error {
	return nil
}

type StringType struct {
	Type
	MinLength int
	MaxLength int
}

type IntType struct {
	Type
	MinValue int
	MaxValue int
}

type FloatType struct {
	Type
	MinValue int
	MaxValue int
}

type TimeType struct {
	Type
	MinValue time.Time
	MaxValue time.Time
}

type Enum struct {
	List []string
}

func (e Enum) Validate(node xml.Node) error {
	return nil
}

func reassemble(start Pattern, others map[string]Pattern) (Pattern, error) {
	switch el := start.(type) {
	case Element:
		return el, nil
	case Attribute:
		return el, nil
	case Choice:
		for i := range el.List {
			tmp, err := reassemble(el.List[i], others)
			if err != nil {
				return nil, err
			}
			el.List[i] = tmp
		}
		return el, nil
	case Group:
		for i := range el.List {
			tmp, err := reassemble(el.List[i], others)
			if err != nil {
				return nil, err
			}
			el.List[i] = tmp
		}
		return el, nil
	case Link:
	default:
		return nil, fmt.Errorf("unsupported pattern")
	}
	link, ok := start.(Link)
	if !ok {
		return start, nil
	}
	el, ok := others[link.Ident].(Element)
	if !ok {
		return nil, fmt.Errorf("%s: pattern not defined", link.Ident)
	}
	if el.Arity == 0 {
		el.Arity = link.Arity
	}
	for i := range el.Patterns {
		p, err := reassemble(el.Patterns[i], others)
		if err != nil {
			return nil, err
		}
		el.Patterns[i] = p
	}
	return el, nil
}
