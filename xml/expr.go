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
	Next(Node, Environ) ([]Item, error)
}

type Context struct {
	Node
	Position int
	Size     int
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

func (q query) Next(node Node, env Environ) ([]Item, error) {
	return q.expr.Next(node, env)
}

type wildcard struct{}

func (w wildcard) Next(curr Node, env Environ) ([]Item, error) {
	if curr.Type() != TypeElement {
		return nil, nil
	}
	var (
		list []Item
		elem = curr.(*Element)
	)
	for i := range elem.Nodes {
		if elem.Nodes[i].Type() != TypeElement {
			continue
		}
		if elem.Nodes[i].Leaf() {
			list = append(list, createNode(elem.Nodes[i]))
		} else {
			others, _ := w.Next(elem.Nodes[i], env)
			list = slices.Concat(list, others)
		}
	}
	return list, nil
}

type current struct{}

func (_ current) Next(curr Node, env Environ) ([]Item, error) {
	return createSingle(createNode(curr)), nil
}

type absolute struct {
	expr Expr
}

func (a absolute) Next(curr Node, env Environ) ([]Item, error) {
	return a.expr.Next(a.root(curr), env)
}

func (a absolute) root(node Node) Node {
	n := node.Parent()
	if n == nil {
		return node
	}
	return a.root(n)
}

type root struct{}

func (_ root) Next(curr Node, env Environ) ([]Item, error) {
	if curr.Type() != TypeDocument {
		return nil, ErrRoot
	}
	return createSingle(createNode(curr)), nil
}

type keeper interface {
	Keep(Node) bool
}

type step struct {
	expr Expr
	keeper
}

func (s step) Next(curr Node, env Environ) ([]Item, error) {
	nodes, err := s.expr.Next(curr, env)
	if err != nil {
		return nil, err
	}
	var list []Item
	for i := range nodes {
		if !s.Keep(nodes[i].Node()) {
			continue
		}
		list = append(list, nodes[i])
	}
	return list, nil
}

type axis struct {
	kind string
	next Expr
}

func (a axis) Next(curr Node, env Environ) ([]Item, error) {
	var list []Item
	if a.kind == selfAxis || a.kind == descendantSelfAxis || a.kind == ancestorSelfAxis {
		other, err := a.next.Next(curr, env)
		if err != nil {
			return nil, err
		}
		list = slices.Concat(list, other)
	}
	switch a.kind {
	case selfAxis:
		return list, nil
	case childAxis:
		others, err := a.child(curr, env)
		if err != nil {
			return nil, err
		}
		list = slices.Concat(list, others)
	case parentAxis:
		p := curr.Parent()
		if p != nil {
			return a.next.Next(p, env)
		}
		return nil, nil
	case ancestorAxis, ancestorSelfAxis:
		for p := curr.Parent(); p != nil; {
			other, err := a.next.Next(p, env)
			if err == nil {
				list = slices.Concat(list, other)
			}
		}
	case descendantAxis, descendantSelfAxis:
		others, err := a.descendant(getNode(curr), env)
		if err != nil {
			return nil, err
		}
		list = slices.Concat(list, others)
	default:
		return nil, errImplemented
	}
	return list, nil
}

func (a axis) descendant(curr Node, env Environ) ([]Item, error) {
	var (
		list  []Item
		nodes = getChildrenNodes(curr)
	)
	for i := range nodes {
		others, err := a.descendant(nodes[i], env)
		if err == nil {
			list = slices.Concat(list, others)
		}
	}
	return list, nil
}

func (a axis) child(curr Node, env Environ) ([]Item, error) {
	var (
		list  []Item
		nodes = getChildrenNodes(curr)
	)
	for _, c := range nodes {
		other, err := a.next.Next(c, env)
		if err == nil {
			list = slices.Concat(list, other)
		}
	}
	return list, nil
}

type identifier struct {
	ident string
}

func (i identifier) Next(curr Node, env Environ) ([]Item, error) {
	expr, err := env.Resolve(i.ident)
	if err != nil {
		return nil, err
	}
	return expr.Next(curr, env)
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

func (n name) Next(curr Node, env Environ) ([]Item, error) {
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
}

func (d descendant) Next(node Node, env Environ) ([]Item, error) {
	if node.Type() == TypeDocument {
		doc := node.(*Document)
		return d.traverse(doc.Root(), env)
	}
	ns, err := d.curr.Next(node, env)
	if err != nil {
		return nil, err
	}
	var list []Item
	for _, n := range ns {
		xs, err := d.traverse(n.Node(), env)
		if err != nil {
			continue
		}
		list = slices.Concat(list, xs)
	}
	if _, ok := d.curr.(root); ok && len(list) > 1 {
		list = list[1:]
	}
	return list, nil
}

func (d *descendant) traverse(n Node, env Environ) ([]Item, error) {
	list, err := d.next.Next(n, env)
	if len(list) > 0 || err != nil {
		return list, err
	}
	// if !d.deep {
	// 	return list, nil
	// }
	nodes := getChildrenNodes(n)
	for i := range nodes {
		tmp, err := d.traverse(nodes[i], env)
		if err == nil {
			list = slices.Concat(list, tmp)
		}
	}
	return list, nil
}

type sequence struct {
	all []Expr
}

func (s sequence) Next(curr Node, env Environ) ([]Item, error) {
	var list []Item
	for i := range s.all {
		is, err := s.all[i].Next(curr, env)
		if err != nil {
			return nil, err
		}
		list = slices.Concat(list, is)
	}
	return list, nil
}

type binary struct {
	left  Expr
	right Expr
	op    rune
}

func (b binary) Next(node Node, env Environ) ([]Item, error) {
	left, err := b.left.Next(node, env)
	if err != nil {
		return nil, err
	}
	right, err := b.right.Next(node, env)
	if err != nil {
		return nil, err
	}
	if len(left) == 0 || len(right) == 0 {
		return nil, fmt.Errorf("empty sequences")
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
		if len(left) == 0 || len(right) == 0 {
			res = false
			break
		}
		res1, err := getBooleanFromItem(left[0])
		if err != nil {
			return nil, err
		}
		res2, err := getBooleanFromItem(right[0])
		if err != nil {
			return nil, err
		}
		res = res1 && res2
	case opOr:
		res1, err := getBooleanFromItem(left[0])
		if err != nil {
			return nil, err
		}
		res2, err := getBooleanFromItem(right[0])
		if err != nil {
			return nil, err
		}
		res = res1 || res2
	case opEq:
		res, err = isEqual(left[0].Value(), right[0].Value())
	case opNe:
		ok, err1 := isEqual(left[0].Value(), right[0].Value())
		res, err = !ok, err1
	case opLt:
		res, err = isLess(left[0].Value(), right[0].Value())
	case opLe:
		ok, err1 := isEqual(left[0].Value(), right[0].Value())
		if !ok {
			ok, err1 = isLess(left[0].Value(), right[0].Value())
		}
		res, err = ok, err1
	case opGt:
		ok, err1 := isEqual(left[0].Value(), right[0].Value())
		if !ok {
			ok, err1 = isLess(left[0].Value(), right[0].Value())
			ok = !ok
		}
		res, err = ok, err1
	case opGe:
		ok, err1 := isEqual(left[0].Value(), right[0].Value())
		if !ok {
			ok, err1 = isLess(left[0].Value(), right[0].Value())
			ok = !ok
		}
		res, err = ok, err1
	default:
		return nil, errImplemented
	}
	return singleValue(res), err
}

type reverse struct {
	expr Expr
}

func (r reverse) Next(node Node, env Environ) ([]Item, error) {
	v, err := r.expr.Next(node, env)
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

func (i literal) Next(_ Node, env Environ) ([]Item, error) {
	return createSingle(createLiteral(i.expr)), nil
}

type number struct {
	expr float64
}

func (n number) Next(_ Node, env Environ) ([]Item, error) {
	return createSingle(createLiteral(n.expr)), nil
}

func isKind(str string) bool {
	switch str {
	case "node", "element":
	case "text":
	case "comment":
	case "document-node":
	case "processing-instruction":
	default:
		return false
	}
	return true
}

type kind struct {
	kind NodeType
}

func (k kind) Next(curr Node, env Environ) ([]Item, error) {
	if curr.Type() == k.kind {
		return createSingle(createNode(curr)), nil
	}
	return nil, errDiscard
}

type call struct {
	ident string
	args  []Expr
}

func (c call) Next(curr Node, env Environ) ([]Item, error) {
	var (
		list []Item
		keep bool
	)
	switch c.ident {
	case "node":
		keep = curr.Type() == TypeElement
	case "text":
		keep = curr.Type() == TypeText
	case "processing-instruction":
		keep = curr.Type() == TypeInstruction
	case "comment":
		keep = curr.Type() == TypeComment
	case "document-node":
		keep = curr.Type() == TypeDocument
	default:
		return c.eval(curr, env)
	}
	if keep {
		list = append(list, createNode(curr))
	}
	return list, nil
}

func (c call) eval(node Node, env Environ) ([]Item, error) {
	fn, ok := builtins[c.ident]
	if !ok {
		return nil, fmt.Errorf("%s: %w function", c.ident, ErrUndefined)
	}
	if fn == nil {
		return nil, errImplemented
	}
	return fn(node, c.args, env)
}

type attr struct {
	ident string
}

func (a attr) Next(node Node, env Environ) ([]Item, error) {
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

func (e except) Next(node Node, env Environ) ([]Item, error) {
	return nil, nil
}

type intersect struct {
	all []Expr
}

func (i intersect) Next(node Node, env Environ) ([]Item, error) {
	return nil, nil
}

type union struct {
	all []Expr
}

func (u union) Next(node Node, env Environ) ([]Item, error) {
	var list []Item
	for i := range u.all {
		res, err := u.all[i].Next(node, env)
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

func (f filter) Next(curr Node, env Environ) ([]Item, error) {
	list, err := f.expr.Next(curr, env)
	if err != nil {
		return nil, err
	}
	var ret []Item
	for j, n := range list {
		res, err := f.check.Next(n.Node(), env)
		if err != nil {
			continue
		}
		if isEmpty(res) {
			continue
		}
		var keep bool
		switch x := res[0].Value().(type) {
		case float64:
			keep = int(x) == j
		case bool:
			keep = x
		default:
			return nil, errType
		}
		if keep {
			ret = append(ret, n)
		}
	}
	return ret, nil
}

type Let struct {
	ident string
	expr  Expr
}

func (e Let) Next(curr Node, env Environ) ([]Item, error) {
	return nil, nil
}

type binding struct {
	ident string
	expr  Expr
}

type loop struct {
	binds []binding
	body  Expr
}

func (o loop) Next(curr Node, env Environ) ([]Item, error) {
	return nil, nil
}

type conditional struct {
	test Expr
	csq  Expr
	alt  Expr
}

func (c conditional) Next(curr Node, env Environ) ([]Item, error) {
	res, err := c.test.Next(curr, env)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return res, nil
	}
	ok, err := getBooleanFromItem(res[0])
	if err != nil {
		return nil, err
	}
	if ok {
		return c.csq.Next(curr, env)
	}
	return c.alt.Next(curr, env)
}

type quantified struct {
	binds []binding
	test  Expr
	every bool
}

func (q quantified) Next(curr Node, env Environ) ([]Item, error) {
	return nil, nil
}

type Type struct {
	QName
}

func (t Type) IsCastable(value any) Item {
	str, ok := value.(string)
	if !ok {
		return createLiteral(ok)
	}
	_, err := t.Cast(str)
	if err == nil {
		return createLiteral(true)
	}
	return createLiteral(false)
}

func (t Type) Cast(str string) (Item, error) {
	var (
		val any
		err error
	)
	switch t.QualifiedName() {
	case "xs:date", "date":
		val, err = castToDate(str)
	default:
		return nil, ErrCast
	}
	if err != nil {
		return nil, err
	}
	return createLiteral(val), nil
}

type cast struct {
	expr Expr
	kind Type
}

func (c cast) Next(curr Node, env Environ) ([]Item, error) {
	is, err := c.expr.Next(curr, env)
	if err != nil {
		return nil, err
	}
	for i := range is {
		if !is[i].Atomic() {
			return nil, errType
		}
		is[i], err = c.kind.Cast(is[i].Value().(string))
		if err != nil {
			return nil, err
		}
	}
	return is, nil
}

type castable struct {
	expr Expr
	kind Type
}

func (c castable) Next(curr Node, env Environ) ([]Item, error) {
	is, err := c.expr.Next(curr, env)
	if err != nil {
		return nil, err
	}
	for i := range is {
		if !is[i].Atomic() {
			return nil, errType
		}
		is[i] = c.kind.IsCastable(is[i].Value())
	}
	return is, nil
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
	case bool:
		return strconv.FormatBool(v), nil
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

func getNode(node Node) Node {
	if node.Type() == TypeDocument {
		doc := node.(*Document)
		return doc.Root()
	}
	return node
}

func getChildrenNodes(node Node) []Node {
	var nodes []Node
	switch c := node.(type) {
	case *Element:
		nodes = c.Nodes
	case *Document:
		root := c.Root()
		if root == nil {
			return nil
		}
		nodes = append(nodes, root)
	default:
	}
	return nodes
}
