package xml

import (
	"iter"
	"slices"
)

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
