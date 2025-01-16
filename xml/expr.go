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
	ErrEmpty     = errors.New("sequence is empty")
)

func FromRoot(expr Expr) Expr {
	var base current
	return fromBase(expr, base)
}

func atRoot(expr Expr) bool {
	e, ok := expr.(step)
	if !ok {
		return false
	}
	switch e := e.curr.(type) {
	case step:
		return atRoot(e)
	case current:
		return true
	case root:
		return true
	default:
		return false
	}
}

func fromBase(expr, base Expr) Expr {
	switch e := expr.(type) {
	case query:
		e.expr = fromBase(e.expr, base)
		return e
	case union:
		for i := range e.all {
			e.all[i] = fromBase(e.all[i], base)
		}
		return e
	case intersect:
		for i := range e.all {
			e.all[i] = fromBase(e.all[i], base)
		}
		return e
	case except:
		for i := range e.all {
			e.all[i] = fromBase(e.all[i], base)
		}
		return e
	case step:
		if atRoot(e) {
			return e
		}
		e.curr = fromBase(e.curr, base)
		return e
	case filter:
		if atRoot(e.expr) {
			return e
		}
		e.expr = fromBase(e.expr, base)
		return e
	case axis:
		return transform(e.next, base)
	case call:
		for i := range e.args {
			e.args[i] = fromBase(e.args[i], base)
		}
		return e
	default:
		return expr
	}
}

func transform(expr Expr, base Expr) Expr {
	c := kind{
		kind: typeAll,
	}
	a := axis{
		kind: descendantSelfAxis,
		next: c,
	}
	expr = step{
		curr: base,
		next: step{
			curr: a,
			next: expr,
		},
	}
	return expr
}

type StepMode int8

func isXsl(mode StepMode) bool {
	return mode == ModeXsl2 || mode == ModeXsl3
}

const (
	ModeXpath3 StepMode = 1 << iota
	ModeXsl2
	ModeXsl3
)

const (
	ModeDefault = ModeXpath3
	ModeXpath   = ModeXpath3
	ModeXsl     = ModeXsl3
)

type Expr interface {
	Find(Node) ([]Item, error)
	find(Context) ([]Item, error)
}

type Context struct {
	Node
	Index         int
	Size          int
	PrincipalType NodeType
	Environ
}

func defaultContext(n Node) Context {
	ctx := createContext(n, 1, 1)
	ctx.Environ = Empty()
	return ctx
}

func createContext(n Node, pos, size int) Context {
	return Context{
		Node:  n,
		Index: pos,
		Size:  size,
	}
}

func (c Context) Sub(n Node, pos int, size int) Context {
	ctx := createContext(n, pos, size)
	ctx.Environ = Enclosed(c)
	ctx.PrincipalType = c.PrincipalType
	return ctx
}

func (c Context) Root() Context {
	curr := c.Node
	for {
		root := curr.Parent()
		if root == nil {
			break
		}
		curr = root
	}
	return c.Sub(curr, 1, 1)
}

func (c Context) Nodes() []Node {
	var nodes []Node
	if c.Type() == TypeDocument {
		doc := c.Node.(*Document)
		nodes = append(nodes, doc.Root())
	} else if c.Type() == TypeElement {
		el := c.Node.(*Element)
		nodes = slices.Clone(el.Nodes)
	}
	return nodes
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

	childTopAxis = "child-or-top"
	attrTopAxis  = "attribute-or-top"
)

func isSelf(axis string) bool {
	return axis == selfAxis || axis == ancestorSelfAxis || axis == descendantSelfAxis
}

type query struct {
	expr Expr
}

func (q query) FindWithEnv(node Node, env Environ) ([]Item, error) {
	ctx := createContext(node, 1, 1)
	ctx.Environ = env
	return q.find(ctx)
}

func (q query) Find(node Node) ([]Item, error) {
	return q.find(defaultContext(node))
}

func (q query) find(ctx Context) ([]Item, error) {
	return q.expr.find(ctx)
}

type wildcard struct{}

func (w wildcard) Find(node Node) ([]Item, error) {
	return w.find(defaultContext(node))
}

func (w wildcard) find(ctx Context) ([]Item, error) {
	// var (
	// 	list  = singleNode(ctx.Node)
	// 	nodes = ctx.Nodes()
	// )
	// for i, n := range nodes {
	// 	others, _ := w.find(ctx.Sub(n, i+1, len(nodes)))
	// 	list = slices.Concat(list, others)
	// }
	// return list, nil
	if ctx.Type() != TypeElement {
		return nil, nil
	}
	return singleNode(ctx.Node), nil
}

type root struct{}

func (r root) Find(node Node) ([]Item, error) {
	return r.find(defaultContext(node).Root())
}

func (_ root) find(ctx Context) ([]Item, error) {
	root := ctx.Root()
	return singleNode(root.Node), nil
}

type current struct{}

func (c current) Find(node Node) ([]Item, error) {
	return c.find(defaultContext(node))
}

func (_ current) find(ctx Context) ([]Item, error) {
	return singleNode(ctx.Node), nil
}

type step struct {
	curr Expr
	next Expr
}

func (s step) Find(node Node) ([]Item, error) {
	return s.find(defaultContext(node))
}

func (s step) find(ctx Context) ([]Item, error) {
	is, err := s.curr.find(ctx)
	if err != nil {
		return nil, err
	}
	var list []Item
	for i, n := range is {
		sub := ctx.Sub(n.Node(), i+1, len(is))
		others, err := s.next.find(sub)
		if err != nil {
			continue
		}
		list = slices.Concat(list, others)
	}
	return list, nil
}

type axis struct {
	kind string
	next Expr
}

func (a axis) Find(node Node) ([]Item, error) {
	return a.find(defaultContext(node))
}

func (a axis) principalType() NodeType {
	switch a.kind {
	case "attribueAxis":
		return TypeAttribute
	default:
		return TypeElement
	}
}

func (a axis) find(ctx Context) ([]Item, error) {
	var list []Item
	ctx.PrincipalType = a.principalType()
	if isSelf(a.kind) && ctx.Type() != TypeDocument {
		others, err := a.next.find(ctx)
		if err == nil {
			list = slices.Concat(list, others)
		}
		// list = slices.Concat(list, singleNode(ctx.Node))
	}
	switch a.kind {
	case selfAxis:
		return list, nil
	case childAxis:
		others, err := a.child(ctx)
		if err != nil {
			return nil, err
		}
		list = slices.Concat(list, others)
	case parentAxis:
		p := ctx.Node.Parent()
		if p != nil {
			return a.next.find(createContext(p, 1, 1))
		}
		return nil, nil
	case ancestorAxis, ancestorSelfAxis:
		node := ctx.Node.Parent()
		for {
			if node == nil {
				break
			}
			p := node.Parent()
			if p == nil {
				break
			}
			other, err := a.next.find(createContext(p, 1, 1))
			if err == nil {
				list = slices.Concat(list, other)
			}
			node = p
		}
	case descendantAxis, descendantSelfAxis:
		others, err := a.descendant(ctx)
		if err == nil {
			list = slices.Concat(list, others)
		}
	default:
		return nil, errImplemented
	}
	return list, nil
}

func (a axis) descendant(ctx Context) ([]Item, error) {
	if !isNode(ctx.Node) {
		return nil, nil
	}
	var (
		list  []Item
		nodes = ctx.Nodes()
		size  = len(nodes)
	)
	for i, n := range nodes {
		sub := ctx.Sub(n, i+1, size)
		res, _ := a.find(sub)
		list = slices.Concat(list, res)
	}
	return list, nil
}

func (a axis) child(ctx Context) ([]Item, error) {
	var (
		list  []Item
		nodes = ctx.Nodes()
	)
	for i, c := range nodes {
		others, err := a.next.find(ctx.Sub(c, i+1, len(nodes)))
		if err == nil {
			list = slices.Concat(list, others)
		}
	}
	return list, nil
}

type identifier struct {
	ident string
}

func (i identifier) Find(node Node) ([]Item, error) {
	return i.find(defaultContext(node))
}

func (i identifier) find(ctx Context) ([]Item, error) {
	expr, err := ctx.Resolve(i.ident)
	if err != nil {
		return nil, err
	}
	res, err := expr.find(ctx)
	return res, err
}

type name struct {
	space string
	ident string
}

func (n name) Find(node Node) ([]Item, error) {
	return n.find(defaultContext(node))
}

func (n name) find(ctx Context) ([]Item, error) {
	if n.space == "*" && n.ident == ctx.LocalName() {
		return singleNode(ctx.Node), nil
	}
	if ctx.QualifiedName() != n.QualifiedName() {
		return nil, errDiscard
	}
	return singleNode(ctx.Node), nil
}

func (n name) QualifiedName() string {
	if n.space == "" {
		return n.ident
	}
	return fmt.Sprintf("%s:%s", n.space, n.ident)
}

type sequence struct {
	all []Expr
}

func (s sequence) Find(node Node) ([]Item, error) {
	return s.find(defaultContext(node))
}

func (s sequence) find(ctx Context) ([]Item, error) {
	var list []Item
	for i := range s.all {
		is, err := s.all[i].find(ctx)
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

func (b binary) Find(node Node) ([]Item, error) {
	return b.find(defaultContext(node))
}

func (b binary) find(ctx Context) ([]Item, error) {
	left, err := b.left.find(ctx)
	if err != nil {
		return nil, err
	}
	right, err := b.right.find(ctx)
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
	case opConcat:
		var str1, str2 string
		if !isEmpty(left) {
			str1, _ = toString(left[0].Value())
		}
		if !isEmpty(right) {
			str2, _ = toString(right[0].Value())
		}
		res = str1 + str2
	case opAnd:
		res = isTrue(left) && isTrue(right)
	case opOr:
		res = isTrue(left) || isTrue(right)
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
	return singleValue(res), err
}

type reverse struct {
	expr Expr
}

func (r reverse) Find(node Node) ([]Item, error) {
	return r.find(defaultContext(node))
}

func (r reverse) find(ctx Context) ([]Item, error) {
	v, err := r.expr.find(ctx)
	if err != nil {
		return nil, err
	}
	x, err := toFloat(v)
	if err == nil {
		x = -x
	}
	return singleValue(x), err
}

type literal struct {
	expr string
}

func (i literal) Find(node Node) ([]Item, error) {
	return i.find(defaultContext(node))
}

func (i literal) find(_ Context) ([]Item, error) {
	return singleValue(i.expr), nil
}

type number struct {
	expr float64
}

func (n number) Find(node Node) ([]Item, error) {
	return n.find(defaultContext(node))
}

func (n number) find(_ Context) ([]Item, error) {
	return singleValue(n.expr), nil
}

func isKind(str string) bool {
	switch str {
	case "node":
	case "element":
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

	localName string
	localType string
}

func (k kind) Find(node Node) ([]Item, error) {
	return k.find(defaultContext(node))
}

func (k kind) find(ctx Context) ([]Item, error) {
	if k.kind == typeAll || ctx.Type() == k.kind {
		return singleNode(ctx.Node), nil
	}
	return nil, errDiscard
}

type call struct {
	ident string
	args  []Expr
}

func (c call) Find(node Node) ([]Item, error) {
	return c.find(defaultContext(node))
}

func (c call) find(ctx Context) ([]Item, error) {
	fn, ok := builtins[c.ident]
	if !ok {
		return nil, fmt.Errorf("%s: %w function", c.ident, ErrUndefined)
	}
	if fn == nil {
		return nil, fmt.Errorf("%s: %s", c.ident, errImplemented)
	}
	items, err := fn(ctx, c.args)
	if err != nil {
		err = fmt.Errorf("%s: %s", c.ident, err)
	}
	return items, err
}

type attr struct {
	ident string
}

func (a attr) Find(node Node) ([]Item, error) {
	return a.find(defaultContext(node))
}

func (a attr) find(ctx Context) ([]Item, error) {
	if ctx.Type() != TypeElement {
		return nil, nil
	}
	el := ctx.Node.(*Element)
	ix := slices.IndexFunc(el.Attrs, func(attr Attribute) bool {
		return attr.Name == a.ident
	})
	if ix < 0 {
		return nil, nil
	}
	return singleNode(&el.Attrs[ix]), nil
}

type except struct {
	all []Expr
}

func (e except) Find(node Node) ([]Item, error) {
	return e.find(defaultContext(node))
}

func (e except) find(ctx Context) ([]Item, error) {
	var list []Item
	for i := range e.all {
		res, err := e.all[i].find(ctx)
		if err != nil {
			continue
		}
		for i := range res {
			ok := slices.ContainsFunc(list, func(item Item) bool {
				return item.Node().Identity() == res[i].Node().Identity()
			})
			if ok {
				continue
			}
			list = append(list, res[i])
		}
	}
	return list, nil
}

type intersect struct {
	all []Expr
}

func (i intersect) Find(node Node) ([]Item, error) {
	return i.find(defaultContext(node))
}

func (e intersect) find(ctx Context) ([]Item, error) {
	var list []Item
	for i := range e.all {
		res, err := e.all[i].find(ctx)
		if err != nil {
			continue
		}

		for i := range res {
			ok := slices.ContainsFunc(list, func(item Item) bool {
				return item.Node().Identity() == res[i].Node().Identity()
			})
			if !ok {
				continue
			}
			list = append(list, res[i])
		}
	}
	return list, nil
}

type union struct {
	all []Expr
}

func (u union) Find(node Node) ([]Item, error) {
	return u.find(defaultContext(node))
}

func (u union) find(ctx Context) ([]Item, error) {
	var list []Item
	for i := range u.all {
		res, err := u.all[i].find(ctx)
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

func (f filter) Find(node Node) ([]Item, error) {
	return f.find(defaultContext(node))
}

func (f filter) find(ctx Context) ([]Item, error) {
	list, err := f.expr.find(ctx)
	if err != nil {
		return nil, err
	}
	var ret []Item
	for j, n := range list {
		res, err := f.check.find(ctx.Sub(n.Node(), j+1, len(list)))
		if err != nil {
			continue
		}
		if isEmpty(res) {
			continue
		}
		if !res[0].Atomic() && isTrue(res) {
			ret = append(ret, n)
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

func (e Let) Find(node Node) ([]Item, error) {
	return e.find(defaultContext(node))
}

func (e Let) find(ctx Context) ([]Item, error) {
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

func (o loop) Find(node Node) ([]Item, error) {
	return o.find(defaultContext(node))
}

func (o loop) find(ctx Context) ([]Item, error) {
	return nil, nil
}

type conditional struct {
	test Expr
	csq  Expr
	alt  Expr
}

func (c conditional) Find(node Node) ([]Item, error) {
	return c.find(defaultContext(node))
}

func (c conditional) find(ctx Context) ([]Item, error) {
	res, err := c.test.find(ctx)
	if err != nil {
		return nil, err
	}
	ok := isTrue(res)
	if ok {
		return c.csq.find(ctx)
	}
	return c.alt.find(ctx)
}

type quantified struct {
	binds []binding
	test  Expr
	every bool
}

func (q quantified) Find(node Node) ([]Item, error) {
	return q.find(defaultContext(node))
}

func (q quantified) find(ctx Context) ([]Item, error) {
	env := ctx.Environ
	ctx.Environ = Enclosed(ctx)
	defer func() {
		ctx.Environ = env
	}()
	items, err := q.binds[0].expr.find(ctx)
	if err != nil {
		return nil, err
	}
	for _, i := range items {
		val := value{
			item: i,
		}
		ctx.Environ.Define(q.binds[0].ident, val)
		res, err := q.test.find(ctx)
		if err != nil {
			return nil, err
		}
		if !isTrue(res) && q.every {
			return singleValue(false), nil
		} else if isTrue(res) && !q.every {
			return singleValue(true), nil
		}
	}
	return singleValue(true), nil
}

type value struct {
	item Item
}

func (v value) Find(node Node) ([]Item, error) {
	return v.find(defaultContext(node))
}

func (v value) find(ctx Context) ([]Item, error) {
	return []Item{v.item}, nil
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

func (c cast) Find(node Node) ([]Item, error) {
	return c.find(defaultContext(node))
}

func (c cast) find(ctx Context) ([]Item, error) {
	is, err := c.expr.find(ctx)
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

func (c castable) Find(node Node) ([]Item, error) {
	return c.find(defaultContext(node))
}

func (c castable) find(ctx Context) ([]Item, error) {
	is, err := c.expr.find(ctx)
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

func apply(left, right []Item, do func(left, right float64) (float64, error)) (any, error) {
	if isEmpty(left) {
		return math.NaN(), nil
	}
	if isEmpty(right) {
		return math.NaN(), nil
	}
	x, err := toFloat(left[0].Value())
	if err != nil {
		return nil, err
	}
	y, err := toFloat(right[0].Value())
	if err != nil {
		return nil, err
	}
	return do(x, y)
}

func isLess(left, right []Item) (bool, error) {
	if isEmpty(left) {
		return false, nil
	}
	if isEmpty(right) {
		return false, nil
	}
	switch x := left[0].Value().(type) {
	case float64:
		y, err := toFloat(right[0].Value())
		return x < y, err
	case string:
		y, err := toString(right[0].Value())
		return x < y, err
	default:
		return false, errType
	}
}

func isEqual(left, right []Item) (bool, error) {
	if isEmpty(left) {
		return false, nil
	}
	if isEmpty(right) {
		return false, nil
	}
	switch x := left[0].Value().(type) {
	case float64:
		y, err := toFloat(right[0].Value())
		if err != nil {
			return false, err
		}
		return x == y, nil
	case string:
		y, err := toString(right[0].Value())
		if err != nil {
			return false, err
		}
		return x == y, nil
	case bool:
		return x == toBool(right[0].Value()), nil
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
		return math.NaN(), nil
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
