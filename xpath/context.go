package xpath

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"

	"github.com/midbel/codecs/environ"
	"github.com/midbel/codecs/xml"
)

type DecimalFormatter struct {
	GroupSep    rune
	DecimalSep  rune
	ExponentSep rune

	PercentChar rune
}

func (f DecimalFormatter) Format(value float64) float64 {
	return value
}

type Engine struct {
	baseURI   url.URL
	defaultNS map[string]url.URL
	funcNS    url.URL
	elemNS    url.URL
	variables environ.Environ[Expr]
	builtins  environ.Environ[BuiltinFunc]
}

func NewEngine() *Engine {
	e := Engine{
		defaultNS: make(map[string]url.URL),
		variables: environ.Empty[Expr](),
		builtins:  DefaultBuiltin(),
	}
	return &e
}

func (e *Engine) Eval(query string) (Sequence, error) {
	return nil, nil
}

type Context struct {
	xml.Node
	Index         int
	Size          int
	BaseURI       string
	PrincipalType xml.NodeType

	environ.Environ[Expr]
	Builtins environ.Environ[BuiltinFunc]
}

func DefaultContext(node xml.Node) Context {
	ctx := createContext(node, 1, 1)
	ctx.Environ = environ.Empty[Expr]()
	return ctx
}

func createContext(node xml.Node, pos, size int) Context {
	return Context{
		Node:     node,
		Index:    pos,
		Size:     size,
		Builtins: DefaultBuiltin(),
	}
}

func (c Context) UriCollection(uri string) (Sequence, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" && u.Host == "" {
		b, _ := url.Parse(c.BaseURI)
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

func (c Context) DefaultUriCollection() Sequence {
	var s Sequence
	return s
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
