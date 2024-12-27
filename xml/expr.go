package xml

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
)

var (
	ErrNode      = errors.New("element node expected")
	ErrRoot      = errors.New("root element expected")
	ErrUndefined = errors.New("undefined")
)

type Expr interface {
	Next(Node) ([]Item, error)
}

const (
	childAxis          = "child"
	parentAxis         = "parent"
	selfAxis           = "self"
	ancestorAxis       = "ancestor"
	ancestorSelfAxis   = "ancestor-or-self"
	descendantAxis     = "descendant"
	descendantSelfAxis = "descendant-or-self"
	prevAxis           = "preceding"
	prevSiblingAxis    = "preceding-sibling"
	nextAxis           = "following"
	nextSiblingAxis    = "following-sibling"
)

type query struct {
	expr Expr
}

func (q query) Next(node Node) ([]Item, error) {
	// el, ok := node.(*Element)
	// if !ok {
	// 	return nil, fmt.Errorf("xml element expected")
	// }
	// clone := &Element{
	// 	QName:    el.QName,
	// 	parent:   nil,
	// 	position: el.position,
	// }
	// clone.Attrs = slices.Clone(el.Attrs)
	// for i := range el.Nodes {
	// 	clone.Append(el.Nodes[i])
	// }
	// if r := clone.Parent(); r == nil {
	// 	var qn QName
	// 	root := NewElement(qn)
	// 	root.Append(clone)
	// 	node = root
	// }
	return q.expr.Next(node)
}

type all struct{}

func (_ all) Next(curr Node) ([]Item, error) {
	if _, ok := curr.(*Element); !ok {
		return nil, ErrNode
	}
	return createSingle(createNode(curr)), nil
}

type current struct{}

func (_ current) Next(curr Node) ([]Item, error) {
	return createSingle(createNode(curr)), nil
}

type parent struct{}

func (_ parent) Next(curr Node) ([]Item, error) {
	n := curr.Parent()
	if n == nil {
		return nil, fmt.Errorf("root element has no parent")
	}
	return createSingle(createNode(curr)), nil
}

type absolute struct {
	expr Expr
}

func (a absolute) Next(curr Node) ([]Item, error) {
	return a.expr.Next(a.root(curr))
}

func (a absolute) root(node Node) Node {
	n := node.Parent()
	if n == nil {
		return node
	}
	return a.root(n)
}

type root struct{}

func (_ root) Next(curr Node) ([]Item, error) {
	n := curr.Parent()
	if n != nil {
		return nil, ErrRoot
	}
	return createSingle(createNode(curr)), nil
}

type axis struct {
	ident string
	next  Expr
}

func (a axis) Next(curr Node) ([]Item, error) {
	var list []Item
	if a.ident == selfAxis || a.ident == descendantSelfAxis || a.ident == ancestorSelfAxis {
		other, err := a.next.Next(curr)
		if err != nil {
			return nil, err
		}
		list = slices.Concat(list, other)
	}
	switch a.ident {
	case selfAxis:
		return list, nil
	case childAxis:
		others, err := a.child(curr)
		if err != nil {
			return nil, err
		}
		list = slices.Concat(list, others)
	case parentAxis:
		p := curr.Parent()
		if p != nil {
			return a.next.Next(p)
		}
		return nil, errDiscard
	case ancestorAxis, ancestorSelfAxis:
		for p := curr.Parent(); p != nil; {
			other, err := a.next.Next(p)
			if err == nil {
				list = slices.Concat(list, other)
			}
		}
	case descendantAxis, descendantSelfAxis:
		el, ok := curr.(*Element)
		if !ok {
			return nil, ErrNode
		}
		for i := range el.Nodes {
			other, err := a.next.Next(el.Nodes[i])
			if err == nil {
				list = slices.Concat(list, other)
			}
		}
	default:
		return nil, errImplemented
	}
	return list, nil
}

func (a axis) child(curr Node) ([]Item, error) {
	var nodes []Node
	switch c := curr.(type) {
	case *Element:
		nodes = slices.Concat(nodes, c.Nodes)
	case *Document:
		root := c.Root()
		if root == nil {
			return nil, ErrRoot
		}
		nodes = append(nodes, root)
	default:
		return nil, ErrNode
	}
	var list []Item
	for _, c := range nodes {
		other, err := a.next.Next(c)
		if err == nil {
			list = slices.Concat(list, other)
		}
	}
	return list, nil
}

type identifier struct {
	ident string
}

func (i identifier) Next(curr Node) ([]Item, error) {
	return nil, nil
}

type name struct {
	space string
	ident string
}

func (n name) QualifiedName() string {
	if n.space == "" {
		return n.ident
	}
	return fmt.Sprintf("%s:%s", n.space, n.ident)
}

func (n name) Next(curr Node) ([]Item, error) {
	if curr.QualifiedName() != n.QualifiedName() {
		return nil, errDiscard
	}
	return createSingle(createNode(curr)), nil
}

func (n name) Eval(curr Node) (any, error) {
	el, ok := curr.(*Element)
	if !ok {
		return nil, ErrNode
	}
	child := el.Find(n.ident)
	if child == nil {
		return "", nil
	}
	return child.Value(), nil
}

type descendant struct {
	curr Expr
	next Expr
	deep bool
}

func (d descendant) Next(node Node) ([]Item, error) {
	ns, err := d.curr.Next(node)
	if err != nil {
		return nil, err
	}
	var list []Item
	for _, n := range ns {
		xs, err := d.traverse(n.Node())
		if err != nil {
			continue
		}
		list = slices.Concat(list, xs)
	}
	return list, nil
}

func (d *descendant) traverse(n Node) ([]Item, error) {
	list, err := d.next.Next(n)
	if err == nil && len(list) > 0 {
		return list, nil
	}
	if !d.deep {
		return nil, errDiscard
	}
	var nodes []Node
	switch c := n.(type) {
	case *Element:
		nodes = slices.Concat(nodes, c.Nodes)
	case *Document:
		root := c.Root()
		if root == nil {
			return nil, ErrRoot
		}
		nodes = append(nodes, root)
	default:
		return nil, ErrNode
	}
	list = list[:0]
	for i := range nodes {
		tmp, err := d.next.Next(nodes[i])
		if (err != nil || len(tmp) == 0) && d.deep {
			tmp, err = d.traverse(nodes[i])
		}
		if err == nil {
			list = slices.Concat(list, tmp)
		}
	}

	return list, nil
}

type sequence struct {
	all []Expr
}

func (s sequence) Next(_ Node) ([]Item, error) {
	return nil, nil
}

type binary struct {
	left  Expr
	right Expr
	op    rune
}

func (b binary) Next(node Node) ([]Item, error) {
	left, err := b.left.Next(node)
	if err != nil {
		return nil, err
	}
	right, err := b.right.Next(node)
	if err != nil {
		return nil, err
	}
	var res any
	switch b.op {
	case opAdd:
		res, err = apply(left, right, func(left, right float64) (float64, error) {
			return left + right, nil
		})
	case opSub:
		res, err = apply(left, right, func(left, right float64) (float64, error) {
			return left - right, nil
		})
	case opMul:
		res, err = apply(left, right, func(left, right float64) (float64, error) {
			return left * right, nil
		})
	case opDiv:
		res, err = apply(left, right, func(left, right float64) (float64, error) {
			if right == 0 {
				return 0, errZero
			}
			return left / right, nil
		})
	case opMod:
		res, err = apply(left, right, func(left, right float64) (float64, error) {
			if right == 0 {
				return 0, errZero
			}
			return math.Mod(left, right), nil
		})
	case opAnd:
		res = toBool(left) && toBool(right)
	case opOr:
		res = toBool(left) || toBool(right)
	case opEq:
		res, err = isEqual(left, right)
	case opNe:
		ok, err1 := isEqual(left, right)
		res, err = !ok, err1
	case opLt:
		res, err = isLess(left, right)
	case opLe:
		ok, err1 := isEqual(left, right)
		if !ok {
			ok, err1 = isLess(left, right)
		}
		res, err = ok, err1
	case opGt:
		ok, err1 := isEqual(left, right)
		if !ok {
			ok, err1 = isLess(left, right)
			ok = !ok
		}
		res, err = ok, err1
	case opGe:
		ok, err1 := isEqual(left, right)
		if !ok {
			ok, err1 = isLess(left, right)
			ok = !ok
		}
		res, err = ok, err1
	default:
		return nil, errImplemented
	}
	return createSingle(createLiteral(res)), err
}

type reverse struct {
	expr Expr
}

func (r reverse) Next(node Node) ([]Item, error) {
	v, err := r.expr.Next(node)
	if err != nil {
		return nil, err
	}
	x, err := toFloat(v)
	if err == nil {
		x = -x
	}
	return createSingle(createLiteral(x)), err
}

type literal struct {
	expr string
}

func (i literal) Next(_ Node) ([]Item, error) {
	return createSingle(createLiteral(i.expr)), nil
}

type number struct {
	expr float64
}

func (n number) Next(_ Node) ([]Item, error) {
	return createSingle(createLiteral(n.expr)), nil
}

type call struct {
	ident string
	args  []Expr
}

func (c call) Next(curr Node) ([]Item, error) {
	var (
		list []Item
		keep bool
	)
	switch c.ident {
	case "node":
		_, keep = curr.(*Element)
	case "text":
		_, keep = curr.(*Text)
	case "processing-instruction":
		_, keep = curr.(*Instruction)
	case "comment":
		_, keep = curr.(*Comment)
	default:
		return c.eval(curr)
	}
	if keep {
		list = append(list, createNode(curr))
	}
	return list, nil
}

func (c call) eval(node Node) ([]Item, error) {
	fn, ok := builtins[c.ident]
	if !ok {
		return nil, fmt.Errorf("%s: %w function", c.ident, ErrUndefined)
	}
	if fn == nil {
		return nil, errImplemented
	}
	return nil, nil
}

type attr struct {
	ident string
}

func (a attr) Next(node Node) ([]Item, error) {
	return nil, errImplemented
}

func (a attr) eval(node Node) ([]Item, error) {
	el, ok := node.(*Element)
	if !ok {
		return nil, ErrNode
	}
	ix := slices.IndexFunc(el.Attrs, func(attr Attribute) bool {
		return attr.Name == a.ident
	})
	if ix >= 0 {
		return createSingle(createLiteral(el.Attrs[ix].Value)), nil
	}
	return createSingle(createLiteral("")), nil
}

type except struct {
	all []Expr
}

func (e except) Next(node Node) ([]Item, error) {
	return nil, nil
}

type intersect struct {
	all []Expr
}

func (i intersect) Next(node Node) ([]Item, error) {
	return nil, nil
}

type union struct {
	all []Expr
}

func (u union) Next(node Node) ([]Item, error) {
	var list []Item
	for i := range u.all {
		res, err := u.all[i].Next(node)
		if err != nil {
			continue
		}
		list = slices.Concat(list, res)
	}
	return list, nil
}

type filter struct {
	expr  Expr
	check Expr
}

func (f filter) Next(curr Node) ([]Item, error) {
	list, err := f.expr.Next(curr)
	if err != nil {
		return nil, err
	}
	var ret []Item
	for j, n := range list {
		res, err := f.check.Next(n.Node())
		if err != nil {
			continue
		}
		if isEmpty(res) {
			return nil, errType
		}
		switch x := res[0].Value().(type) {
		case float64:
			if int(x) == j {
				ret = append(ret, n)
			}
		case bool:
			if x {
				ret = append(ret, n)
			}
		default:
			return nil, errType
		}
	}
	return ret, nil
}

func apply(left, right any, do func(left, right float64) (float64, error)) (any, error) {
	x, err := toFloat(left)
	if err != nil {
		return nil, err
	}
	y, err := toFloat(right)
	if err != nil {
		return nil, err
	}
	return do(x, y)
}

func isLess(left, right any) (bool, error) {
	switch x := left.(type) {
	case float64:
		y, err := toFloat(right)
		return x < y, err
	case string:
		y, err := toString(right)
		return x < y, err
	default:
		return false, errType
	}
}

func isEqual(left, right any) (bool, error) {
	switch x := left.(type) {
	case float64:
		y, err := toFloat(right)
		if err != nil {
			return false, err
		}
		return x == y, nil
	case string:
		y, err := toString(right)
		if err != nil {
			return false, err
		}
		return x == y, nil
	case bool:
		return x == toBool(right), nil
	default:
		return false, errType
	}
}

func toFloat(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, errType
	}
}

func toString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	default:
		return "", errType
	}
}

func toBool(v any) bool {
	switch v := v.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case string:
		return len(v) > 0
	default:
		return false
	}
}
