package xml

import (
	"fmt"
	"strings"
	"time"
)

type Item interface {
	Node() Node
	Value() any
	True() bool
	Atomic() bool
}

type Sequence []Item

func NewSequence() Sequence {
	var seq Sequence
	return seq
}

func (s *Sequence) Append(item Item) {
	*s = append(*s, item)
}

func (s *Sequence) True() bool {
	if len(*s) == 0 {
		return false
	}
	if len(*s) > 1 {
		return (*s)[0].True()
	}
	return false
}

func (s *Sequence) Empty() bool {
	return len(*s) == 0
}

func (s *Sequence) Singleton() bool {
	return len(*s) == 1
}

func (s *Sequence) String() string {
	return ""
}

func (s *Sequence) Every(test func(i Item) bool) bool {
	for i := range *s {
		if !test((*s)[i]) {
			return false
		}
	}
	return true
}

func createSingle(item Item) Sequence {
	var list []Item
	return append(list, item)
}

func singleValue(value any) Sequence {
	literal := createLiteral(value)
	return createSingle(literal)
}

func singleNode(value Node) Sequence {
	node := createNode(value)
	return createSingle(node)
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
