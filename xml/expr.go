package xml

import (
	"errors"
	"fmt"
	"iter"
	"math"
	"slices"
	"strconv"
	"time"
)

var (
	ErrNode      = errors.New("element node expected")
	ErrRoot      = errors.New("root element expected")
	ErrUndefined = errors.New("undefined")
	ErrEmpty     = errors.New("sequence is empty")
)

const (
	prioLow  = -1
	prioMed  = 0
	prioHigh = 1
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

func updateNS(expr Expr, ns string) Expr {
	switch e := expr.(type) {
	case name:
		e.Space = ns
		return e
	case query:
		e.expr = updateNS(e.expr, ns)
		return e
	case union:
		for i := range e.all {
			e.all[i] = updateNS(e.all[i], ns)
		}
		return e
	case intersect:
		for i := range e.all {
			e.all[i] = updateNS(e.all[i], ns)
		}
		return e
	case except:
		for i := range e.all {
			e.all[i] = updateNS(e.all[i], ns)
		}
		return e
	case step:
		e.curr = updateNS(e.curr, ns)
		e.next = updateNS(e.next, ns)
		return e
	case filter:
		e.expr = updateNS(e.expr, ns)
		return e
	case axis:
		e.next = updateNS(e.next, ns)
		return e
	case call:
		for i := range e.args {
			e.args[i] = updateNS(e.args[i], ns)
		}
		return e
	default:
		return expr
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

type Callable interface {
	Call(ctx Context, args []Expr) (Sequence, error)
}

func Call(ctx Context, body []Expr) (Sequence, error) {
	var (
		is  Sequence
		err error
	)
	for i := range body {
		is, err = body[i].find(ctx)
		if err != nil {
			break
		}
	}
	return is, err
}

type Expr interface {
	Find(Node) (Sequence, error)
	find(Context) (Sequence, error)
	MatchPriority() int
}

type Context struct {
	Node
	Index         int
	Size          int
	PrincipalType NodeType

	Environ[Expr]
	Builtins Environ[BuiltinFunc]
}

func DefaultContext(n Node) Context {
	ctx := createContext(n, 1, 1)
	ctx.Environ = Empty[Expr]()
	return ctx
}

func createContext(n Node, pos, size int) Context {
	return Context{
		Node:     n,
		Index:    pos,
		Size:     size,
		Builtins: DefaultBuiltin(),
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
	return getNodes(c.Node)
}

func getNodes(c Node) []Node {
	var nodes []Node
	if c.Type() == TypeDocument {
		doc := c.(*Document)
		nodes = append(nodes, doc.Root())
	} else if c.Type() == TypeElement {
		el := c.(*Element)
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

type Query struct {
	expr Expr
	Environ[Expr]
	Builtins Environ[BuiltinFunc]
}

func Build(query string) (*Query, error) {
	expr, err := CompileString(query)
	if err != nil {
		return nil, err
	}
	q := Query{
		expr: expr,
	}
	return &q, nil
}

func (q *Query) Find(node Node) (Sequence, error) {
	ctx := Context{
		Node:     node,
		Size:     1,
		Index:    1,
		Builtins: q.Builtins,
		Environ:  q.Environ,
	}
	if ctx.Builtins == nil {
		ctx.Builtins = DefaultBuiltin()
	}
	if ctx.Environ == nil {
		ctx.Environ = Empty[Expr]()
	}
	return q.find(ctx)
}

func (q *Query) UseNamespace(ns string) {
	if q.expr == nil {
		return
	}
	q.expr = updateNS(q.expr, ns)
}

func (q *Query) MatchPriority() int {
	if q.expr == nil {
		return prioLow
	}
	return q.expr.MatchPriority()
}

func (q *Query) find(ctx Context) (Sequence, error) {
	if q.expr == nil {
		return nil, fmt.Errorf("no query given")
	}
	return q.expr.find(ctx)
}

type query struct {
	expr Expr
}

func (q query) FindWithEnv(node Node, env Environ[Expr]) (Sequence, error) {
	ctx := createContext(node, 1, 1)
	ctx.Environ = env
	return q.find(ctx)
}

func (q query) Find(node Node) (Sequence, error) {
	return q.find(DefaultContext(node))
}

func (q query) MatchPriority() int {
	return q.expr.MatchPriority()
}

func (q query) find(ctx Context) (Sequence, error) {
	if ctx.Builtins == nil {
		ctx.Builtins = DefaultBuiltin()
	}
	return q.expr.find(ctx)
}

type wildcard struct{}

func (w wildcard) Find(node Node) (Sequence, error) {
	return w.find(DefaultContext(node))
}

func (w wildcard) MatchPriority() int {
	return prioLow
}

func (w wildcard) find(ctx Context) (Sequence, error) {
	if ctx.Type() != TypeElement {
		return nil, nil
	}
	return Singleton(ctx.Node), nil
}

type root struct{}

func (r root) Find(node Node) (Sequence, error) {
	return r.find(DefaultContext(node).Root())
}

func (r root) MatchPriority() int {
	return prioHigh
}

func (_ root) find(ctx Context) (Sequence, error) {
	root := ctx.Root()
	return Singleton(root.Node), nil
}

type current struct{}

func (c current) Find(node Node) (Sequence, error) {
	return c.find(DefaultContext(node))
}

func (c current) MatchPriority() int {
	return prioMed
}

func (_ current) find(ctx Context) (Sequence, error) {
	return Singleton(ctx.Node), nil
}

type step struct {
	curr Expr
	next Expr
}

func (s step) Find(node Node) (Sequence, error) {
	return s.find(DefaultContext(node))
}

func (s step) MatchPriority() int {
	return getPriority(prioMed, s.curr, s.next)
}

func (s step) find(ctx Context) (Sequence, error) {
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

func (a axis) Find(node Node) (Sequence, error) {
	return a.find(DefaultContext(node))
}

func (a axis) MatchPriority() int {
	return getPriority(prioMed, a.next)
}

func (a axis) principalType() NodeType {
	switch a.kind {
	case "attribueAxis":
		return TypeAttribute
	default:
		return TypeElement
	}
}

func (a axis) find(ctx Context) (Sequence, error) {
	var list []Item
	ctx.PrincipalType = a.principalType()
	if isSelf(a.kind) && ctx.Type() != TypeDocument {
		others, err := a.next.find(ctx)
		if err == nil {
			list = slices.Concat(list, others)
		}
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
	case prevAxis:
	case prevSiblingAxis:
		nodes := getNodes(ctx.Node.Parent())
		for i, x := range nodes {
			if x.Position() >= ctx.Node.Position() {
				break
			}
			other, err := a.next.find(ctx.Sub(x, i+1, len(nodes)))
			if err == nil {
				list = slices.Concat(list, other)
			}
		}
	case nextAxis:
	case nextSiblingAxis:
		nodes := getNodes(ctx.Node.Parent())
		for i, x := range slices.Backward(nodes) {
			if x.Position() <= ctx.Node.Position() {
				break
			}
			other, err := a.next.find(ctx.Sub(x, i+1, len(nodes)))
			if err == nil {
				list = slices.Concat(list, other)
			}
		}
	default:
		return nil, errImplemented
	}
	return list, nil
}

func (a axis) descendant(ctx Context) (Sequence, error) {
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

func (a axis) child(ctx Context) (Sequence, error) {
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

func (i identifier) Find(node Node) (Sequence, error) {
	return i.find(DefaultContext(node))
}

func (i identifier) MatchPriority() int {
	return prioHigh
}

func (i identifier) find(ctx Context) (Sequence, error) {
	expr, err := ctx.Resolve(i.ident)
	if err != nil {
		return nil, err
	}
	if expr == nil {
		return nil, nil
	}
	res, err := expr.find(ctx)
	return res, err
}

type name struct {
	QName
}

func (n name) Find(node Node) (Sequence, error) {
	return n.find(DefaultContext(node))
}

func (n name) MatchPriority() int {
	return prioMed
}

func (n name) find(ctx Context) (Sequence, error) {
	if n.Space == "*" && n.Name == ctx.LocalName() {
		return Singleton(ctx.Node), nil
	}
	if ctx.QualifiedName() != n.QualifiedName() {
		return nil, nil
	}
	return Singleton(ctx.Node), nil
}

type sequence struct {
	all []Expr
}

func (s sequence) Find(node Node) (Sequence, error) {
	return s.find(DefaultContext(node))
}

func (s sequence) MatchPriority() int {
	return prioLow
}

func (s sequence) find(ctx Context) (Sequence, error) {
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

type BinaryFunc func(Sequence, Sequence) (Sequence, error)

func doAdd(left, right Sequence) (Sequence, error) {
	return apply(left, right, func(left, right float64) (float64, error) {
		return left + right, nil
	})
}

func doSub(left, right Sequence) (Sequence, error) {
	return apply(left, right, func(left, right float64) (float64, error) {
		return left - right, nil
	})
}

func doMul(left, right Sequence) (Sequence, error) {
	return apply(left, right, func(left, right float64) (float64, error) {
		return left * right, nil
	})
}

func doDiv(left, right Sequence) (Sequence, error) {
	return apply(left, right, func(left, right float64) (float64, error) {
		if right == 0 {
			return 0, errZero
		}
		return left / right, nil
	})
}

func doMod(left, right Sequence) (Sequence, error) {
	return apply(left, right, func(left, right float64) (float64, error) {
		if right == 0 {
			return 0, errZero
		}
		return math.Mod(left, right), nil
	})
}

func doConcat(left, right Sequence) (Sequence, error) {
	var str1, str2 string
	if !left.Empty() {
		str1, _ = toString(left[0].Value())
	}
	if !right.Empty() {
		str2, _ = toString(right[0].Value())
	}
	return Singleton(str1 + str2), nil
}

func doAnd(left, right Sequence) (Sequence, error) {
	ok := isTrue(left) && isTrue(right)
	return Singleton(ok), nil
}

func doOr(left, right Sequence) (Sequence, error) {
	ok := isTrue(left) || isTrue(right)
	return Singleton(ok), nil
}

func doBefore(left, right Sequence) (Sequence, error) {
	if left.Empty() || right.Empty() {
		return Singleton(false), nil
	}
	ok := isBefore(left[0].Node(), right[0].Node())
	return Singleton(ok), nil
}

func doAfter(left, right Sequence) (Sequence, error) {
	if left.Empty() || right.Empty() {
		return Singleton(false), nil
	}
	ok := isAfter(left[0].Node(), right[0].Node())
	return Singleton(ok), nil
}

func doEqual(left, right Sequence) (Sequence, error) {
	res, err := isEqual(left, right)
	return Singleton(res), err
}

func doNotEqual(left, right Sequence) (Sequence, error) {
	res, err := isEqual(left, right)
	if err != nil {
		return nil, err
	}
	return Singleton(!res), nil
}

func doLesser(left, right Sequence) (Sequence, error) {
	res, err := isLess(left, right)
	if err != nil {
		return nil, err
	}
	return Singleton(res), nil
}

func doLessEq(left, right Sequence) (Sequence, error) {
	ok, err := isEqual(left, right)
	if err != nil {
		return nil, err
	}
	if ok {
		return Singleton(ok), nil
	}
	ok, err = isLess(left, right)
	if err != nil {
		return nil, err
	}
	return Singleton(ok), nil
}

func doGreater(left, right Sequence) (Sequence, error) {
	ok, err := isEqual(left, right)
	if err != nil {
		return nil, err
	}
	if ok {
		return Singleton(false), nil
	}
	ok, err = isLess(left, right)
	if err != nil {
		return nil, err
	}
	return Singleton(!ok), nil
}

func doGreatEq(left, right Sequence) (Sequence, error) {
	ok, err := isEqual(left, right)
	if err != nil {
		return nil, err
	}
	if ok {
		return Singleton(ok), nil
	}
	ok, err = isLess(left, right)
	if err != nil {
		return nil, err
	}
	return Singleton(!ok), nil
}

var binaryOp = map[rune]BinaryFunc{
	opAdd:    doAdd,
	opSub:    doSub,
	opMul:    doMul,
	opDiv:    doDiv,
	opMod:    doMod,
	opConcat: doConcat,
	opAnd:    doAnd,
	opOr:     doOr,
	opBefore: doBefore,
	opAfter:  doAfter,
	opEq:     doEqual,
	opNe:     doNotEqual,
	opLt:     doLesser,
	opLe:     doLessEq,
	opGt:     doGreater,
	opGe:     doGreatEq,
}

type binary struct {
	left  Expr
	right Expr
	op    rune
}

func (b binary) Find(node Node) (Sequence, error) {
	return b.find(DefaultContext(node))
}

func (b binary) MatchPriority() int {
	return getPriority(prioMed, b.left, b.right)
}

func (b binary) find(ctx Context) (Sequence, error) {
	left, err := b.left.find(ctx)
	if err != nil {
		return nil, err
	}
	right, err := b.right.find(ctx)
	if err != nil {
		return nil, err
	}
	fn, ok := binaryOp[b.op]
	if !ok {
		return nil, errImplemented
	}
	return fn(left, right)
}

type identity struct {
	left  Expr
	right Expr
}

func (i identity) Find(node Node) (Sequence, error) {
	return i.find(DefaultContext(node))
}

func (i identity) MatchPriority() int {
	return getPriority(prioMed, i.left, i.right)
}

func (i identity) find(ctx Context) (Sequence, error) {
	left, err := i.left.find(ctx)
	if err != nil {
		return nil, err
	}
	right, err := i.right.find(ctx)
	if err != nil {
		return nil, err
	}
	if left.Empty() || right.Empty() {
		return Singleton(false), nil
	}
	var (
		n1 = left[0].Node()
		n2 = right[0].Node()
	)
	return Singleton(n1.Identity() == n2.Identity()), nil
}

type reverse struct {
	expr Expr
}

func (r reverse) Find(node Node) (Sequence, error) {
	return r.find(DefaultContext(node))
}

func (r reverse) MatchPriority() int {
	return getPriority(prioMed, r.expr)
}

func (r reverse) find(ctx Context) (Sequence, error) {
	v, err := r.expr.find(ctx)
	if err != nil {
		return nil, err
	}
	x, err := toFloat(v)
	if err == nil {
		x = -x
	}
	return Singleton(x), err
}

type literal struct {
	expr string
}

func (i literal) Find(node Node) (Sequence, error) {
	return i.find(DefaultContext(node))
}

func (i literal) MatchPriority() int {
	return prioLow
}

func (i literal) find(_ Context) (Sequence, error) {
	return Singleton(i.expr), nil
}

type number struct {
	expr float64
}

func (n number) Find(node Node) (Sequence, error) {
	return n.find(DefaultContext(node))
}

func (n number) MatchPriority() int {
	return prioLow
}

func (n number) find(_ Context) (Sequence, error) {
	return Singleton(n.expr), nil
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

func (k kind) Find(node Node) (Sequence, error) {
	return k.find(DefaultContext(node))
}

func (k kind) MatchPriority() int {
	return prioLow
}

func (k kind) find(ctx Context) (Sequence, error) {
	if k.kind == typeAll || ctx.Type() == k.kind {
		return Singleton(ctx.Node), nil
	}
	return nil, nil
}

type call struct {
	QName
	args []Expr
}

func (c call) Find(node Node) (Sequence, error) {
	return c.find(DefaultContext(node))
}

func (c call) MatchPriority() int {
	return prioHigh
}

func (c call) find(ctx Context) (Sequence, error) {
	fn, err := ctx.Builtins.Resolve(c.QualifiedName())
	if err != nil {
		return c.callUserDefinedFunction(ctx)
	}
	if fn == nil {
		return nil, fmt.Errorf("%s: %s", errImplemented, c.QualifiedName())
	}
	items, err := fn(ctx, c.args)
	if err != nil {
		err = fmt.Errorf("%s: %s", c.QualifiedName(), err)
	}
	return items, err
}

func (c call) callUserDefinedFunction(ctx Context) (Sequence, error) {
	res, ok := ctx.Environ.(interface {
		ResolveFunc(string) (Callable, error)
	})
	if !ok {
		return nil, fmt.Errorf("%s can not be resolved", c.QualifiedName())
	}
	fn, err := res.ResolveFunc(c.QualifiedName())
	if err != nil {
		return nil, err
	}
	return fn.Call(ctx, c.args)
}

type attr struct {
	ident string
}

func (a attr) Find(node Node) (Sequence, error) {
	return a.find(DefaultContext(node))
}

func (a attr) MatchPriority() int {
	return prioMed
}

func (a attr) find(ctx Context) (Sequence, error) {
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
	return Singleton(&el.Attrs[ix]), nil
}

type except struct {
	all []Expr
}

func (e except) Find(node Node) (Sequence, error) {
	return e.find(DefaultContext(node))
}

func (e except) MatchPriority() int {
	return getPriority(prioMed, e.all...)
}

func (e except) find(ctx Context) (Sequence, error) {
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

func (e intersect) Find(node Node) (Sequence, error) {
	return e.find(DefaultContext(node))
}

func (e intersect) MatchPriority() int {
	return getPriority(prioMed, e.all...)
}

func (e intersect) find(ctx Context) (Sequence, error) {
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

func (u union) Find(node Node) (Sequence, error) {
	return u.find(DefaultContext(node))
}

func (u union) MatchPriority() int {
	return getPriority(prioMed, u.all...)
}

func (u union) find(ctx Context) (Sequence, error) {
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

func (f filter) Find(node Node) (Sequence, error) {
	return f.find(DefaultContext(node))
}

func (f filter) MatchPriority() int {
	return getPriority(prioHigh, f.expr, f.check)
}

func (f filter) find(ctx Context) (Sequence, error) {
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
		if res.Empty() {
			continue
		}
		if !res[0].Atomic() && isTrue(res) {
			ret = append(ret, n)
			continue
		}
		var keep bool
		switch x := res[0].Value().(type) {
		case float64:
			keep = int(x) == j+1
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

func Assign(ident string, expr Expr) Expr {
	return Let{
		ident: ident,
		expr:  expr,
	}
}

func (e Let) Find(node Node) (Sequence, error) {
	return e.find(DefaultContext(node))
}

func (e Let) MatchPriority() int {
	return prioLow
}

func (e Let) find(ctx Context) (Sequence, error) {
	ctx.Define(e.ident, e.expr)
	return nil, nil
}

type let struct {
	binds []binding
	expr  Expr
}

func (e let) Find(node Node) (Sequence, error) {
	return e.find(DefaultContext(node))
}

func (e let) MatchPriority() int {
	return prioLow
}

func (e let) find(ctx Context) (Sequence, error) {
	return nil, nil
}

type rng struct {
	left  Expr
	right Expr
}

func (r rng) Find(node Node) (Sequence, error) {
	return r.find(DefaultContext(node))
}

func (r rng) MatchPriority() int {
	return prioLow
}

func (r rng) find(ctx Context) (Sequence, error) {
	left, err := r.left.find(ctx)
	if err != nil {
		return nil, err
	}
	right, err := r.right.find(ctx)
	if err != nil {
		return nil, err
	}
	if left.Empty() || right.Empty() {
		return nil, nil
	}
	beg, err := toFloat(left[0].Value())
	if err != nil {
		return nil, err
	}
	end, err := toFloat(right[0].Value())
	if err != nil {
		return nil, err
	}
	var list []Item
	if beg < end {
		for i := int(beg); i <= int(end); i++ {
			list = append(list, createLiteral(float64(i)))
		}
	}
	return list, nil
}

type binding struct {
	ident string
	expr  Expr
}

type loop struct {
	binds []binding
	body  Expr
}

func (o loop) Find(node Node) (Sequence, error) {
	return o.find(DefaultContext(node))
}

func (o loop) MatchPriority() int {
	return prioLow
}

func (o loop) find(ctx Context) (Sequence, error) {
	return nil, nil
}

type conditional struct {
	test Expr
	csq  Expr
	alt  Expr
}

func (c conditional) Find(node Node) (Sequence, error) {
	return c.find(DefaultContext(node))
}

func (c conditional) MatchPriority() int {
	return getPriority(prioHigh, c.test)
}

func (c conditional) find(ctx Context) (Sequence, error) {
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

func (q quantified) Find(node Node) (Sequence, error) {
	return q.find(DefaultContext(node))
}

func (q quantified) MatchPriority() int {
	return getPriority(prioHigh, q.test)
}

func (q quantified) find(ctx Context) (Sequence, error) {
	env := ctx.Environ
	ctx.Environ = Enclosed(ctx)
	defer func() {
		ctx.Environ = env
	}()
	for items, err := range combine(q.binds, ctx) {
		if err != nil {
			return nil, err
		}
		for j, item := range items {
			val := value{
				item: item,
			}
			ctx.Environ.Define(q.binds[j].ident, val)
		}
		res, err := q.test.find(ctx)
		if err != nil {
			return nil, err
		}
		if !isTrue(res) && q.every {
			return Singleton(false), nil
		} else if isTrue(res) && !q.every {
			return Singleton(true), nil
		}
	}
	return Singleton(true), nil
}

func combine(list []binding, ctx Context) iter.Seq2[[]Item, error] {
	if len(list) == 0 {
		return nil
	}
	fn := func(yield func([]Item, error) bool) {
		is, err := list[0].expr.find(ctx)
		if err != nil {
			yield(nil, err)
			return
		}
		for _, i := range is {
			it := combine(list[1:], ctx)
			if it == nil {
				break
			}
			for arr, err := range it {
				if err != nil {
					yield(nil, err)
					return
				}
				vs := []Item{i}
				ok := yield(append(vs, arr...), nil)
				if !ok {
					break
				}
			}
		}
	}
	return fn
}

type value struct {
	item Item
}

func NewValue(item Item) Expr {
	return value{
		item: item,
	}
}

func NewValueFromLiteral(value any) Expr {
	return NewValue(createLiteral(value))
}

func NewValueFromNode(node Node) Expr {
	return NewValue(createNode(node))
}

func (v value) Find(node Node) (Sequence, error) {
	return v.find(DefaultContext(node))
}

func (v value) MatchPriority() int {
	return prioLow
}

func (v value) find(ctx Context) (Sequence, error) {
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

func (t Type) Cast(in any) (Item, error) {
	var (
		val any
		err error
	)
	switch t.QualifiedName() {
	case "xs:date", "date":
		val, err = castToDate(in)
	case "xs:decimal", "decimal":
		val, err = castToFloat(in)
	case "xs:boolean", "boolean":
		val, err = castToBool(in)
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

func As(expr Expr, name QName) Expr {
	if name.Zero() {
		return expr
	}
	t := Type{
		QName: name,
	}
	return cast{
		expr: expr,
		kind: t,
	}
}

func (c cast) Find(node Node) (Sequence, error) {
	return c.find(DefaultContext(node))
}

func (c cast) MatchPriority() int {
	return getPriority(prioLow, c.expr)
}

func (c cast) find(ctx Context) (Sequence, error) {
	is, err := c.expr.find(ctx)
	if err != nil {
		return nil, err
	}
	for i := range is {
		item, err := atomicItem(is[i])
		if err != nil {
			return nil, errType
		}
		is[i], err = c.kind.Cast(item.Value())
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

func (c castable) Find(node Node) (Sequence, error) {
	return c.find(DefaultContext(node))
}

func (c castable) MatchPriority() int {
	return getPriority(prioLow, c.expr)
}

func (c castable) find(ctx Context) (Sequence, error) {
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

func isBefore(left, right Node) bool {
	var (
		p1 = left.path()
		p2 = right.path()
	)
	for i := 0; i < len(p1) && i < len(p2); i++ {
		if p1[i] < p2[i] {
			return true
		} else if p1[i] > p2[i] {
			return false
		}
	}
	return len(p1) < len(p2)
}

func isAfter(left, right Node) bool {
	var (
		p1 = left.path()
		p2 = right.path()
	)
	for i := 0; i < len(p1) && i < len(p2); i++ {
		if p1[i] > p2[i] {
			return true
		} else if p1[i] < p2[i] {
			return false
		}
	}
	return len(p1) > len(p2)
}

func apply(left, right Sequence, do func(left, right float64) (float64, error)) (Sequence, error) {
	if left.Empty() || right.Empty() {
		return Singleton(math.NaN()), nil
	}
	x, err := toFloat(left[0].Value())
	if err != nil {
		return nil, err
	}
	y, err := toFloat(right[0].Value())
	if err != nil {
		return nil, err
	}
	res, err := do(x, y)
	if err != nil {
		return nil, err
	}
	return Singleton(res), nil
}

func compareItems(left, right Sequence, cmp func(left, right Item) (bool, error)) (bool, error) {
	if left.Empty() {
		return false, nil
	}
	if right.Empty() {
		return false, nil
	}
	for i := range left {
		for j := range right {
			ok, err := cmp(left[i], right[j])
			if ok || err != nil {
				return ok, err
			}
		}
	}
	return false, nil
}

func isLess(left, right Sequence) (bool, error) {
	return compareItems(left, right, func(left, right Item) (bool, error) {
		switch x := left.Value().(type) {
		case float64:
			y, err := toFloat(right.Value())
			return x < y, err
		case string:
			y, err := toString(right.Value())
			return x < y, err
		case time.Time:
			y, err := toTime(right.Value())
			return x.Before(y), err
		default:
			return false, errType
		}
	})

}

func isEqual(left, right Sequence) (bool, error) {
	return compareItems(left, right, func(left, right Item) (bool, error) {
		switch x := left.Value().(type) {
		case float64:
			y, err := toFloat(right.Value())
			return nearlyEqual(x, y), err
		case string:
			y, err := toString(right.Value())
			return x == y, err
		case bool:
			return x == toBool(right.Value()), nil
		case time.Time:
			y, err := toTime(right.Value())
			return x.Equal(y), err
		default:
			return false, errType
		}
	})
}

func nearlyEqual(left, right float64) bool {
	if left == right {
		return true
	}
	return math.Abs(left-right) < 0.000001
}

func toTime(value any) (time.Time, error) {
	switch v := value.(type) {
	case time.Time:
		return v, nil
	case string:
		return time.Parse("2006-01-02", v)
	case float64:
		return time.UnixMilli(int64(v)), nil
	default:
		var zero time.Time
		return zero, errType
	}
}

func toFloat(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	case time.Time:
		return float64(v.Unix()), nil
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
	case time.Time:
		return v.Format("2006-01-02"), nil
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
	case time.Time:
		return !v.IsZero()
	default:
		return false
	}
}

func lowestValue[T string | float64](items []T) T {
	var res T
	for i := range items {
		if i == 0 {
			res = items[i]
			continue
		}
		res = min(items[i], res)
	}
	return res
}

func greatestValue[T string | float64](items []T) T {
	var res T
	for i := range items {
		if i == 0 {
			res = items[i]
			continue
		}
		res = max(items[i], res)
	}
	return res
}

func getPriority(base int, values ...Expr) int {
	for i := range values {
		base += values[i].MatchPriority()
	}
	return base
}
