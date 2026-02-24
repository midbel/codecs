package xml

import (
	"errors"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"maps"
	"sort"
	"strings"
)

type CmpMode int

const (
	CmpOrdered CmpMode = iota
	CmpUnordered
)

type CmpResult struct {
	Source Node
	Target Node
	Match  bool
}

var ErrCompare = errors.New("documents mismatched")

func Compare(source, target string, mode CmpMode) (CmpResult, error) {
	doc1, err := buildHash(source, mode)
	if err != nil {
		var res CmpResult
		return res, err
	}
	doc2, err := buildHash(target, mode)
	if err != nil {
		var res CmpResult
		return res, err
	}
	res := doc1.Compare(doc2, mode)
	if !res.Match {
		err = ErrCompare
	}
	return res, err

}

func buildHash(file string, mode CmpMode) (*hashNode, error) {
	doc, err := ParseFile(file)
	if err != nil {
		return nil, err
	}
	return buildHashTree(doc.Root(), mode), nil
}

type hashNode struct {
	Node
	orderedHash   uint64
	unorderedHash uint64
	children      map[uint64]*hashNode
}

func (n *hashNode) Compare(other *hashNode, mode CmpMode) CmpResult {
	res := CmpResult{
		Source: n.Node,
		Target: other.Node,
	}
	if len(other.children) != len(n.children) {
		return res
	}
	values := maps.Clone(other.children)
	for k, v := range n.children {
		x, ok := values[k]
		if !ok {
			res.Source = v.Node
			res.Target = other.Node
			break
		}
		if res := v.Compare(x, mode); !res.Match {
			break
		}
		delete(values, k)
	}
	if mode == CmpUnordered {
		res.Match = n.unorderedHash == other.unorderedHash
	} else {
		res.Match = n.orderedHash == other.orderedHash
	}
	return res
}

func buildHashTree(root Node, mode CmpMode) *hashNode {
	node := hashNode{
		Node:     root,
		children: make(map[uint64]*hashNode),
	}
	if root.Leaf() {
		node.orderedHash = computeHashForNode(root)
		node.unorderedHash = node.orderedHash
		return &node
	}
	if elem, ok := root.(*Element); ok {
		var (
			orderedHash   []uint64
			unorderedHash []uint64
		)
		for _, el := range elem.Nodes {
			h := buildHashTree(el, mode)

			unorderedHash = append(unorderedHash, h.unorderedHash)
			orderedHash = append(orderedHash, h.orderedHash)

			if mode == CmpOrdered {
				node.children[h.unorderedHash] = h
			} else {
				node.children[h.orderedHash] = h
			}
		}
		sort.Slice(unorderedHash, func(i, j int) bool {
			return unorderedHash[i] < unorderedHash[j]
		})

		node.unorderedHash = computeHash(unorderedHash)
		node.orderedHash = computeHash(orderedHash)
	}
	return &node
}

func computeHash(values []uint64) uint64 {
	var (
		sum = fnv.New64a()
		buf = make([]byte, 8)
	)
	for i := range values {
		binary.LittleEndian.PutUint64(buf, values[i])
		sum.Write(buf)
	}
	return sum.Sum64()
}

func computeHashForNode(root Node) uint64 {
	switch n := root.(type) {
	case *Element:
		var values []uint64

		values = append(values, getHashForText(n.QName.QualifiedName()))
		for _, a := range n.Attrs {
			v := computeHashForNode(&a)
			values = append(values, v)
		}
		if n.Leaf() && len(n.Nodes) > 0 {
			values = append(values, computeHashForNode(n.Nodes[0]))
		}
		return computeHash(values)
	case *Instruction:
		var values []uint64

		values = append(values, getHashForText(n.QName.QualifiedName()))
		for _, a := range n.Attrs {
			v := computeHashForNode(&a)
			values = append(values, v)
		}
		return computeHash(values)
	case *Attribute:
		str := fmt.Sprintf("%s = %s", n.QName.QualifiedName(), n.Value())
		return getHashForText(str)
	case *Comment:
		str := fmt.Sprintf("<!-- %s -- >", n.Content)
		return getHashForText(str)
	case *Text:
		return getHashForText(n.Content)
	case *CharData:
		return getHashForText(n.Content)
	default:
	}
	return 0
}

func getHashForText(str string) uint64 {
	str = strings.TrimSpace(str)
	s := fnv.New64a()
	s.Write([]byte(str))
	return s.Sum64()
}
