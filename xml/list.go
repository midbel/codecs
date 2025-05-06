package xml

import (
	"fmt"
	"maps"
	"strings"
	"time"
)

type Environ[T any] interface {
	Resolve(string) (T, error)
	Define(string, T)
}

type Env[T any] struct {
	values map[string]T
	parent Environ[T]
}

func Empty[T any]() Environ[T] {
	return Enclosed[T](nil)
}

func Enclosed[T any](parent Environ[T]) Environ[T] {
	e := Env[T]{
		values: make(map[string]T),
		parent: parent,
	}
	return &e
}

func (e *Env[T]) Define(ident string, expr T) {
	e.values[ident] = expr
}

func (e *Env[T]) Resolve(ident string) (T, error) {
	expr, ok := e.values[ident]
	if ok {
		return expr, nil
	}
	if e.parent != nil {
		return e.parent.Resolve(ident)
	}
	var t T
	return t, fmt.Errorf("%s: identifier not defined", ident)
}

func (e *Env[T]) Unwrap() Environ[T] {
	if e.parent == nil {
		return e
	}
	return e.parent
}

func (e *Env[T]) Merge(other Environ[T]) {
	x, ok := other.(*Env[T])
	if !ok {
		return
	}
	maps.Copy(e.values, x.values)
}

func (e *Env[T]) Clone() Environ[T] {
	var x Env[T]
	x.values = make(map[string]T)
	maps.Copy(x.values, e.values)

	if c, ok := e.parent.(interface{ Clone() Environ[T] }); ok {
		x.parent = c.Clone()
	}
	return &x
}

type Item interface {
	Node() Node
	Value() any
	True() bool
	Atomic() bool
}

func createSingle(item Item) []Item {
	var list []Item
	return append(list, item)
}

func singleValue(value any) []Item {
	literal := createLiteral(value)
	return createSingle(literal)
}

func singleNode(value Node) []Item {
	node := createNode(value)
	return createSingle(node)
}

func isSingleton(list []Item) bool {
	return len(list) == 1
}

func isEmpty(list []Item) bool {
	return len(list) == 0
}

func isTrue(list []Item) bool {
	return len(list) > 0 && list[0].True()
}

func atomicItem(item Item) (Item, error) {
	if item.Atomic() {
		return item, nil
	}
	n, ok := item.(nodeItem)
	if !ok || !n.Node().Leaf() {
		return nil, fmt.Errorf("item can not be converted to literal")
	}
	return createLiteral(n.Value()), nil
}

type literalItem struct {
	value any
}

func NewLiteralItem(value any) Item {
	return createLiteral(value)
}

func createLiteral(value any) Item {
	return literalItem{
		value: value,
	}
}

func (i literalItem) Assert(_ Expr, _ Environ[Expr]) ([]Item, error) {
	return nil, fmt.Errorf("can not assert on literal item")
}

func (i literalItem) Atomic() bool {
	return true
}

func (i literalItem) True() bool {
	switch v := i.value.(type) {
	case []byte:
		return len(v) != 0
	case float64:
		return v != 0
	case string:
		return v != ""
	case bool:
		return v
	case time.Time:
		return !v.IsZero()
	default:
		return false
	}
}

func (i literalItem) Node() Node {
	var (
		qn = QName{
			Name: "literal",
		}
		res = NewElement(qn)
	)
	str, _ := toString(i.value)
	res.Append(NewText(str))
	return res
}

func (i literalItem) Value() any {
	return i.value
}

type nodeItem struct {
	node Node
}

func NewNodeItem(node Node) Item {
	return createNode(node)
}

func createNode(node Node) Item {
	return nodeItem{
		node: node,
	}
}

func (i nodeItem) Assert(expr Expr, env Environ[Expr]) ([]Item, error) {
	ctx := createContext(i.node, 1, 1)
	ctx.Environ = env
	if ctx.Builtins == nil {
		ctx.Builtins = DefaultBuiltin()
	}
	return expr.find(ctx)
}

func (i nodeItem) Atomic() bool {
	return false
}

func (i nodeItem) Node() Node {
	return i.node
}

func (i nodeItem) True() bool {
	return true
}

func (i nodeItem) Value() any {
	if i.node.Type() == TypeAttribute {
		return i.node.Value()
	}
	var traverse func(Node) []string
	traverse = func(n Node) []string {
		el, ok := n.(*Element)
		if !ok {
			return []string{n.Value()}
		}
		var arr []string
		for _, n := range el.Nodes {
			if n.Leaf() {
				arr = append(arr, n.Value())
				continue
			}
			arr = append(arr, traverse(n)...)
		}
		return arr
	}
	str := traverse(i.node)
	return strings.Join(str, "")
}

func isFloat(i Item) bool {
	_, ok := i.Value().(float64)
	return ok
}

func every(items []Item, test func(i Item) bool) bool {
	for i := range items {
		if !test(items[i]) {
			return false
		}
	}
	return true
}

func convert[T string | float64](items []Item, do func(any) (T, error)) ([]T, error) {
	var list []T
	for i := range items {
		x, err := do(items[i].Value())
		if err != nil {
			return nil, err
		}
		list = append(list, x)
	}
	return list, nil
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

type Sequence []Item

func (s Sequence) IsTrue() bool {
	return false
}

func (s Sequence) Join() string {
	return ""
}
