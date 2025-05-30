package xpath

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/midbel/codecs/environ"
	"github.com/midbel/codecs/xml"
)

type Item interface {
	Node() xml.Node
	Value() any
	True() bool
	Atomic() bool
}

type Sequence []Item

func NewSequence() Sequence {
	var seq Sequence
	return seq
}

func Singleton(value any) Sequence {
	var item Item
	if n, ok := value.(xml.Node); ok {
		item = createNode(n)
	} else {
		item = createLiteral(value)
	}
	var seq Sequence
	seq.Append(item)
	return seq
}

func (s *Sequence) First() (Item, bool) {
	if s.Empty() {
		return nil, false
	}
	return (*s)[0], true
}

func (s *Sequence) Len() int {
	return len(*s)
}

func (s *Sequence) Append(item Item) {
	*s = append(*s, item)
}

func (s *Sequence) Concat(other Sequence) {
	*s = slices.Concat(*s, other)
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

func (i literalItem) Node() xml.Node {
	str, _ := toString(i.value)
	return xml.NewText(str)
}

func (i literalItem) Value() any {
	return i.value
}

func (i literalItem) Assert(_ Expr, _ environ.Environ[Expr]) (Sequence, error) {
	return nil, fmt.Errorf("can not assert on literal item")
}

type nodeItem struct {
	node xml.Node
}

func NewNodeItem(node xml.Node) Item {
	return createNode(node)
}

func createNode(node xml.Node) Item {
	return nodeItem{
		node: node,
	}
}

func (i nodeItem) Atomic() bool {
	return false
}

func (i nodeItem) Node() xml.Node {
	return i.node
}

func (i nodeItem) True() bool {
	return true
}

func (i nodeItem) Value() any {
	if i.node.Type() == xml.TypeAttribute {
		return i.node.Value()
	}
	var traverse func(xml.Node) []string
	traverse = func(n xml.Node) []string {
		el, ok := n.(*xml.Element)
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

func (i nodeItem) Assert(expr Expr, env environ.Environ[Expr]) (Sequence, error) {
	ctx := createContext(i.node, 1, 1)
	ctx.Environ = env
	if ctx.Builtins == nil {
		ctx.Builtins = DefaultBuiltin()
	}
	return expr.find(ctx)
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
