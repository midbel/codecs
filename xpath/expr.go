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

type Option func(q *Query) error

func WithEnforceNS() Option {
	return func(q *Query) error {
		q.enforceNS = true
		return nil
	}
}

func WithBaseUri(uri string) Option {
	return func(q *Query) error {
		if uri != "" {
			q.baseURI = uri
		}
		return nil
	}
}

func WithNamespace(prefix, uri string) Option {
	return func(q *Query) error {
		if uri != "" && prefix != "" {
			q.namespaces.Define(prefix, uri)
		}
		return nil
	}
}

func WithElementNS(uri string) Option {
	return func(q *Query) error {
		if uri != "" {
			q.static.elementNS = uri
		}
		return nil
	}
}

func WithTypeNS(uri string) Option {
	return func(q *Query) error {
		if uri != "" {
			q.typeNS = uri
		}
		return nil
	}
}

func WithFuncNS(uri string) Option {
	return func(q *Query) error {
		if uri != "" {
			q.funcNS = uri
		}
		return nil
	}
}

func WithVariable(ident, value string) Option {
	return func(q *Query) error {
		q.Define(ident, NewValueFromLiteral(value))
		return nil
	}
}

type Expr interface {
	Find(xml.Node) (Sequence, error)
	find(Context) (Sequence, error)
	MatchPriority() int
}

type TypedExpr interface {
	Expr
	Type() XdmType
}

type Callable interface {
	Call(Context, []Expr) (Sequence, error)
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

type Query struct {
	expr Expr
	environ.Environ[Expr]
	Builtins environ.Environ[BuiltinFunc]

	*static
}

func Find(node xml.Node, query string) (Sequence, error) {
	q, err := Build(query)
	if err != nil {
		return nil, err
	}
	return q.Find(node)
}

func BuildWith(query string, options ...Option) (*Query, error) {
	q := Query{
		static:  createStatic(),
		Environ: environ.Empty[Expr](),
	}
	for _, o := range options {
		if err := o(&q); err != nil {
			return nil, err
		}
	}
	var (
		cp  = NewCompiler(strings.NewReader(query))
		err error
	)
	for _, n := range q.namespaces.Names() {
		uri, _ := q.namespaces.Resolve(n)
		cp.DefineNS(n, uri)
	}
	if q.expr, err = cp.Compile(); err != nil {
		return nil, err
	}
	return &q, nil
}

func Build(query string) (*Query, error) {
	return BuildWith(query)
}

func (q *Query) Find(node xml.Node) (Sequence, error) {
	ctx := createContext(node, 1, 1)
	ctx.static = q.static.Readonly()
	ctx.Builtins = q.Builtins
	ctx.Environ = q.Environ

	if ctx.Builtins == nil {
		ctx.Builtins = DefaultBuiltin()
	}
	if ctx.Environ == nil {
		ctx.Environ = environ.Empty[Expr]()
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
		return nil, fmt.Errorf("no query can be executed")
	}
	return q.expr.find(ctx)
}

type query struct {
	expr Expr
}

func (q query) FindWithEnv(node xml.Node, env environ.Environ[Expr]) (Sequence, error) {
	ctx := createContext(node, 1, 1)
	ctx.Environ = env
	return q.find(ctx)
}

func (q query) Find(node xml.Node) (Sequence, error) {
	return q.find(defaultContext(node))
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

func (w wildcard) Find(node xml.Node) (Sequence, error) {
	return w.find(defaultContext(node))
}

func (w wildcard) MatchPriority() int {
	return prioLow
}

func (w wildcard) find(ctx Context) (Sequence, error) {
	if ctx.Type() != xml.TypeElement {
		return nil, nil
	}
	return Singleton(ctx.Node), nil
}

type root struct{}

func (r root) Find(node xml.Node) (Sequence, error) {
	return r.find(defaultContext(node).Root())
}

func (r root) MatchPriority() int {
	return prioHigh
}

func (_ root) find(ctx Context) (Sequence, error) {
	root := ctx.Root()
	return Singleton(root.Node), nil
}

type current struct{}

func (c current) Find(node xml.Node) (Sequence, error) {
	return c.find(defaultContext(node))
}

func (c current) MatchPriority() int {
	return prioMed
}

func (_ current) find(ctx Context) (Sequence, error) {
	return Singleton(ctx.Node), nil
}

type stepmap struct {
	step Expr
	expr Expr
}

func (s stepmap) Find(node xml.Node) (Sequence, error) {
	return s.find(defaultContext(node))
}

func (s stepmap) MatchPriority() int {
	return getPriority(prioMed, s.step, s.expr)
}

func (s stepmap) find(ctx Context) (Sequence, error) {
	items, err := s.step.find(ctx)
	if err != nil {
		return nil, err
	}
	if items.Empty() {
		return items, nil
	}
	ctx.Size = items.Len()

	var seq Sequence
	for j, n := range items {
		ctx.Node = n.Node()
		ctx.Index = j + 1
		others, err := s.expr.find(ctx)
		if err != nil {
			return nil, err
		}
		seq.Concat(others)
	}
	return seq, nil
}

type step struct {
	curr Expr
	next Expr
}

func (s step) Find(node xml.Node) (Sequence, error) {
	return s.find(defaultContext(node))
}

func (s step) MatchPriority() int {
	return getPriority(prioMed, s.curr, s.next)
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

func (a axis) MatchPriority() int {
	return getPriority(prioMed, a.next)
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
	case prevSiblingAxis:
		nodes := getNodes(ctx.Parent())
		ctx.Size = len(nodes)
		for i := ctx.Node.Position() - 1; i >= 0; i-- {
			ctx.Node = nodes[i]
			ctx.Index = i
			others, err := a.next.find(ctx)
			if err == nil {
				list.Concat(others)
			}
		}
	case nextAxis:
	case nextSiblingAxis:
		nodes := getNodes(ctx.Parent())
		ctx.Size = len(nodes)
		for i := ctx.Node.Position() + 1; i < len(nodes); i++ {
			ctx.Node = nodes[i]
			ctx.Index = i
			others, err := a.next.find(ctx)
			if err == nil {
				list.Concat(others)
			}
		}
	case attributeAxis:
		return a.attribute(ctx)
	default:
		return nil, ErrImplemented
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
		matches, err := a.next.Find(ctx)
		if err != nil {
			return nil, err
		}
		seq.Concat(matches)
	}
	return seq, nil
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
	return expr.find(ctx)
}

type name struct {
	xml.QName
}

func (n name) Find(node xml.Node) (Sequence, error) {
	return n.find(defaultContext(node))
}

func (n name) MatchPriority() int {
	return prioMed
}

func (n name) find(ctx Context) (Sequence, error) {
	if n.Space == "*" && n.Name == ctx.LocalName() {
		return Singleton(ctx.Node), nil
	}
	qn := n.getQName(ctx.Node)
	if !ctx.enforceNS {
		if n.Name == "*" && n.Space == qn.Space {
			return Singleton(ctx.Node), nil
		}
		if ctx.QualifiedName() != n.QualifiedName() {
			return nil, nil
		}
		return Singleton(ctx.Node), nil
	}
	if n.Uri == "" {
		n.Uri = ctx.elementNS
	}
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

func (s sequence) MatchPriority() int {
	return prioLow
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

func (r reverse) Find(node xml.Node) (Sequence, error) {
	return r.find(defaultContext(node))
}

func (r reverse) MatchPriority() int {
	return getPriority(prioMed, r.expr)
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

func (i literal) MatchPriority() int {
	return prioLow
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

func (n number) MatchPriority() int {
	return prioLow
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

func (b boolean) MatchPriority() int {
	return prioLow
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
	default:
		return false
	}
	return true
}

type kind struct {
	kind xml.NodeType

	localName string
	localType string
}

func (k kind) Find(node xml.Node) (Sequence, error) {
	return k.find(defaultContext(node))
}

func (k kind) MatchPriority() int {
	return prioLow
}

func (k kind) find(ctx Context) (Sequence, error) {
	if k.kind == 0 || k.kind == xml.TypeNode || ctx.Type() == k.kind {
		return Singleton(ctx.Node), nil
	}
	return nil, nil
}

type call struct {
	xml.QName
	args []Expr
}

func (c call) Find(node xml.Node) (Sequence, error) {
	return c.find(defaultContext(node))
}

func (c call) MatchPriority() int {
	return prioHigh
}

func (c call) find(ctx Context) (Sequence, error) {
	if c.QName.Uri == "" {
		c.QName.Uri = ctx.funcNS
	}
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

func (a attr) MatchPriority() int {
	return prioMed
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

func (e except) MatchPriority() int {
	return getPriority(prioMed, e.all...)
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

func (e intersect) MatchPriority() int {
	return getPriority(prioMed, e.all...)
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

func (u union) MatchPriority() int {
	return getPriority(prioMed, u.all...)
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
	return left, nil
}

type subscript struct {
	expr  Expr
	index Expr
}

func (i subscript) Find(node xml.Node) (Sequence, error) {
	return i.find(defaultContext(node))
}

func (i subscript) MatchPriority() int {
	return getPriority(prioHigh, i.expr, i.index)
}

func (i subscript) find(ctx Context) (Sequence, error) {
	arr, err := i.getArrayFromExpr(ctx)
	if err != nil {
		return nil, err
	}
	res, err := i.index.find(ctx)
	if err != nil {
		return nil, err
	}
	if res.Empty() {
		return nil, nil
	}
	ix, err := toInt(res[0].Value())
	if err != nil {
		return nil, err
	}
	ix--
	if ix < 0 || ix >= int64(len(arr.all)) {
		return nil, nil
	}
	return arr.all[ix].find(ctx)
}

func (i subscript) getArrayFromExpr(ctx Context) (array, error) {
	var arr array
	switch id := i.expr.(type) {
	case subscript:
		seq, err := id.find(ctx)
		if err != nil {
			return arr, err
		}
		if seq.Empty() {
			return arr, nil
		}
		values, ok := seq[0].Value().([]any)
		if !ok {
			return arr, ErrType
		}
		for j := range values {
			arr.all = append(arr.all, NewValueFromLiteral(values[j]))
		}
		return arr, nil
	case identifier:
		id, ok := i.expr.(identifier)
		if !ok {
			return arr, fmt.Errorf("identifier expected")
		}
		value, err := ctx.Resolve(id.ident)
		if err != nil {
			return arr, err
		}
		arr, ok := value.(array)
		if !ok {
			return arr, ErrType
		}
		return arr, nil
	case array:
		return id, nil
	default:
		return arr, ErrType
	}
}

type filter struct {
	expr  Expr
	check Expr
}

func (f filter) Find(node xml.Node) (Sequence, error) {
	return f.find(defaultContext(node))
}

func (f filter) MatchPriority() int {
	return getPriority(prioHigh, f.expr, f.check)
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

func (e Let) Find(node xml.Node) (Sequence, error) {
	return e.find(defaultContext(node))
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

func (e let) Find(node xml.Node) (Sequence, error) {
	return e.find(defaultContext(node))
}

func (e let) MatchPriority() int {
	return prioLow
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

func (o loop) MatchPriority() int {
	return prioLow
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

func (c conditional) MatchPriority() int {
	return getPriority(prioHigh, c.test)
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

func (q quantified) MatchPriority() int {
	return getPriority(prioHigh, q.test)
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

func (v value) MatchPriority() int {
	return prioLow
}

func (v value) find(ctx Context) (Sequence, error) {
	return slices.Clone(v.seq), nil
}

func (v value) Type() XdmType {
	fmt.Printf("%+v: %[1]T\n", v.seq[0])
	return xsUntyped
}

type array struct {
	all []Expr
}

func (a array) Find(node xml.Node) (Sequence, error) {
	return a.find(defaultContext(node))
}

func (a array) MatchPriority() int {
	return prioLow
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

func (i instanceof) MatchPriority() int {
	return getPriority(prioLow, i.expr)
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

func (c cast) MatchPriority() int {
	return getPriority(prioLow, c.expr)
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

func (c castable) MatchPriority() int {
	return getPriority(prioLow, c.expr)
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

func getPriority(base int, values ...Expr) int {
	for i := range values {
		base += values[i].MatchPriority()
	}
	return base
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
