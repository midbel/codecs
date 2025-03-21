package relax

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/midbel/codecs/xml"
)

var (
	ErrRange  = errors.New("value out of range")
	ErrLength = errors.New("invalid length")
	ErrFormat = errors.New("invalid format")
)

type NodeError struct {
	Node  xml.Node
	Cause string
}

func createError(cause string, node xml.Node) error {
	return NodeError{
		Node:  node,
		Cause: cause,
	}
}

func (n NodeError) Error() string {
	return n.Cause
}

type cardinality int8

const (
	zeroOrMore cardinality = 1 << iota
	zeroOrOne
	oneOrMore
	one
)

func (a cardinality) Zero() bool {
	return a == zeroOrMore || a == zeroOrOne
}

func (a cardinality) One() bool {
	return a == 0 || a == zeroOrOne || a == oneOrMore || a == one
}

func (a cardinality) More() bool {
	return a == zeroOrMore || a == oneOrMore
}

type Pattern interface {
	Validate(xml.Node) error
	validate(xml.Node, Resolver) error
}

type Resolver interface {
	Resolve(Pattern) (Pattern, error)
}

type emptyResolver struct{}

func (_ emptyResolver) Resolve(p Pattern) (Pattern, error) {
	return p, nil
}

var noopResolver emptyResolver

type Grammar struct {
	Links map[string]Pattern
	Start Pattern
}

func (g Grammar) Validate(node xml.Node) error {
	return g.validate(node, g)
}

func (g Grammar) validate(node xml.Node, ctx Resolver) error {
	p, err := ctx.Resolve(g.Start)
	if err != nil {
		return err
	}
	return p.validate(node, ctx)
}

func (g Grammar) Resolve(pattern Pattern) (Pattern, error) {
	switch k := pattern.(type) {
	case Link:
		p, ok := g.Links[k.Ident]
		if !ok {
			return nil, fmt.Errorf("%s: undefined pattern", k.Ident)
		}
		return p, nil
	default:
		return pattern, nil
	}
}

func Valid() Pattern {
	return valid{}
}

type valid struct{}

func (_ valid) Validate(_ xml.Node) error { return nil }

func (_ valid) validate(_ xml.Node, _ Resolver) error { return nil }

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
	cardinality
}

func (k Link) Validate(node xml.Node) error {
	return k.validate(node, noopResolver)
}

func (k Link) validate(node xml.Node, ctx Resolver) error {
	p, err := ctx.Resolve(k)
	if err != nil {
		return err
	}
	return p.validate(node, ctx)
}

type Attribute struct {
	QName
	cardinality
	Value Pattern
}

func (a Attribute) Validate(node xml.Node) error {
	return a.validate(node, noopResolver)
}

func (a Attribute) validate(node xml.Node, _ Resolver) error {
	el, ok := node.(*xml.Element)
	if !ok {
		return createError("xml element expected", node)
	}
	ix := slices.IndexFunc(el.Attrs, func(attr xml.Attribute) bool {
		return a.QualifiedName() == attr.QualifiedName()
	})
	if ix < 0 && !a.Zero() {
		msg := fmt.Sprintf("%s: attribute is missing", a.QualifiedName())
		return createError(msg, node)
	}
	if a.Value == nil {
		return nil
	}
	v, ok := a.Value.(interface{ validateValue(string) error })
	if !ok {
		return fmt.Errorf("pattern not applicatble for attribute")
	}
	return v.validateValue(el.Attrs[ix].Value())
}

type Group struct {
	List []Pattern
}

func (_ Group) Validate(_ xml.Node) error             { return nil }
func (_ Group) validate(_ xml.Node, _ Resolver) error { return nil }

type Choice struct {
	List []Pattern
}

func (c Choice) Validate(node xml.Node) error {
	_, err := validateChoice([]xml.Node{node}, c, noopResolver)
	return err
}

func (c Choice) validate(node xml.Node, ctx Resolver) error {
	_, err := validateChoice([]xml.Node{node}, c, ctx)
	return err
}

type Element struct {
	QName
	cardinality
	Value    Pattern
	Patterns []Pattern
}

func (e Element) Validate(node xml.Node) error {
	return e.validate(node, noopResolver)
}

func (e Element) validate(node xml.Node, ctx Resolver) error {
	if e.QualifiedName() != node.QualifiedName() {
		msg := fmt.Sprintf("want %s but got %s", e.QualifiedName(), node.QualifiedName())
		return createError(msg, node)
	}
	curr, ok := node.(*xml.Element)
	if !ok {
		return createError("xml element expected", node)
	}
	var (
		offset int
		attrs  int
	)
	for _, el := range e.Patterns {
		var err error
		switch el := el.(type) {
		case Element:
			step, err1 := validateNodes(curr.Nodes[offset:], el, ctx)
			offset += step
			err = err1
		case Attribute:
			err = el.validate(curr, ctx)
			attrs++
		case Choice:
			err = el.validate(curr, ctx)
			if err == nil {
				attrs++
				break
			}
			step, err1 := validateChoice(curr.Nodes[offset:], el, ctx)
			offset += step
			err = err1
		case Link:
			step, err1 := validateNodes(curr.Nodes[offset:], el, ctx)
			offset += step
			err = err1
		default:
			return fmt.Errorf("pattern not applicatble for element")
		}
		if err != nil {
			return err
		}
	}
	// if len(curr.Attrs) > attrs {
	// 	return fmt.Errorf("element has more attributes than expected")
	// }
	if e.Value != nil {
		return e.Value.Validate(curr)
	}
	return nil
}

type Text struct{}

func (t Text) Validate(node xml.Node) error {
	return t.validate(node, noopResolver)
}

func (_ Text) validate(node xml.Node, _ Resolver) error {
	if !node.Leaf() {
		return createError("element is not a text node", node)
	}
	return nil
}

func (_ Text) validateValue(_ string) error {
	return nil
}

type Empty struct{}

func (e Empty) Validate(node xml.Node) error {
	return e.validate(node, noopResolver)
}

func (_ Empty) validate(node xml.Node, _ Resolver) error {
	el, ok := node.(*xml.Element)
	if ok && len(el.Nodes) != 0 {
		return createError("element is not empty", node)
	}
	return nil
}

type Type struct {
	Name    string
	Format  string
	Pattern string
}

func (_ Type) Validate(_ xml.Node) error             { return nil }
func (_ Type) validate(_ xml.Node, _ Resolver) error { return nil }

type BoolType struct {
	Type
}

func (t BoolType) Validate(node xml.Node) error {
	return t.validate(node, noopResolver)
}

func (t BoolType) validate(node xml.Node, _ Resolver) error {
	return t.validateValue(node.Value())
}

func (t BoolType) validateValue(str string) error {
	_, err := strconv.ParseBool(str)
	if err != nil {
		err = ErrFormat
	}
	return err
}

type StringType struct {
	Type
	MinLength int
	MaxLength int
}

func (t StringType) Validate(node xml.Node) error {
	return t.validate(node, noopResolver)
}

func (t StringType) validate(node xml.Node, _ Resolver) error {
	return t.validateValue(node.Value())
}

func (t StringType) validateValue(str string) error {
	if t.MinLength > 0 && len(str) < t.MinLength {
		return ErrLength
	}
	if t.MaxLength > 0 && len(str) > t.MaxLength {
		return ErrLength
	}
	var err error
	switch t.Format {
	case "uri":
		_, err = url.Parse(str)
	case "hex":
		_, err = hex.DecodeString(str)
		return err
	case "base64":
		_, err = base64.StdEncoding.DecodeString(str)
	case "email":
	default:
	}
	if err != nil {
		return ErrFormat
	}
	return nil
}

type IntType struct {
	Type
	MinValue int
	MaxValue int
}

func (t IntType) Validate(node xml.Node) error {
	return t.validate(node, noopResolver)
}

func (t IntType) validate(node xml.Node, _ Resolver) error {
	return t.validateValue(node.Value())
}

func (t IntType) validateValue(str string) error {
	val, err := strconv.ParseInt(str, 0, 64)
	if err != nil {
		return ErrFormat
	}
	if val < int64(t.MinValue) {
		return ErrRange
	}
	if val > int64(t.MaxValue) {
		return ErrRange
	}
	return nil
}

type FloatType struct {
	Type
	MinValue float64
	MaxValue float64
}

func (t FloatType) Validate(node xml.Node) error {
	return t.validate(node, noopResolver)
}

func (t FloatType) validate(node xml.Node, _ Resolver) error {
	return t.validateValue(node.Value())
}

func (t FloatType) validateValue(str string) error {
	val, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return ErrFormat
	}
	if val < t.MinValue {
		return ErrRange
	}
	if val > t.MaxValue {
		return ErrRange
	}
	return nil
}

type TimeType struct {
	Type
	MinValue time.Time
	MaxValue time.Time
}

func (t TimeType) Validate(node xml.Node) error {
	return t.validate(node, noopResolver)
}

func (t TimeType) validate(node xml.Node, _ Resolver) error {
	return t.validateValue(node.Value())
}

func (t TimeType) validateValue(str string) error {
	layout := "2006-01-02"
	if t.Format != "" {
		layout = t.Format
	}
	when, err := time.Parse(layout, str)
	if err != nil {
		return ErrFormat
	}
	if !t.MinValue.IsZero() && when.Before(t.MinValue) {
		return ErrRange
	}
	if !t.MaxValue.IsZero() && when.After(t.MaxValue) {
		return ErrRange
	}
	return nil
}

type Enum struct {
	List []string
}

func (e Enum) Validate(node xml.Node) error {
	return e.validate(node, noopResolver)
}

func (e Enum) validate(node xml.Node, _ Resolver) error {
	return e.validateValue(node.Value())
}

func (e Enum) validateValue(str string) error {
	ok := slices.Contains(e.List, str)
	if !ok {
		return fmt.Errorf("%q: value not allowed (%s)", str, strings.Join(e.List, ", "))
	}
	return nil
}

func reassemble(start Pattern, others map[string]Pattern) (Pattern, error) {
	switch el := start.(type) {
	case Element:
		for i := range el.Patterns {
			p, err := reassemble(el.Patterns[i], others)
			if err != nil {
				return nil, err
			}
			el.Patterns[i] = p
		}
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
	el, ok := others[link.Ident]
	if !ok {
		return nil, fmt.Errorf("%s: pattern not defined")
	}
	switch el := el.(type) {
	case Element:
		if el.cardinality == 0 {
			el.cardinality = link.cardinality
		}
		for i := range el.Patterns {
			p, err := reassemble(el.Patterns[i], others)
			if err != nil {
				return nil, err
			}
			el.Patterns[i] = p
		}
		return el, nil
	case Choice:
		for i := range el.List {
			p, err := reassemble(el.List[i], others)
			if err != nil {
				return nil, err
			}
			el.List[i] = p
		}
		return el, nil
	default:
		return nil, fmt.Errorf("%s: unsupported pattern type", link.Ident)
	}
}

func validateNodes(nodes []xml.Node, elem Pattern, ctx Resolver) (int, error) {
	if c, ok := elem.(Choice); ok {
		return validateChoice(nodes, c, ctx)
	}
	var (
		count int
		ptr   int
		prv   = -1
	)
	for ; ptr < len(nodes); ptr++ {
		if _, ok := nodes[ptr].(*xml.Element); !ok {
			continue
		}
		if prv >= 0 && nodes[ptr].QualifiedName() != nodes[prv].QualifiedName() {
			break
		}
		if err := elem.validate(nodes[ptr], ctx); err != nil {
			if a, ok := elem.(Element); ok && a.Zero() {
				return 0, nil
			}
			return 0, err
		}
		count++
		prv = ptr
	}
	a, ok := elem.(Element)
	if !ok {
		return ptr, nil
	}
	switch {
	case count == 0 && a.cardinality.Zero():
	case count == 1 && a.cardinality.One():
	case count > 1 && a.cardinality.More():
	default:
		return 0, fmt.Errorf("element count mismatched")
	}
	return ptr, nil
}

func validateChoice(nodes []xml.Node, el Choice, ctx Resolver) (int, error) {
	var (
		step int
		err  error
	)
	for _, el := range el.List {
		if g, ok := el.(Group); ok {
			step, err = validateGroup(nodes, g, ctx)
		} else {
			step, err = validateNodes(nodes, el, ctx)
		}
		if err == nil {
			break
		}
	}
	return step, err
}

func validateGroup(nodes []xml.Node, el Group, ctx Resolver) (int, error) {
	var step int
	for i := range el.List {
		x, err := validateNodes(nodes[step:], el.List[i], ctx)
		if err != nil {
			return 0, err
		}
		step += x
	}
	return step, nil
}
