package xml

import (
	"iter"
	"slices"
)

type ResultItem interface {
	Node() Node
	Value() any
}

type literalItem struct {
	value any
}

func createLiteral(value any) ResultItem {
	return literalItem{
		value: value,
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

func createNode(node Node) ResultItem {
	return nodeItem{
		node: node,
	}
}

func (i nodeItem) Node() Node {
	return i.node
}

func (i nodeItem) Value() any {
	var traverse func(Node) any
	traverse = func(n Node) any {
		el, ok := n.(*Element)
		if !ok {
			return n.Value()
		}
		var arr []any
		for _, n := range el.Nodes {
			if n.Leaf() {
				arr = append(arr, n.Value())
				continue
			}
			arr = append(arr, traverse(n))
		}
		return arr
	}
	return traverse(i.node)
}

type ResultList struct {
	items []ResultItem
}

func createList() *ResultList {
	return &ResultList{}
}

func (r *ResultList) Items() []ResultItem {
	tmp := slices.Clone(r.items)
	return tmp
}

func (r *ResultList) Merge(other *ResultList) {
	r.items = slices.Concat(r.items, other.items)
}

func (r *ResultList) Push(node Node) {
	r.items = append(r.items, createNode(node))
}

func (r *ResultList) Empty() bool {
	return len(r.items) == 0
}

func (r *ResultList) Len() int {
	return len(r.items)
}

func (r *ResultList) Nodes() iter.Seq[Node] {
	do := func(yield func(Node) bool) {
		for _, i := range r.items {
			if !yield(i.Node()) {
				break
			}
		}
	}
	return do
}

func (r *ResultList) All() iter.Seq[ResultItem] {
	do := func(yield func(ResultItem) bool) {
		for _, i := range r.items {
			if !yield(i) {
				break
			}
		}
	}
	return do
}

func (r *ResultList) Values() []any {
	var list []any
	for i := range n.All() {
		list = append(list, i.Value())
	}
	return list
}
