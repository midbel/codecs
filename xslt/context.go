package xslt

import (
	"github.com/midbel/codecs/xml"
)

type Context struct {
	CurrentNode xml.Node
	Index       int
	Size        int
	Mode        string

	*Stylesheet
	*Env
}

func (c *Context) Self() *Context {
	return c.Sub(c.CurrentNode)
}

func (c *Context) Sub(node xml.Node) *Context {
	child := Context{
		CurrentNode: node,
		Index:       1,
		Size:        1,
		Stylesheet:  c.Stylesheet,
		Env:         c.Env.Sub(),
	}
	return &child
}

func (c *Context) Execute(query string, node xml.Node) (xml.Sequence, error) {
	return c.Env.Execute(query, node)
}

func (c *Context) NotFound(node xml.Node, err error, mode string) error {
	var tmp xml.Node
	switch mode := c.getMode(mode); mode.NoMatch {
	case MatchDeepCopy:
		tmp = cloneNode(c.CurrentNode)
	case MatchShallowCopy:
		qn, err := xml.ParseName(c.CurrentNode.QualifiedName())
		if err != nil {
			return err
		}
		tmp = xml.NewElement(qn)
		if el, ok := c.CurrentNode.(*xml.Element); ok {
			a := tmp.(*xml.Element)
			for i := range el.Attrs {
				a.SetAttribute(el.Attrs[i])
			}
			tmp = a
		}
	case MatchTextOnlyCopy:
		tmp = xml.NewText(c.CurrentNode.Value())
	case MatchFail:
		return err
	default:
		return err
	}
	return replaceNode(node, tmp)
}

type Resolver interface {
	Resolve(string) (xml.Expr, error)
}

type Env struct {
	other     Resolver
	Namespace string
	Vars      xml.Environ[xml.Expr]
	Params    xml.Environ[xml.Expr]
	Builtins  xml.Environ[xml.BuiltinFunc]
}

func Empty() *Env {
	return Enclosed(nil)
}

func Enclosed(other Resolver) *Env {
	return &Env{
		other:    other,
		Vars:     xml.Empty[xml.Expr](),
		Params:   xml.Empty[xml.Expr](),
		Builtins: xml.DefaultBuiltin(),
	}
}

func (e *Env) Sub() *Env {
	return &Env{
		other:     e.other,
		Namespace: e.Namespace,
		Vars:      xml.Enclosed[xml.Expr](e.Vars),
		Params:    xml.Enclosed[xml.Expr](e.Params),
		Builtins:  e.Builtins,
	}
}

func (e *Env) Merge(other *Env) {
	if m, ok := e.Vars.(interface{ Merge(xml.Environ[xml.Expr]) }); ok {
		m.Merge(other.Vars)
	}
	if m, ok := e.Params.(interface{ Merge(xml.Environ[xml.Expr]) }); ok {
		m.Merge(other.Params)
	}
}

func (e *Env) Execute(query string, datum xml.Node) (xml.Sequence, error) {
	return e.ExecuteNS(query, "", datum)
}

func (e *Env) ExecuteNS(query, namespace string, datum xml.Node) (xml.Sequence, error) {
	if query == "" {
		i := xml.NewNodeItem(datum)
		return []xml.Item{i}, nil
	}
	q, err := e.CompileNS(query, namespace)
	if err != nil {
		return nil, err
	}
	return q.Find(datum)
}

func (e *Env) Query(query string, datum xml.Node) (xml.Sequence, error) {
	return e.ExecuteNS(query, e.Namespace, datum)
}

func (e *Env) Compile(query string) (xml.Expr, error) {
	return e.CompileNS(query, "")
}

func (e *Env) CompileNS(query, namespace string) (xml.Expr, error) {
	q, err := xml.Build(query)
	if err != nil {
		return nil, err
	}
	q.Environ = e
	q.Builtins = e.Builtins
	if namespace != "" {
		q.UseNamespace(namespace)
	}
	return q, nil
}

func (e *Env) Test(query string, datum xml.Node) (bool, error) {
	items, err := e.Execute(query, datum)
	if err != nil {
		return false, err
	}
	return isTrue(items), nil
}

func (e *Env) Resolve(ident string) (xml.Expr, error) {
	expr, err := e.Vars.Resolve(ident)
	if err == nil {
		return expr, nil
	}
	expr, err = e.Params.Resolve(ident)
	if err == nil {
		return expr, nil
	}
	if e.other != nil {
		return e.other.Resolve(ident)
	}
	return nil, err
}

func (e *Env) Define(param string, expr xml.Expr) {
	e.Vars.Define(param, expr)
}

func (e *Env) DefineParam(param, value string) error {
	expr, err := e.Compile(value)
	if err == nil {
		e.DefineExprParam(param, expr)
	}
	return err
}

func (e *Env) DefineExprParam(param string, expr xml.Expr) {
	e.Params.Define(param, expr)
}

func (e *Env) EvalParam(param, query string, datum xml.Node) error {
	items, err := e.Execute(query, datum)
	if err == nil {
		e.DefineExprParam(param, xml.NewValueFromSequence(items))
	}
	return err
}

func isTrue(seq xml.Sequence) bool {
	if seq.Empty() {
		return false
	}
	first, ok := seq.First()
	if !first.Atomic() {
		return true
	}
	switch res := first.Value().(type) {
	case bool:
		ok = res
	case float64:
		ok = res != 0
	case string:
		ok = res != ""
	default:
	}
	return ok
}
