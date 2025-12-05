package xpath

import (
	"errors"
	"fmt"
	"iter"
	"slices"
	"strings"

	"github.com/midbel/codecs/environ"
	"github.com/midbel/codecs/xml"
)

var (
	ErrType        = errors.New("invalid type")
	ErrIndex       = errors.New("index out of range")
	ErrNode        = errors.New("element node expected")
	ErrRoot        = errors.New("root element expected")
	ErrUndefined   = errors.New("undefined")
	ErrEmpty       = errors.New("sequence is empty")
	ErrImplemented = errors.New("not implemented")
	ErrZero        = errors.New("division by zero")
	ErrArgument    = errors.New("invalid number of argument(s)")
	ErrSyntax      = errors.New("invalid syntax")
)

const (
	prioLow  = -1
	prioMed  = 0
	prioHigh = 1
)

type Expr interface {
	Find(xml.Node) (Sequence, error)
	find(Context) (Sequence, error)
}

type TypedExpr interface {
	Expr
	Type() XdmType
}

type Callable interface {
	Call(Context, []Expr) (Sequence, error)
}

type Evaluator struct {
	namespaces environ.Environ[string]
	variables  environ.Environ[Expr]
	builtins   environ.Environ[BuiltinFunc]
	baseURI    string
	elemNS     string
	typeNS     string
	funcNS     string

	thousandSep rune
	decimalSep  rune
}

func NewEvaluator() *Evaluator {
	e := Evaluator{
		namespaces:  environ.Empty[string](),
		variables:   environ.Empty[Expr](),
		builtins:    DefaultBuiltin(), // environ.Empty[BuiltinFunc](),
		elemNS:      "",
		typeNS:      schemaNS,
		funcNS:      functionNS,
		thousandSep: ',',
		decimalSep:  '.',
	}
	return &e
}

func (e *Evaluator) Sub() *Evaluator {
	x := *e
	x.namespaces = environ.Enclosed[string](e.namespaces)
	x.variables = environ.Enclosed[Expr](e.variables)
	x.builtins = environ.Enclosed[BuiltinFunc](e.builtins)
	return &x
}

func (e *Evaluator) Merge(other *Evaluator) {
	if m, ok := e.namespaces.(interface{ Merge(environ.Environ[string]) }); ok {
		m.Merge(other.namespaces)
	}
	if m, ok := e.variables.(interface{ Merge(environ.Environ[Expr]) }); ok {
		m.Merge(other.variables)
	}
	if m, ok := e.builtins.(interface {
		Merge(environ.Environ[BuiltinFunc])
	}); ok {
		m.Merge(other.builtins)
	}
}

func (e *Evaluator) Clone() *Evaluator {
	x := *e
	if c, ok := e.namespaces.(interface {
		Clone() environ.Environ[string]
	}); ok {
		x.namespaces = c.Clone()
	}
	if c, ok := e.variables.(interface{ Clone() environ.Environ[Expr] }); ok {
		x.variables = c.Clone()
	}
	if c, ok := e.builtins.(interface {
		Clone() environ.Environ[BuiltinFunc]
	}); ok {
		x.builtins = c.Clone()
	}
	return &x
}

func (e *Evaluator) Create(in string) (Expr, error) {
	var (
		cp  = NewCompiler(strings.NewReader(in))
		err error
	)
	cp.elemNS = e.elemNS
	cp.typeNS = e.typeNS
	cp.funcNS = e.funcNS

	for _, n := range e.namespaces.Names() {
		uri, _ := e.namespaces.Resolve(n)
		cp.RegisterNS(n, uri)
	}
	expr, err := cp.Compile()
	if err != nil {
		return nil, err
	}
	if q, ok := expr.(query); ok {
		q.ctx = defaultContext(nil)
		q.ctx.Environ = environ.ReadOnly(e.variables)
		q.ctx.Builtins = environ.ReadOnly(e.builtins)
		expr = q
	}
	return expr, nil
}

func (e *Evaluator) Find(query string, node xml.Node) (Sequence, error) {
	expr, err := e.Create(query)
	if err != nil {
		return nil, err
	}
	return expr.Find(node)
}

func (e *Evaluator) RegisterFunc(ident string, fn BuiltinFunc) {
	qn, err := xml.ParseName(ident)
	if err == nil {
		qn.Uri = defaultNS[qn.Space]
		if qn.Space == "" {
			qn.Uri = functionNS
		}
		ident = qn.ExpandedName()
	} else {
		qn.Uri = functionNS
		ident = qn.ExpandedName()
	}
	e.builtins.Define(ident, fn)
}

func (e *Evaluator) ResolveFunc(ident string) (BuiltinFunc, error) {
	return e.builtins.Resolve(ident)
}

func (e *Evaluator) RegisterNS(prefix, uri string) {
	e.namespaces.Define(prefix, uri)
}

func (e *Evaluator) ResolveNS(ident string) (string, error) {
	return e.namespaces.Resolve(ident)
}

func (e *Evaluator) Set(ident string, value Expr) {
	e.variables.Define(ident, value)
}

func (e *Evaluator) Define(ident, value string) {
	e.Set(ident, NewValueFromLiteral(value))
}

func (e *Evaluator) Resolve(ident string) (Expr, error) {
	return e.variables.Resolve(ident)
}

func (e *Evaluator) GetElemNS() string {
	return e.elemNS
}

func (e *Evaluator) SetElemNS(ns string) {
	e.elemNS = ns
}

func (e *Evaluator) SetFuncNS(ns string) {
	e.funcNS = ns
}

func (e *Evaluator) SetTypeNS(ns string) {
	e.typeNS = ns
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

type query struct {
	expr Expr
	ctx  Context
}

func (q query) Find(node xml.Node) (Sequence, error) {
	q.ctx.Node = node
	return q.find(q.ctx)
}

func (q query) find(ctx Context) (Sequence, error) {
	if ctx.Builtins == nil {
		ctx.Builtins = DefaultBuiltin()
	}
	return q.expr.find(ctx)
}

type wildcard struct{}

func (w wildcard) Find(node xml.Node) (Sequence, error) {
	return w.find(defaultContext(node))
}

func (w wildcard) find(ctx Context) (Sequence, error) {
	return Singleton(ctx.Node), nil
}

type root struct{}

func (r root) Find(node xml.Node) (Sequence, error) {
	return r.find(defaultContext(node).Root())
}

func (_ root) find(ctx Context) (Sequence, error) {
	root := ctx.Root()
	return Singleton(root.Node), nil
}

type current struct{}

func (c current) Find(node xml.Node) (Sequence, error) {
	return c.find(defaultContext(node))
}

func (_ current) find(ctx Context) (Sequence, error) {
	return Singleton(ctx.Node), nil
}

type step struct {
	curr Expr
	next Expr
}

func (s step) Find(node xml.Node) (Sequence, error) {
	return s.find(defaultContext(node))
}

func (s step) find(ctx Context) (Sequence, error) {
	is, err := s.curr.find(ctx)
	if err != nil {
		return nil, err
	}
	ctx.Size = len(is)

	var list Sequence
	for i, n := range is {
		ctx.Node = n.Node()
		ctx.Index = i + 1
		others, err := s.next.find(ctx)
		if err != nil {
			continue
		}
		list.Concat(others)
	}
	return list, nil
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
	attributeAxis      = "attribute"

	childTopAxis = "child-or-top"
	attrTopAxis  = "attribute-or-top"
)

type axis struct {
	kind string
	next Expr
}

func (a axis) Find(node xml.Node) (Sequence, error) {
	return a.find(defaultContext(node))
}

func (a axis) principalType() xml.NodeType {
	switch a.kind {
	case attributeAxis:
		return xml.TypeAttribute
	default:
		return xml.TypeElement
	}
}

func (a axis) isSelf() bool {
	return a.kind == selfAxis || a.kind == ancestorSelfAxis || a.kind == descendantSelfAxis
}

func (a axis) find(ctx Context) (Sequence, error) {
	var (
		list Sequence
		err  error
	)
	ctx.PrincipalType = a.principalType()
	switch a.kind {
	case selfAxis:
		return a.next.find(ctx)
	case childAxis:
		others, err := a.child(ctx)
		if err != nil {
			return nil, err
		}
		list.Concat(others)
	case parentAxis:
		p := ctx.Node.Parent()
		if p != nil {
			ctx.Node = p
			ctx.Index = 1
			ctx.Size = 1
			return a.next.find(ctx)
		}
		return nil, nil
	case ancestorAxis, ancestorSelfAxis:
		if a.isSelf() {
			list, err = a.next.find(ctx)
			if err != nil {
				return nil, err
			}
		}
		node := ctx.Node.Parent()
		for node != nil {
			ctx.Node = node
			ctx.Size = 1
			ctx.Index = 1
			other, err := a.next.find(ctx)
			if err == nil {
				list.Concat(other)
			}
			node = node.Parent()
		}
	case descendantAxis, descendantSelfAxis:
		if a.isSelf() {
			list, err = a.next.find(ctx)
			if err != nil {
				return nil, err
			}
		}
		others, err := a.descendant(ctx)
		if err == nil {
			list.Concat(others)
		}
	case prevAxis:
		return a.preceding(ctx)
	case prevSiblingAxis:
		return a.precedingSibling(ctx)
	case nextAxis:
		return a.following(ctx)
	case nextSiblingAxis:
		return a.followingSiblings(ctx)
	case attributeAxis:
		return a.attribute(ctx)
	default:
		return nil, ErrImplemented
	}
	return list, nil
}

func (a axis) preceding(ctx Context) (Sequence, error) {
	parent := ctx.Parent()
	if parent == nil {
		return nil, nil
	}
	var (
		list  Sequence
		nodes = getNodes(parent)
	)
	ctx.Size = len(nodes)
	for i := ctx.Node.Position() - 1; i >= 0; i-- {
		ctx.Node = nodes[i]
		ctx.Index = i
		ctx.Size = 1

		others, err := a.descendantReverse(ctx)
		if err == nil {
			list.Concat(others)
		}
		if others, err = a.next.find(ctx); err == nil {
			list.Concat(others)
		}
	}
	top := parent.Parent()
	if top == nil {
		return list, nil
	}
	ctx.Node = parent
	ctx.Size = 1
	ctx.Index = 1
	others, err := a.preceding(ctx)
	if err == nil {
		list.Concat(others)
	}
	return list, nil
}

func (a axis) precedingSibling(ctx Context) (Sequence, error) {
	var (
		list  Sequence
		nodes = getNodes(ctx.Parent())
	)
	ctx.Size = len(nodes)
	for i := ctx.Node.Position() - 1; i >= 0; i-- {
		ctx.Node = nodes[i]
		ctx.Index = i
		others, err := a.next.find(ctx)
		if err == nil {
			list.Concat(others)
		}
	}
	return list, nil
}

func (a axis) following(ctx Context) (Sequence, error) {
	parent := ctx.Parent()
	if parent == nil {
		return nil, nil
	}
	var (
		list  Sequence
		nodes = getNodes(parent)
	)
	ctx.Size = len(nodes)
	for i := ctx.Node.Position() + 1; i < len(nodes); i++ {
		ctx.Node = nodes[i]
		ctx.Index = i
		ctx.Size = 1

		others, err := a.next.find(ctx)
		if err == nil {
			list.Concat(others)
		}
		if others, err = a.descendant(ctx); err == nil {
			list.Concat(others)
		}
	}
	top := parent.Parent()
	if top == nil {
		return list, nil
	}
	next, ok := top.(interface{ NextSibling() xml.Node })
	if !ok {
		return list, nil
	}
	if next := next.NextSibling(); next == nil {
		parent = top
	} else {
		parent = next
	}
	ctx.Node = parent
	ctx.Size = 1
	ctx.Index = 1
	others, err := a.following(ctx)
	if err == nil {
		list.Concat(others)
	}
	return list, nil
}

func (a axis) followingSiblings(ctx Context) (Sequence, error) {
	var (
		list  Sequence
		nodes = getNodes(ctx.Parent())
	)
	ctx.Size = len(nodes)
	for i := ctx.Node.Position() + 1; i < len(nodes); i++ {
		ctx.Node = nodes[i]
		ctx.Index = i
		others, err := a.next.find(ctx)
		if err == nil {
			list.Concat(others)
		}
	}
	return list, nil
}

func (a axis) attribute(ctx Context) (Sequence, error) {
	if ctx.Type() != xml.TypeElement {
		return nil, nil
	}
	var (
		seq Sequence
		el  = ctx.Node.(*xml.Element)
	)
	ctx.Size = len(el.Attrs)
	for i := range el.Attrs {
		ctx.Node = &el.Attrs[i]
		ctx.Index = i + 1
		matches, err := a.next.find(ctx)
		if err != nil {
			return nil, err
		}
		seq.Concat(matches)
	}
	return seq, nil
}

func (a axis) descendantReverse(ctx Context) (Sequence, error) {
	var (
		list  Sequence
		nodes = getNodes(ctx.Node)
	)
	slices.Reverse(nodes)

	ctx.Size = len(nodes)
	for i, n := range nodes {
		ctx.Node = n
		ctx.Index = i
		matches, err := a.next.find(ctx)
		if err != nil {
			return nil, err
		}
		list.Concat(matches)

		matches, err = a.descendantReverse(ctx)
		if err != nil {
			return nil, err
		}
		list.Concat(matches)
	}
	return list, nil
}

func (a axis) descendant(ctx Context) (Sequence, error) {
	var (
		list  Sequence
		nodes = getNodes(ctx.Node)
	)
	ctx.Size = len(nodes)
	for i, n := range nodes {
		ctx.Node = n
		ctx.Index = i
		matches, err := a.next.find(ctx)
		if err != nil {
			return nil, err
		}
		list.Concat(matches)

		matches, err = a.descendant(ctx)
		if err != nil {
			return nil, err
		}
		list.Concat(matches)
	}
	return list, nil
}

func (a axis) child(ctx Context) (Sequence, error) {
	var (
		list  Sequence
		nodes = getNodes(ctx.Node)
	)
	ctx.Size = len(nodes)
	for i, c := range nodes {
		ctx.Node = c
		ctx.Index = i + 1
		others, _ := a.next.find(ctx)
		list.Concat(others)
	}
	return list, nil
}

type identifier struct {
	ident string
}

func (i identifier) Find(node xml.Node) (Sequence, error) {
	return i.find(defaultContext(node))
}

func (i identifier) find(ctx Context) (Sequence, error) {
	expr, err := ctx.Resolve(i.ident)
	if err != nil {
		return nil, err
	}
	if expr == nil {
		return nil, nil
	}
	return expr.find(ctx)
}

type name struct {
	xml.QName
}

func (n name) Find(node xml.Node) (Sequence, error) {
	return n.find(defaultContext(node))
}

func (n name) find(ctx Context) (Sequence, error) {
	if n.Space == "*" && n.Name == ctx.LocalName() {
		return Singleton(ctx.Node), nil
	}
	qn := n.getQName(ctx.Node)
	if n.Name == "*" && n.Uri == qn.Uri {
		return Singleton(ctx.Node), nil
	}
	if !n.QName.Equal(qn) {
		return nil, nil
	}
	return Singleton(ctx.Node), nil
}

func (n name) getQName(node xml.Node) xml.QName {
	var qn xml.QName
	switch x := node.(type) {
	case *xml.Element:
		qn = x.QName
	case *xml.Attribute:
		qn = x.QName
	case *xml.Instruction:
		qn = x.QName
	default:
	}
	return qn
}

type sequence struct {
	all []Expr
}

func (s sequence) Find(node xml.Node) (Sequence, error) {
	return s.find(defaultContext(node))
}

func (s sequence) find(ctx Context) (Sequence, error) {
	var list Sequence
	for i := range s.all {
		is, err := s.all[i].find(ctx)
		if err != nil {
			return nil, err
		}
		if is.Empty() {
			continue
		}
		list.Concat(is)
	}
	return list, nil
}

type binary struct {
	left  Expr
	right Expr
	op    rune
}

func (b binary) Find(node xml.Node) (Sequence, error) {
	return b.find(defaultContext(node))
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
		return nil, ErrImplemented
	}
	return fn(left, right)
}

type identity struct {
	left  Expr
	right Expr
}

func (i identity) Find(node xml.Node) (Sequence, error) {
	return i.find(defaultContext(node))
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

func (r reverse) Find(node xml.Node) (Sequence, error) {
	return r.find(defaultContext(node))
}

func (r reverse) find(ctx Context) (Sequence, error) {
	v, err := r.expr.find(ctx)
	if err != nil {
		return nil, err
	}
	if v.Empty() {
		return v, nil
	}
	x, err := toFloat(v[0].Value())
	if err == nil {
		x = -x
	}
	return Singleton(x), err
}

type literal struct {
	expr string
}

func (i literal) Find(node xml.Node) (Sequence, error) {
	return i.find(defaultContext(node))
}

func (i literal) find(_ Context) (Sequence, error) {
	return Singleton(i.expr), nil
}

func (i literal) Type() XdmType {
	return xsString
}

type number struct {
	expr float64
}

func (n number) Find(node xml.Node) (Sequence, error) {
	return n.find(defaultContext(node))
}

func (n number) find(_ Context) (Sequence, error) {
	return Singleton(n.expr), nil
}

func (n number) Type() XdmType {
	return xsDecimal
}

type boolean struct {
	expr bool
}

func (b boolean) Find(node xml.Node) (Sequence, error) {
	return b.find(defaultContext(node))
}

func (b boolean) find(_ Context) (Sequence, error) {
	return Singleton(b.expr), nil
}

func (b boolean) Type() XdmType {
	return xsBool
}

func isKind(str string) bool {
	switch str {
	case "node":
	case "element":
	case "text":
	case "comment":
	case "document-node":
	case "processing-instruction":
	case "attribute":
	default:
		return false
	}
	return true
}

type typeText struct{}

func (k typeText) Find(node xml.Node) (Sequence, error) {
	return k.find(defaultContext(node))
}

func (typeText) find(ctx Context) (Sequence, error) {
	var seq Sequence
	if ctx.Type() == xml.TypeText {
		seq = Singleton(ctx.Node)
	}
	return seq, nil
}

type typeComment struct{}

func (k typeComment) Find(node xml.Node) (Sequence, error) {
	return k.find(defaultContext(node))
}

func (typeComment) find(ctx Context) (Sequence, error) {
	var seq Sequence
	if ctx.Type() == xml.TypeComment {
		seq = Singleton(ctx.Node)
	}
	return seq, nil
}

type typeNode struct{}

func (k typeNode) Find(node xml.Node) (Sequence, error) {
	return k.find(defaultContext(node))
}

func (typeNode) find(ctx Context) (Sequence, error) {
	return Singleton(ctx.Node), nil
}

type typeDocument struct{}

func (k typeDocument) Find(node xml.Node) (Sequence, error) {
	return k.find(defaultContext(node))
}

func (typeDocument) find(ctx Context) (Sequence, error) {
	var seq Sequence
	if ctx.Type() == xml.TypeDocument {
		seq = Singleton(ctx.Node)
	}
	return seq, nil
}

type typeInstruction struct {
	name Expr
}

func (k typeInstruction) Find(node xml.Node) (Sequence, error) {
	return k.find(defaultContext(node))
}

func (t typeInstruction) find(ctx Context) (Sequence, error) {
	if ctx.Type() != xml.TypeInstruction {
		return nil, nil
	}
	if t.name != nil {
		res, err := t.name.find(ctx)
		if err != nil || res.Empty() {
			return nil, nil
		}
	}
	seq := Singleton(ctx.Node)
	return seq, nil
}

type typeAttribute struct {
	name Expr
}

func (k typeAttribute) Find(node xml.Node) (Sequence, error) {
	return k.find(defaultContext(node))
}

func (t typeAttribute) find(ctx Context) (Sequence, error) {
	switch ctx.Type() {
	case xml.TypeAttribute:
		return t.name.find(ctx)
	case xml.TypeElement:
		var (
			el  = ctx.Node.(*xml.Element)
			seq Sequence
		)
		for _, a := range el.Attributes() {
			ctx.Size = 1
			ctx.Index = 1
			ctx.Node = &a
			if res, err := t.name.find(ctx); err == nil {
				seq.Concat(res)
			}
		}
		return seq, nil
	case xml.TypeInstruction:
		var (
			el  = ctx.Node.(*xml.Instruction)
			seq Sequence
		)
		for _, a := range el.Attributes() {
			ctx.Size = 1
			ctx.Index = 1
			ctx.Node = &a
			if res, err := t.name.find(ctx); err == nil {
				seq.Concat(res)
			}
		}
		return seq, nil
	default:
		return nil, nil
	}
}

type typeElement struct {
	name Expr
}

func (k typeElement) Find(node xml.Node) (Sequence, error) {
	return k.find(defaultContext(node))
}

func (t typeElement) find(ctx Context) (Sequence, error) {
	if ctx.Type() != xml.TypeElement {
		return nil, nil
	}
	if t.name != nil {
		res, err := t.name.find(ctx)
		if err != nil || res.Empty() {
			return nil, nil
		}
	}
	seq := Singleton(ctx.Node)
	return seq, nil
}

type call struct {
	xml.QName
	args []Expr
}

func (c call) Find(node xml.Node) (Sequence, error) {
	return c.find(defaultContext(node))
}

func (c call) find(ctx Context) (Sequence, error) {
	fn, err := ctx.Builtins.Resolve(c.ExpandedName())
	if err != nil {
		return c.callUserDefinedFunction(ctx)
	}
	if fn == nil {
		return nil, fmt.Errorf("%s: %s", ErrImplemented, c.QualifiedName())
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

func (a attr) Find(node xml.Node) (Sequence, error) {
	return a.find(defaultContext(node))
}

func (a attr) find(ctx Context) (Sequence, error) {
	if ctx.Type() != xml.TypeElement {
		return nil, nil
	}
	el := ctx.Node.(*xml.Element)
	ix := slices.IndexFunc(el.Attrs, func(attr xml.Attribute) bool {
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

func (e except) Find(node xml.Node) (Sequence, error) {
	return e.find(defaultContext(node))
}

func (e except) find(ctx Context) (Sequence, error) {
	left, err := e.all[0].find(ctx)
	if err != nil {
		return nil, err
	}
	right, err := e.all[1].find(ctx)
	if err != nil {
		return nil, err
	}
	var res Sequence
	for i := range left {
		ok := slices.ContainsFunc(right, func(item Item) bool {
			return item.Node().Identity() == left[i].Node().Identity()
		})
		if !ok {
			res.Append(left[i])
		}
	}
	return res, nil
}

type intersect struct {
	all []Expr
}

func (e intersect) Find(node xml.Node) (Sequence, error) {
	return e.find(defaultContext(node))
}

func (e intersect) find(ctx Context) (Sequence, error) {
	left, err := e.all[0].find(ctx)
	if err != nil {
		return nil, err
	}
	right, err := e.all[1].find(ctx)
	if err != nil {
		return nil, err
	}
	var res Sequence
	for i := range right {
		ok := slices.ContainsFunc(left, func(item Item) bool {
			return item.Node().Identity() == right[i].Node().Identity()
		})
		if ok {
			res.Append(right[i])
		}
	}
	return res, nil
}

type union struct {
	all []Expr
}

func (u union) Find(node xml.Node) (Sequence, error) {
	return u.find(defaultContext(node))
}

func (u union) find(ctx Context) (Sequence, error) {
	left, err := u.all[0].find(ctx)
	if err != nil {
		return nil, err
	}
	right, err := u.all[1].find(ctx)
	if err != nil {
		return nil, err
	}
	left.Concat(right)
	return left.Unique(), nil
}

type subscript struct {
	expr  Expr
	index Expr
}

func (i subscript) Find(node xml.Node) (Sequence, error) {
	return i.find(defaultContext(node))
}

func (i subscript) find(ctx Context) (Sequence, error) {
	expr, err := i.subscriptExpr(ctx, i.expr)
	if err != nil || expr == nil {
		return nil, err
	}
	return expr.find(ctx)
}

func (i subscript) subscriptExpr(ctx Context, expr Expr) (Expr, error) {
	var err error
	switch e := expr.(type) {
	case identifier:
		expr, err := ctx.Resolve(e.ident)
		if err != nil {
			return nil, err
		}
		other := subscript{
			expr:  expr,
			index: i.index,
		}
		return other.subscriptExpr(ctx, other.expr)
	case array:
		expr, err = i.subscriptArray(ctx, e)
	case hashmap:
		expr, err = i.subscriptHashmap(ctx, e)
	case subscript:
		expr, err = e.subscriptExpr(ctx, e.expr)
		if err == nil {
			expr, err = i.subscriptExpr(ctx, expr)
		}
	default:
		err = fmt.Errorf("expression is not subscriptable")
	}
	return expr, err
}

func (i subscript) subscriptHashmap(ctx Context, arr hashmap) (Expr, error) {
	index, err := i.at(ctx)
	if err != nil {
		return nil, err
	}
	var sub Expr
	switch v := index.(type) {
	case string:
		sub = literal{
			expr: v,
		}
	case int64:
		sub = number{
			expr: float64(v),
		}
	case float64:
		sub = number{
			expr: v,
		}
	default:
		return nil, fmt.Errorf("map key can only be atomic value")
	}
	return arr.values[sub], nil
}

func (i subscript) subscriptArray(ctx Context, arr array) (Expr, error) {
	index, err := i.at(ctx)
	if err != nil {
		return nil, err
	}
	x, err := toInt(index)
	if err != nil {
		return nil, err
	}
	x--
	if x < 0 || int(x) >= len(arr.all) {
		return nil, nil
	}
	return arr.all[x], nil
}

func (i subscript) validIndex() error {
	switch i.index.(type) {
	case literal:
	case number:
	case identifier:
	default:
		return fmt.Errorf("expression can not be used as index")
	}
	return nil
}

func (i subscript) at(ctx Context) (any, error) {
	if err := i.validIndex(); err != nil {
		return nil, err
	}
	index, err := i.index.find(ctx)
	if err != nil {
		return nil, err
	}
	if index.Empty() {
		return nil, nil
	}
	if !index.Singleton() {
		return nil, fmt.Errorf("subscript returns more than one expr")
	}
	return index.First().Value(), nil
}

type filter struct {
	expr  Expr
	check Expr
}

func (f filter) Find(node xml.Node) (Sequence, error) {
	return f.find(defaultContext(node))
}

func (f filter) find(ctx Context) (Sequence, error) {
	list, err := f.expr.find(ctx)
	if err != nil {
		return nil, err
	}
	ctx.Size = list.Len()
	var ret Sequence
	for j, n := range list {
		ctx.Node = n.Node()
		ctx.Index = j + 1
		res, err := f.check.find(ctx)
		if err != nil || res.Empty() {
			continue
		}
		var (
			it = res.First()
			ok bool
		)
		switch x := it.Value().(type) {
		case float64:
			ok = ctx.Index == int(x)
		case int64:
			ok = ctx.Index == int(x)
		default:
			ok = EffectiveBooleanValue(res)
		}
		if ok {
			ret.Append(n)
		}
	}
	return ret, nil
}

type let struct {
	binds []binding
	expr  Expr
}

func (e let) Find(node xml.Node) (Sequence, error) {
	return e.find(defaultContext(node))
}

func (e let) find(ctx Context) (Sequence, error) {
	nest := ctx.Nest()
	for _, b := range e.binds {
		nest.Define(b.ident, b.expr)
	}
	return e.expr.find(nest)
}

type rng struct {
	left  Expr
	right Expr
}

func (r rng) Find(node xml.Node) (Sequence, error) {
	return r.find(defaultContext(node))
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
	var list Sequence
	if beg < end {
		for i := int(beg); i <= int(end); i++ {
			list.Append(createLiteral(float64(i)))
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

func (o loop) Find(node xml.Node) (Sequence, error) {
	return o.find(defaultContext(node))
}

func (o loop) find(ctx Context) (Sequence, error) {
	return nil, ErrImplemented
}

type conditional struct {
	test Expr
	csq  Expr
	alt  Expr
}

func (c conditional) Find(node xml.Node) (Sequence, error) {
	return c.find(defaultContext(node))
}

func (c conditional) find(ctx Context) (Sequence, error) {
	res, err := c.test.find(ctx)
	if err != nil {
		return nil, err
	}
	if res.True() {
		return c.csq.find(ctx)
	}
	return c.alt.find(ctx)
}

type quantified struct {
	binds []binding
	test  Expr
	every bool
}

func (q quantified) Find(node xml.Node) (Sequence, error) {
	return q.find(defaultContext(node))
}

func (q quantified) find(ctx Context) (Sequence, error) {
	for items, err := range combine(q.binds, ctx) {
		if err != nil {
			return nil, err
		}
		if items.Empty() {
			continue
		}
		nest := ctx.Nest()
		for j := range items {
			val := NewValue(items[j])
			nest.Define(q.binds[j].ident, val)
		}
		res, err := q.test.find(nest)
		if err != nil {
			if items.Len() != len(q.binds) {
				return Singleton(false), nil
			}
			return nil, err
		}
		if !res.True() && q.every {
			return Singleton(false), nil
		} else if isTrue(res) && !q.every {
			return Singleton(true), nil
		}
	}
	return Singleton(q.every), nil
}

func combine(list []binding, ctx Context) iter.Seq2[Sequence, error] {
	if len(list) == 0 {
		return nil
	}
	fn := func(yield func(Sequence, error) bool) {
		items, err := list[0].expr.find(ctx)
		if err != nil || items.Empty() {
			yield(nil, err)
			return
		}
		if len(list) == 1 {
			for i := range items {
				if !yield(Singleton(items[i]), nil) {
					break
				}
			}
			return
		}
		for _, i := range items {
			it := combine(list[1:], ctx)
			if it == nil {
				break
			}
			for arr, err := range it {
				if err != nil {
					yield(nil, err)
					return
				}
				var seq Sequence
				seq.Append(i)
				seq.Concat(arr)
				if ok := yield(seq, nil); !ok {
					return
				}
			}
		}
	}
	return fn
}

type value struct {
	seq Sequence
}

func NewValue(item Item) Expr {
	return value{
		seq: Singleton(item),
	}
}

func NewValueFromSequence(seq Sequence) Expr {
	return value{
		seq: slices.Clone(seq),
	}
}

func NewValueFromLiteral(value any) Expr {
	return NewValue(createLiteral(value))
}

func NewValueFromNode(node xml.Node) Expr {
	return NewValue(createNode(node))
}

func (v value) Find(node xml.Node) (Sequence, error) {
	return v.find(defaultContext(node))
}

func (v value) find(ctx Context) (Sequence, error) {
	return slices.Clone(v.seq), nil
}

func (v value) Type() XdmType {
	return xsUntyped
}

type hashmap struct {
	values map[Expr]Expr
}

func (a hashmap) Find(node xml.Node) (Sequence, error) {
	return a.find(defaultContext(node))
}

func (a hashmap) find(ctx Context) (Sequence, error) {
	vs := make(map[Item]Item)
	for k, v := range a.values {
		i, err := k.find(ctx)
		if err != nil {
			return nil, err
		}
		if i.Empty() {
			continue
		}
		j, err := v.find(ctx)
		if err != nil {
			return nil, err
		}
		vs[i.First()] = j.First()
	}
	return Singleton(vs), nil
}

type array struct {
	all []Expr
}

func (a array) Find(node xml.Node) (Sequence, error) {
	return a.find(defaultContext(node))
}

func (a array) find(ctx Context) (Sequence, error) {
	var list []Item
	for i := range a.all {
		others, err := a.all[i].find(ctx)
		if err != nil {
			return nil, err
		}
		if others.Empty() {
			continue
		}
		for j := range others {
			list = append(list, others[j])
		}
	}
	return Singleton(list), nil
}

type OccurrenceType int8

const (
	ZeroOrOneOccurrence OccurrenceType = 1 << iota
	ZeroOrMoreOccurrence
	OneOrMoreOccurrence
)

type instanceof struct {
	expr       Expr
	types      []XdmType
	occurrence OccurrenceType
}

func (i instanceof) Find(node xml.Node) (Sequence, error) {
	return i.find(defaultContext(node))
}

func (i instanceof) find(ctx Context) (Sequence, error) {
	seq, err := i.expr.find(ctx)
	if err != nil {
		return nil, err
	}
	var success int
	for _, item := range seq {
		sub, err := exprFromItem(item)
		if err != nil {
			return nil, err
		}
		var ok bool
		for _, t := range i.types {
			ok = t.InstanceOf(sub)
			if ok {
				success++
				break
			}
		}
	}
	var ok bool
	switch i.occurrence {
	case ZeroOrOneOccurrence:
		ok = success <= 1
	case ZeroOrMoreOccurrence:
		ok = true
	case OneOrMoreOccurrence:
		ok = success >= 1
	default:
		ok = success == 1
	}
	return Singleton(ok), nil
}

type cast struct {
	expr          Expr
	kind          XdmType
	allowEmptySeq bool
}

func (c cast) Find(node xml.Node) (Sequence, error) {
	return c.find(defaultContext(node))
}

func (c cast) find(ctx Context) (Sequence, error) {
	seq, err := c.expr.find(ctx)
	if err != nil {
		return nil, err
	}
	if seq.Empty() {
		if c.allowEmptySeq {
			return nil, nil
		}
		return nil, fmt.Errorf("empty sequence can not be cast to target type")
	}
	if !seq.Singleton() {
		return nil, fmt.Errorf("expected only one value to be casted")
	}
	return c.kind.Cast(seq.First().Value())
}

type castable struct {
	expr          Expr
	kind          XdmType
	allowEmptySeq bool
}

func (c castable) Find(node xml.Node) (Sequence, error) {
	return c.find(defaultContext(node))
}

func (c castable) find(ctx Context) (Sequence, error) {
	seq, err := c.expr.find(ctx)
	if err != nil {
		return nil, err
	}
	if seq.Empty() {
		if c.allowEmptySeq {
			return nil, nil
		}
		return nil, fmt.Errorf("empty sequence can not be cast to target type")
	}
	if !seq.Singleton() {
		return nil, fmt.Errorf("expected only one value to be casted")
	}
	ok := c.kind.Castable(seq.First().Value())
	return Singleton(ok), nil
}

func exprFromItem(it Item) (Expr, error) {
	var e Expr
	switch v := it.Value().(type) {
	case int64:
		e = number{
			expr: float64(v),
		}
	case float64:
		e = number{
			expr: v,
		}
	case string:
		e = literal{
			expr: v,
		}
	case bool:
		e = boolean{
			expr: v,
		}
	default:
		return nil, fmt.Errorf("item can not be converted to expr")
	}
	return e, nil
}
