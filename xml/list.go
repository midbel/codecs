package xml

import (
	"fmt"
)

type Environ interface {
	Resolve(string) (Expr, error)
	Define(string, Expr)
}

type Env struct {
	values map[string]Expr
	parent Environ
}

func Empty() Environ {
	return Enclosed(nil)
}

func Enclosed(parent Environ) Environ {
	e := Env{
		values: make(map[string]Expr),
		parent: parent,
	}
	return &e
}

func (e *Env) Define(ident string, expr Expr) {
	e.values[ident] = expr
}

func (e *Env) Resolve(ident string) (Expr, error) {
	expr, ok := e.values[ident]
	if ok {
		return expr, nil
	}
	if e.parent != nil {
		return e.parent.Resolve(ident)
	}
	return nil, fmt.Errorf("%s: identifier not defined", ident)
}

type Item interface {
	Node() Node
	Value() any
	Atomic() bool
	Assert(Expr, Environ) ([]Item, error)
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

func (i literalItem) Assert(_ Expr, _ Environ) ([]Item, error) {
	return nil, fmt.Errorf("can not assert on literal item")
}

func (i literalItem) Atomic() bool {
	return true
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

func (i nodeItem) Assert(expr Expr, env Environ) ([]Item, error) {
	return expr.Next(i.node, env)
}

func (i nodeItem) Atomic() bool {
	return false
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
