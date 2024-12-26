package xml

type Item interface {
	Node() Node
	Value() any
	// Type() string
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

type literalItem struct {
	value any
}

func createLiteral(value any) Item {
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

func createNode(node Node) Item {
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
