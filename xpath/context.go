package xpath

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/midbel/codecs/environ"
	"github.com/midbel/codecs/xml"
)

type static struct {
	namespaces environ.Environ[string]
	variables  environ.Environ[Expr]
	baseURI    string
	elementNS  string
	typeNS     string
	funcNS     string
	enforceNS  bool
}

func createStatic() *static {
	var (
		ns = environ.Empty[string]()
		vs = environ.Empty[Expr]()
	)
	return &static{
		namespaces: ns,
		variables:  vs,
		funcNS:     functionNS,
		typeNS:     schemaNS,
	}
}

func (c static) Readonly() *static {
	return &static{
		namespaces: environ.ReadOnly(c.namespaces),
		variables:  environ.ReadOnly(c.variables),
		baseURI:    c.baseURI,
		elementNS:  c.elementNS,
		typeNS:     c.typeNS,
		funcNS:     c.funcNS,
		enforceNS:  c.enforceNS,
	}
}

func (c static) UriCollection(uri string) (Sequence, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" && u.Host == "" {
		b, _ := url.Parse(c.baseURI)
		u.Scheme = b.Scheme
		u.Host = b.Host
	}
	switch u.Scheme {
	case "file", "http", "":
		es, err := os.ReadDir(u.Path)
		if err != nil {
			return nil, err
		}
		var s Sequence
		for i := range es {
			s.Append(NewLiteralItem(filepath.Join(u.Path, es[i].Name())))
		}
		return s, nil
	default:
		return nil, fmt.Errorf("unsupported uri scheme %s", u.Scheme)
	}
}

func (c static) DefaultUriCollection() Sequence {
	var s Sequence
	return s
}

type Context struct {
	*static

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
	ctx.static = createStatic()
	ctx.Environ = environ.Empty[Expr]()
	return ctx
}

func createContext(node xml.Node, pos, size int) Context {
	return Context{
		Node:     node,
		Index:    pos,
		Size:     size,
		Builtins: DefaultBuiltin(),
		Now:      time.Now(),
	}
}

func (c Context) Resolve(ident string) (Expr, error) {
	e, err := c.Environ.Resolve(ident)
	if err == nil {
		return e, err
	}
	return c.variables.Resolve(ident)
}

func (c Context) Nest() Context {
	ctx := createContext(c.Node, c.Index, c.Size)
	ctx.static = c.static
	ctx.Environ = environ.Enclosed(c.Environ)
	ctx.PrincipalType = c.PrincipalType
	return ctx
}

func (c Context) Sub(node xml.Node, pos int, size int) Context {
	ctx := createContext(node, pos, size)
	ctx.static = c.static
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
