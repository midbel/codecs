package xslt

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/midbel/codecs/xml"
)

const (
	xsltNamespaceUri    = "http://www.w3.org/1999/XSL/Transform"
	xsltNamespacePrefix = "xsl"
)

var (
	errImplemented = errors.New("not implemented")
	errUndefined   = errors.New("undefined")
	errSkip        = errors.New("skip")
	ErrTerminate   = errors.New("terminate")
)

type Output struct {
	Name       string
	Method     string
	Encoding   string
	Version    string
	Indent     bool
	OmitProlog bool
}

func defaultOutput() *Output {
	out := &Output{
		Method:   "xml",
		Version:  "1.0",
		Encoding: "UTF-8",
		Indent:   false,
	}
	return out
}

type MatchMode int8

const (
	MatchDeepCopy MatchMode = 1 << iota
	MatchShallowCopy
	MatchDeepSkip
	MatchShallowSkip
	MatchTextOnlyCopy
	MatchFail
)

type Mode struct {
	Name       string
	NoMatch    MatchMode
	MultiMatch MatchMode
}

func unnamedMode() *Mode {
	return &Mode{
		NoMatch:    MatchFail,
		MultiMatch: MatchFail,
	}
}

func (m Mode) Unnamed() bool {
	return m.Name == ""
}

type AttributeSet struct {
	Name  string
	Attrs []xml.Attribute
}

type Stylesheet struct {
	DefaultMode string

	namespace   string
	Mode        string
	currentMode *Mode
	Modes       []*Mode
	AttrSet     []*AttributeSet

	output    []*Output
	Templates []*Template
	*Env
	Tracer

	Context string
	Others  []*Stylesheet
}

func Load(file, contextDir string) (*Stylesheet, error) {
	doc, err := loadDocument(file)
	if err != nil {
		return nil, err
	}
	sheet := Stylesheet{
		Context:     contextDir,
		currentMode: unnamedMode(),
		namespace:   xsltNamespacePrefix,
		Env:         Empty(),
	}
	if sheet.Context == "" {
		sheet.Context = filepath.Dir(file)
	}

	root := doc.Root().(*xml.Element)
	if root != nil {
		all := root.Namespaces()
		ix := slices.IndexFunc(all, func(n xml.NS) bool {
			return n.Uri == xsltNamespaceUri
		})
		if ix >= 0 {
			sheet.Env.Namespace = all[ix].Prefix
			sheet.namespace = all[ix].Prefix
			for e, fn := range executers {
				delete(executers, e)
				e.Space = sheet.namespace
				executers[e] = fn
			}
		}
	}

	if err := sheet.init(doc); err != nil {
		return nil, err
	}
	return &sheet, nil
}

func (s *Stylesheet) Find(name, mode string) (*Template, error) {
	if mode == "" {
		mode = s.CurrentMode()
	}
	ix := slices.IndexFunc(s.Templates, func(t *Template) bool {
		return t.Name == name && t.Mode == mode
	})
	if ix < 0 {
		for _, s := range s.Others {
			if tpl, err := s.Find(name, mode); err == nil {
				return tpl, nil
			}
		}
		return nil, fmt.Errorf("template %s not found", name)
	}
	tpl := s.Templates[ix].Clone()
	return tpl, nil
}

func (s *Stylesheet) MatchImport(node xml.Node, mode string) (*Template, error) {
	for _, s := range s.Others {
		if tpl, err := s.Match(node, mode); err == nil {
			return tpl, err
		}
	}
	return nil, fmt.Errorf("no template found matching given node (%s)", node.QualifiedName())
}

func (s *Stylesheet) Match(node xml.Node, mode string) (*Template, error) {
	tpl, err := s.getMatchingTemplates(s.Templates, node, mode)
	if err == nil {
		return tpl, nil
	}
	return s.MatchImport(node, mode)
}

func (s *Stylesheet) Generate(w io.Writer, doc *xml.Document) error {
	result, err := s.Execute(doc)
	if err != nil {
		return err
	}
	return s.writeDocument(w, "", result.(*xml.Document))
}

func (s *Stylesheet) Execute(doc xml.Node) (xml.Node, error) {
	tpl, err := s.getMainTemplate()
	if err != nil {
		return nil, err
	}
	nodes, err := tpl.Execute(s.createContext(doc))
	if err == nil {
		var root xml.Node
		if len(nodes) != 1 {
			elem := xml.NewElement(xml.LocalName("angle"))
			for i := range nodes {
				elem.Append(nodes[i])
			}
			root = elem
		} else {
			root = nodes[0]
		}
		return xml.NewDocument(root), nil
	}
	return nil, err
}

func (s *Stylesheet) ImportSheet(file string) error {
	other, err := Load(filepath.Join(s.Context, file), s.Context)
	if err != nil {
		return err
	}
	s.Others = append(s.Others, other)
	return nil
}

func (s *Stylesheet) IncludeSheet(file string) error {
	other, err := Load(filepath.Join(s.Context, file), s.Context)
	if err != nil {
		return err
	}
	s.Templates = append(s.Templates, other.Templates...)
	s.Env.Merge(other.Env)
	return nil
}

func (s *Stylesheet) LoadDocument(file string) (xml.Node, error) {
	file = filepath.Join(s.Context, file)
	return loadDocument(file)
}

func (s *Stylesheet) SetAttributes(node xml.Node) error {
	elem := node.(*xml.Element)
	if elem == nil {
		return nil
	}
	ident, err := getAttribute(elem, "use-attribute-sets")
	if err != nil {
		return nil
	}
	ix := slices.IndexFunc(s.AttrSet, func(set *AttributeSet) bool {
		return set.Name == ident
	})
	if ix < 0 {
		return fmt.Errorf("%s: attribute set not found", ident)
	}
	for _, a := range s.AttrSet[ix].Attrs {
		elem.SetAttribute(a)
	}
	elem.RemoveAttr(elem.Attrs[ix].Position())
	return nil
}

func (s *Stylesheet) SetParam(ident string, expr xml.Expr) {
	s.Env.DefineExprParam(ident, expr)
}

func (s *Stylesheet) GetOutput(name string) *Output {
	ix := slices.IndexFunc(s.output, func(o *Output) bool {
		return o.Name == name
	})
	if ix < 0 && name != "" {
		return defaultOutput()
	}
	return s.output[ix]
}

func (s *Stylesheet) CurrentMode() string {
	return s.Mode
}

func (s *Stylesheet) createContext(node xml.Node) *Context {
	env := Enclosed(s)
	env.Namespace = s.namespace
	ctx := Context{
		ContextNode: node,
		Size:        1,
		Index:       1,
		Stylesheet:  s,
		Env:         env,
	}
	return &ctx
}

func (s *Stylesheet) init(doc xml.Node) error {
	if err := s.loadTemplates(doc); err != nil {
		return err
	}
	if err := s.loadOutput(doc); err != nil {
		return err
	}
	if err := s.loadParams(doc); err != nil {
		return err
	}
	if err := s.loadAttributeSet(doc); err != nil {
		return err
	}
	if err := s.includesSheet(doc); err != nil {
		return err
	}
	if err := s.importsSheet(doc); err != nil {
		return err
	}
	return nil
}

func (s *Stylesheet) includesSheet(doc xml.Node) error {
	items, err := s.queryXSL("/stylesheet/include | /transform/include", doc)
	if err != nil {
		return err
	}
	ctx := s.createContext(nil)
	for _, i := range items {
		_, err := executeInclude(ctx.WithXsl(i.Node()))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Stylesheet) importsSheet(doc xml.Node) error {
	items, err := s.queryXSL("/stylesheet/import | /transform/import", doc)
	if err != nil {
		return err
	}
	ctx := s.createContext(nil)
	for _, i := range items {
		_, err := executeImport(ctx.WithXsl(i.Node()))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Stylesheet) loadAttributeSet(doc xml.Node) error {
	items, err := s.queryXSL("/stylesheet/attribute-set | /transform/attribute-set", doc)
	if err != nil {
		return err
	}
	for i := range items {
		n := items[i].Node().(*xml.Element)
		if n == nil {
			continue
		}
		ident, err := getAttribute(n, "name")
		if err != nil {
			return err
		}
		as := AttributeSet{
			Name: ident,
		}
		for _, n := range n.Nodes {
			n := n.(*xml.Element)
			if n == nil {
				continue
			}
			ident, err := getAttribute(n, "name")
			if err != nil {
				return err
			}
			attr := xml.NewAttribute(xml.LocalName(ident), n.Value())
			as.Attrs = append(as.Attrs, attr)
		}
		s.AttrSet = append(s.AttrSet, &as)
	}
	return nil
}

func (s *Stylesheet) loadModes(doc xml.Node) error {
	items, err := s.queryXSL("/stylesheet/mode | /transform/mode", doc)
	if err != nil {
		return err
	}
	for i := range items {
		n := items[i].Node().(*xml.Element)
		if n == nil {
			continue
		}
		var m Mode
		for _, a := range n.Attrs {
			switch attr := a.Value(); a.Name {
			case "name":
				m.Name = attr
			case "on-no-match":
				m.NoMatch = MatchFail
			case "on-multiple-match":
				m.MultiMatch = MatchFail
			case "warning-on-no-match":
				// TODO
			case "warning-on-multiple-match":
				// TODO
			default:
			}
		}
		s.Modes = append(s.Modes, &m)
	}
	return nil
}

func (s *Stylesheet) loadParams(doc xml.Node) error {
	items, err := s.queryXSL("/stylesheet/param | transform/param", doc)
	if err != nil {
		return err
	}
	for i := range items {
		n := items[i].Node().(*xml.Element)
		if n == nil {
			continue
		}
		ident, err := getAttribute(n, "name")
		if err != nil {
			return err
		}
		if query, err := getAttribute(n, "select"); err == nil {
			expr, err := s.CompileQuery(query)
			if err != nil {
				return err
			}
			s.Define(ident, expr)
		} else {
			if len(n.Nodes) != 1 {
				return fmt.Errorf("only one node expected")
			}
			n := cloneNode(n.Nodes[0])
			s.Define(ident, xml.NewValueFromNode(n))
		}
	}
	return nil
}

func (s *Stylesheet) loadOutput(doc xml.Node) error {
	items, err := s.queryXSL("/stylesheet/output | /transform/output", doc)
	if err != nil {
		return err
	}
	for i := range items {
		var (
			node = items[i].Node().(*xml.Element)
			out  Output
		)
		for _, a := range node.Attrs {
			switch value := a.Value(); a.Name {
			case "name":
				out.Name = value
			case "method":
				out.Method = value
			case "version":
				out.Version = value
			case "encoding":
				out.Encoding = value
			case "indent":
				out.Indent = value == "yes"
			case "omit-xml-declaration":
				out.OmitProlog = value == "yes"
			default:
			}
		}
		s.output = append(s.output, &out)
	}
	if len(s.output) == 0 {
		s.output = append(s.output, defaultOutput())
	}
	return nil
}

func (s *Stylesheet) loadTemplates(doc xml.Node) error {
	items, err := s.queryXSL("/stylesheet/template | /transform/template", doc)
	if err != nil {
		return err
	}
	for _, el := range items {
		t, err := NewTemplate(el.Node())
		if err != nil {
			return err
		}
		s.Templates = append(s.Templates, t)
	}
	return nil
}

func (s *Stylesheet) writeDocument(w io.Writer, format string, doc *xml.Document) error {
	var (
		writer  = xml.NewWriter(w)
		setting = s.GetOutput(format)
	)
	if !setting.Indent {
		writer.WriterOptions |= xml.OptionCompact
	}
	if setting.OmitProlog {
		writer.WriterOptions |= xml.OptionNoProlog
	}
	if setting.Method == "html" && (setting.Version == "5" || setting.Version == "5.0") {
		writer.PrologWriter = xml.PrologWriterFunc(writeDoctypeHTML)
	}
	return writer.Write(doc)
}

func (s *Stylesheet) resolve(ident string) (xml.Expr, error) {
	expr, err := s.Env.Resolve(ident)
	if err == nil {
		return expr, nil
	}
	for i := range s.Others {
		e, err := s.Others[i].resolve(ident)
		if err == nil {
			return e, nil
		}
	}
	return nil, err
}

func (s *Stylesheet) getQualifiedName(name string) string {
	qn := xml.QualifiedName(name, s.namespace)
	return qn.QualifiedName()
}

func (s *Stylesheet) getMainTemplate() (*Template, error) {
	if len(s.Templates) == 1 {
		return s.Templates[0], nil
	}
	ix := slices.IndexFunc(s.Templates, func(t *Template) bool {
		return t.isRoot() && t.Mode == s.CurrentMode()
	})
	if ix < 0 {
		return nil, fmt.Errorf("main template not found")
	}
	return s.Templates[ix], nil
}

func (s *Stylesheet) getMatchingTemplates(list []*Template, node xml.Node, mode string) (*Template, error) {
	type TemplateMatch struct {
		*Template
		Priority int
	}
	var results []*TemplateMatch
	for _, t := range list {
		if t.isRoot() || t.Mode != mode || t.Name != "" {
			continue
		}
		expr, err := s.CompileQuery(t.Match)
		if err != nil {
			continue
		}
		ok, prio := templateMatch(expr, node)
		if !ok {
			continue
		}
		m := TemplateMatch{
			Template: t,
			Priority: prio + t.Priority,
		}
		results = append(results, &m)
	}
	if len(results) > 0 {
		slices.SortFunc(results, func(m1, m2 *TemplateMatch) int {
			return m2.Priority - m1.Priority
		})
		tpl := results[0].Template.Clone()
		return tpl, nil
	}
	return nil, fmt.Errorf("no template found matching given node (%s)", node.QualifiedName())
}

func (s *Stylesheet) getMode(mode string) *Mode {
	ix := slices.IndexFunc(s.Modes, func(m *Mode) bool {
		return m.Name == mode
	})
	if ix < 0 {
		return unnamedMode()
	}
	return s.Modes[ix]
}

type Template struct {
	Name     string
	Match    string
	Mode     string
	Priority int

	Nodes []xml.Node

	env *Env
}

func NewTemplate(node xml.Node) (*Template, error) {
	elem, ok := node.(*xml.Element)
	if !ok {
		return nil, fmt.Errorf("%s: xml element expected to load template", node.QualifiedName())
	}
	tpl := Template{
		env: Empty(),
	}
	for _, a := range elem.Attrs {
		switch attr := a.Value(); a.Name {
		case "priority":
			p, err := strconv.Atoi(attr)
			if err != nil {
				return nil, err
			}
			tpl.Priority = p
		case "name":
			tpl.Name = attr
		case "match":
			tpl.Match = attr
		case "mode":
			tpl.Mode = attr
		default:
		}
	}
	for i, n := range elem.Nodes {
		if n.QualifiedName() != "xsl:param" {
			tpl.Nodes = append(tpl.Nodes, elem.Nodes[i:]...)
			break
		}
		if err := processParam(n, tpl.env); err != nil {
			return nil, err
		}
	}
	return &tpl, nil
}

func (t *Template) Clone() *Template {
	tpl := *t
	tpl.Nodes = slices.Clone(tpl.Nodes)
	return &tpl
}

func (t *Template) Execute(ctx *Context) ([]xml.Node, error) {
	var nodes []xml.Node
	for _, n := range slices.Clone(t.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		res, err := transformNode(ctx.WithXsl(c))
		if err != nil {
			if errors.Is(err, errSkip) {
				continue
			}
			return nil, err
		}
		for i := range res {
			nodes = append(nodes, res[i].Node())
		}
	}
	return nodes, nil
}

func (t *Template) mergeContext(other *Context) *Context {
	child := other.Copy()
	child.Env = child.Env.Sub()
	child.Env.Merge(t.env)
	return child
}

func (t *Template) isRoot() bool {
	return t.Match == "/"
}

func templateMatch(expr xml.Expr, node xml.Node) (bool, int) {
	var (
		depth int
		curr  = node
	)
	for curr != nil {
		items, err := expr.Find(curr)
		if err != nil {
			break
		}
		if len(items) > 0 {
			ok := slices.ContainsFunc(items, func(i xml.Item) bool {
				n := i.Node()
				return n.Identity() == node.Identity()
			})
			return ok, depth + expr.MatchPriority()
		}
		depth--
		curr = curr.Parent()
	}
	return false, 0
}
