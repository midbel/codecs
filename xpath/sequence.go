package xpath

import (
	"fmt"
	"math"
	"slices"
	"strconv"
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
	} else if i, ok := value.(literalItem); ok {
		item = i
	} else if i, ok := value.(nodeItem); ok {
		item = i
	} else {
		if vs, ok := value.([]Item); ok {
			item = createArray(vs)
		} else {
			item = createLiteral(value)
		}
	}
	var seq Sequence
	seq.Append(item)
	return seq
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
	return EffectiveBooleanValue(*s)
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

func (s *Sequence) Compare(other *Sequence) int {
	if s.Len() < other.Len() {
		return -1
	}
	if s.Len() > other.Len() {
		return 1
	}
	var res int
	for i := range *s {
		v1, v2 := (*s)[i], (*other)[i]
		switch v1 := v1.Value().(type) {
		case string:
			s, ok := v2.Value().(string)
			if !ok {
				break
			}
			res += strings.Compare(v1, s)
		case float64:
			s, ok := v2.Value().(float64)
			if !ok {
				break
			}
			res += int(v1 - s)
		case int64:
			s, ok := v2.Value().(int64)
			if !ok {
				break
			}
			res += int(v1 - s)
		case time.Time:
		case bool:
			s, ok := v2.Value().(bool)
			if !ok {
				break
			}
			_ = s
		default:
		}
	}
	return res
}

func (s *Sequence) CanonicalizeString() string {
	if s.Empty() {
		return ("seq()")
	}
	var str strings.Builder
	str.WriteString("seq(")
	for i := range *s {
		switch x := (*s)[i].Value().(type) {
		case xml.Node:
			str.WriteString("node(")
			str.WriteString(x.Identity())
			str.WriteString(")")
		case Sequence:
			str.WriteString(x.CanonicalizeString())
		case string:
			str.WriteString("str(")
			str.WriteString(x)
			str.WriteString(")")
		case float64:
			str.WriteString("float(")
			str.WriteString(strconv.FormatFloat(x, 'f', -1, 64))
			str.WriteString(")")
		case bool:
			str.WriteString("bool(")
			str.WriteString(strconv.FormatBool(x))
			str.WriteString(")")
		default:
		}
	}
	str.WriteString(")")
	return str.String()
}

func EffectiveBooleanValue(seq Sequence) bool {
	if seq.Empty() {
		return false
	}
	if seq.Singleton() {
		if !seq[0].Atomic() {
			return true
		}
		switch x := seq[0].Value().(type) {
		case string:
			return x != ""
		case float64:
			return x != 0 && !math.IsNaN(x)
		case bool:
			return x
		default:
			return false
		}
	}
	for i := range seq {
		if !seq[i].Atomic() {
			return true
		}
	}
	return false
}

func createSingle(item Item) Sequence {
	var list []Item
	return append(list, item)
}

func isTrue(list Sequence) bool {
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
	if i, ok := value.(literalItem); ok {
		return i
	}
	return literalItem{
		value: value,
	}
}

func (i literalItem) Sequence() Sequence {
	return Singleton(i)
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
	case int64:
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

type arrayItem struct {
	values []Item
}

func createArray(vs []Item) Item {
	return arrayItem{
		values: slices.Clone(vs),
	}
}

func (i arrayItem) Sequence() Sequence {
	s := NewSequence()
	for j := range i.values {
		s.Append(i.values[j])
	}
	return s
}

func (i arrayItem) Node() xml.Node {
	return nil
}

func (i arrayItem) Value() any {
	var list []any
	for j := range i.values {
		list = append(list, i.values[j].Value())
	}
	return list
}

func (i arrayItem) True() bool {
	if len(i.values) == 0 {
		return false
	}
	for j := range i.values {
		if !i.values[j].True() {
			return false
		}
	}
	return true
}

func (i arrayItem) Atomic() bool {
	return false
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

func (i nodeItem) Sequence() Sequence {
	return Singleton(i)
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
