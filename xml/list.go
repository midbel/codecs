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

type NodeList struct {
	nodes []Node
}

func createList() *NodeList {
	return &NodeList{}
}

func (n *NodeList) Nodes() []Node {
	tmp := slices.Clone(n.nodes)
	return tmp
}

func (n *NodeList) Merge(other *NodeList) {
	n.nodes = slices.Concat(n.nodes, other.nodes)
}

func (n *NodeList) Push(node Node) {
	n.nodes = append(n.nodes, node)
}

func (n *NodeList) Empty() bool {
	return len(n.nodes) == 0
}

func (n *NodeList) Len() int {
	return len(n.nodes)
}

func (n *NodeList) All() iter.Seq[Node] {
	do := func(yield func(Node) bool) {
		for _, n := range n.nodes {
			if !yield(n) {
				break
			}
		}
	}
	return do
}

func (n *NodeList) Values() []any {
	var list []any
	for i := range n.All() {
		list = append(list, i.Value())
	}
	return list
}
