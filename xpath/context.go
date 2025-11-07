package xpath

import (
	"slices"
	"time"

	"github.com/midbel/codecs/environ"
	"github.com/midbel/codecs/xml"
)

type Context struct {
	xml.Node
	Index         int
	Size          int
	PrincipalType xml.NodeType

	environ.Environ[Expr]
	Builtins environ.Environ[BuiltinFunc]
	Now      time.Time
}

func defaultContext(node xml.Node) Context {
	ctx := createContext(node, 1, 1)
	ctx.Environ = environ.Empty[Expr]()
	return ctx
}

func createContext(node xml.Node, pos, size int) Context {
	return Context{
		Node:     node,
		Index:    pos,
		Size:     size,
		ENviron: 
		Builtins: DefaultBuiltin(),
		Now:      time.Now(),
	}
}

func (c Context) Nest() Context {
	ctx := createContext(c.Node, c.Index, c.Size)
	ctx.Environ = environ.Enclosed(c.Environ)
	ctx.PrincipalType = c.PrincipalType
	return ctx
}

func (c Context) Sub(node xml.Node, pos int, size int) Context {
	ctx := createContext(node, pos, size)
	ctx.Environ = environ.Enclosed(c)
	ctx.PrincipalType = c.PrincipalType
	return ctx
}

func (c Context) Root() Context {
	if c.Node == nil {
		return c
	}
	curr := c.Node
	for {
		root := curr.Parent()
		if root == nil {
			break
		}
		curr = root
	}
	return c.Sub(curr, 1, 1)
}

func (c Context) Nodes() []xml.Node {
	return getNodes(c.Node)
}

func (c *Context) UriCollection(uri string) (Sequence, error) {
	return nil, nil
}

func (c *Context) DefaultUriCollection() Sequence {
	return nil
}

func getNodes(parent xml.Node) []xml.Node {
	var nodes []xml.Node
	if parent.Type() == xml.TypeDocument {
		doc := parent.(*xml.Document)
		nodes = append(nodes, doc.Root())
	} else if parent.Type() == xml.TypeElement {
		el := parent.(*xml.Element)
		nodes = slices.Clone(el.Nodes)
	}
	return nodes
}
