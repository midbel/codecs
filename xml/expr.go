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
	Next(Node) (*NodeList, error)
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

func (q query) Next(node Node) (*NodeList, error) {
	if r := node.Parent(); r == nil {
		var qn QName
		root := NewElement(qn)
		root.Append(node)
		node = root
	}
	return q.expr.Next(node)
}

type all struct{}

func (_ all) Next(curr Node) (*NodeList, error) {
	if _, ok := curr.(*Element); !ok {
		return nil, ErrNode
	}
	list := createList()
	list.Push(curr)
	return list, nil
}

type current struct{}

func (_ current) Next(curr Node) (*NodeList, error) {
	list := createList()
	list.Push(curr)
	return list, nil
}

type parent struct{}

func (_ parent) Next(curr Node) (*NodeList, error) {
	n := curr.Parent()
	if n == nil {
		return nil, fmt.Errorf("root element has no parent")
	}
	list := createList()
	list.Push(curr)
	return list, nil
}

type absolute struct {
	expr Expr
}

func (a absolute) Next(curr Node) (*NodeList, error) {
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

func (_ root) Next(curr Node) (*NodeList, error) {
	n := curr.Parent()
	if n != nil {
		return nil, ErrRoot
	}
	list := createList()
	list.Push(curr)
	return list, nil
}

type axis struct {
	ident string
	next  Expr
}

func (a axis) Next(curr Node) (*NodeList, error) {
	list := createList()
	if a.ident == selfAxis || a.ident == descendantSelfAxis || a.ident == ancestorSelfAxis {
		other, err := a.next.Next(curr)
		if err != nil {
			return nil, err
		}
		list.Merge(other)
	}
	switch a.ident {
	case selfAxis:
		return list, nil
	case childAxis:
		el, ok := curr.(*Element)
		if !ok {
			return nil, ErrNode
		}
		for _, c := range el.Nodes {
			other, err := a.next.Next(c)
			if err == nil {
				list.Merge(other)
			}
		}
	case parentAxis:
		p := curr.Parent()
		if p != nil {
			return a.next.Next(p)
		}
		return nil, errDiscard
	case ancestorAxis, ancestorSelfAxis:
		el, ok := curr.(*Element)
		if !ok {
			return nil, ErrNode
		}
		for p := el.Parent(); p != nil; {
			other, err := a.next.Next(p)
			if err == nil {
				list.Merge(other)
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
				list.Merge(other)
			}
		}
	default:
		return nil, errImplemented
	}
	return list, nil
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

func (n name) Next(curr Node) (*NodeList, error) {
	if curr.QualifiedName() != n.QualifiedName() {
		return nil, errDiscard
	}
	list := createList()
	list.Push(curr)
	return list, nil
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

func (d descendant) Next(node Node) (*NodeList, error) {
	ns, err := d.curr.Next(node)
	if err != nil {
		return nil, err
	}
	list := createList()
	for i := range ns.All() {
		xs, err := d.traverse(i)
		if err != nil {
			continue
		}
		list.Merge(xs)
	}
	return list, nil
}

func (d *descendant) traverse(n Node) (*NodeList, error) {
	list, err := d.next.Next(n)
	if err == nil && list.Len() > 0 {
		return list, nil
	}
	if !d.deep {
		return nil, errDiscard
	}
	el, ok := n.(*Element)
	if !ok {
		return nil, errDiscard
	}
	list = createList()
	for i := range el.Nodes {
		tmp, err := d.next.Next(el.Nodes[i])
		if (err != nil || tmp.Len() == 0) && d.deep {
			tmp, err = d.traverse(el.Nodes[i])
		}
		if err == nil {
			list.Merge(tmp)
		}
	}

	return list, nil
}

type Predicate interface {
	Eval(Node) (any, error)
}

type noopExpr struct {
	Predicate
}

func createNoop(p Predicate) Expr {
	return noopExpr{
		Predicate: p,
	}
}

func evalExpr(e Expr, node Node) (any, error) {
	p, ok := e.(Predicate)
	if !ok {
		return nil, fmt.Errorf("expression can not be use as a predicate")
	}
	return p.Eval(node)
}

func (_ noopExpr) Next(_ Node) (*NodeList, error) {
	return nil, errImplemented
}

type binary struct {
	left  Expr
	right Expr
	op    rune
}

func (b binary) Eval(node Node) (any, error) {
	left, err := evalExpr(b.left, node)
	if err != nil {
		return nil, err
	}
	right, err := evalExpr(b.right, node)
	if err != nil {
		return nil, err
	}
	switch b.op {
	case opAdd:
		return apply(left, right, func(left, right float64) (float64, error) {
			return left + right, nil
		})
	case opSub:
		return apply(left, right, func(left, right float64) (float64, error) {
			return left - right, nil
		})
	case opMul:
		return apply(left, right, func(left, right float64) (float64, error) {
			return left * right, nil
		})
	case opDiv:
		return apply(left, right, func(left, right float64) (float64, error) {
			if right == 0 {
				return 0, errZero
			}
			return left / right, nil
		})
	case opMod:
		return apply(left, right, func(left, right float64) (float64, error) {
			if right == 0 {
				return 0, errZero
			}
			return math.Mod(left, right), nil
		})
	case opAnd:
		return toBool(left) && toBool(right), nil
	case opOr:
		return toBool(left) || toBool(right), nil
	case opEq:
		ok, err := isEqual(left, right)
		return ok, err
	case opNe:
		ok, err := isEqual(left, right)
		return !ok, err
	case opLt:
		ok, err := isLess(left, right)
		return ok, err
	case opLe:
		ok, err := isEqual(left, right)
		if !ok {
			ok, err = isLess(left, right)
		}
		return ok, err
	case opGt:
		ok, err := isEqual(left, right)
		if !ok {
			ok, err = isLess(left, right)
			ok = !ok
		}
		return ok, err
	case opGe:
		ok, err := isEqual(left, right)
		if !ok {
			ok, err = isLess(left, right)
			ok = !ok
		}
		return ok, err
	default:
		return nil, errImplemented
	}
}

type reverse struct {
	expr Expr
}

func (r reverse) Eval(node Node) (any, error) {
	v, err := evalExpr(r.expr, node)
	if err != nil {
		return nil, err
	}
	x, err := toFloat(v)
	if err == nil {
		x = -x
	}
	return x, err
}

type literal struct {
	expr string
}

func (i literal) Eval(_ Node) (any, error) {
	return i.expr, nil
}

type number struct {
	expr float64
}

func (n number) Eval(_ Node) (any, error) {
	return n.expr, nil
}

type call struct {
	ident string
	args  []Expr
}

func (c call) Next(curr Node) (*NodeList, error) {
	var (
		list = createList()
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
		return nil, fmt.Errorf("%s: %w function", c.ident, ErrUndefined)
	}
	if keep {
		list.Push(curr)
	}
	return list, nil
}

func (c call) Eval(node Node) (any, error) {
	fn, ok := builtins[c.ident]
	if !ok {
		return nil, fmt.Errorf("%s: %w function", c.ident, ErrUndefined)
	}
	if fn == nil {
		return nil, errImplemented
	}
	var args []any
	for i := range c.args {
		a, err := evalExpr(c.args[i], node)
		if err != nil {
			return nil, err
		}
		args = append(args, a)
	}
	return fn(node, args)
}

type attr struct {
	ident string
}

func (a attr) Next(node Node) (*NodeList, error) {
	return nil, errImplemented
}

func (a attr) Eval(node Node) (any, error) {
	el, ok := node.(*Element)
	if !ok {
		return nil, ErrNode
	}
	ix := slices.IndexFunc(el.Attrs, func(attr Attribute) bool {
		return attr.Name == a.ident
	})
	if ix >= 0 {
		return el.Attrs[ix].Value, nil
	}
	return "", nil
}

type alternative struct {
	all []Expr
}

func (a alternative) Next(node Node) (*NodeList, error) {
	list := createList()
	for i := range a.all {
		res, err := a.all[i].Next(node)
		if err != nil {
			return nil, err
		}
		list.Merge(res)
	}
	return list, nil
}

type filter struct {
	expr  Expr
	check Expr
}

func (f filter) Next(curr Node) (*NodeList, error) {
	list, err := f.expr.Next(curr)
	if err != nil {
		return nil, err
	}
	ret := createList()
	for n := range list.All() {
		res, err := evalExpr(f.check, n)
		if err != nil {
			continue
		}
		switch x := res.(type) {
		case float64:
			if int(x) == n.Position() {
				ret.Push(n)
			}
		case bool:
			if x {
				ret.Push(n)
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
