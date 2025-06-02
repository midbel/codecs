package xslt

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/midbel/codecs/xml"
	"github.com/midbel/codecs/xpath"
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

type AttributeSet struct {
	Name  string
	Attrs []xml.Attribute
}

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

type NoMatchMode int8

const (
	NoMatchDeepCopy NoMatchMode = 1 << iota
	NoMatchShallowCopy
	NoMatchDeepSkip
	NoMatchShallowSkip
	NoMatchTextOnlyCopy
	NoMatchFail
)

type MultiMatchMode int8

const (
	MultiMatchFail MultiMatchMode = 1 << iota
	MultiMatchLast
)

type Mode struct {
	Name       string
	Default    bool
	NoMatch    NoMatchMode
	MultiMatch MultiMatchMode

	Templates []*Template
	Builtins  []*Template
}

func namedMode(name string) *Mode {
	return &Mode{
		Name:       name,
		NoMatch:    NoMatchFail,
		MultiMatch: MultiMatchFail,
	}
}

func unnamedMode() *Mode {
	return namedMode("")
}

func (m *Mode) Unnamed() bool {
	return m.Name == ""
}

func (m *Mode) Merge(other *Mode) error {
	for _, t := range other.Templates {
		if err := m.Append(t); err != nil {
			return err
		}
	}
	return nil
}

func (m *Mode) Append(t *Template) error {
	found := slices.ContainsFunc(m.Templates, func(other *Template) bool {
		return other.Name == t.Name && other.Match == t.Match
	})
	if found {
		return fmt.Errorf("duplicate match/name/mode template")
	}
	m.Templates = append(m.Templates, t)
	return nil
}

func (m *Mode) callTemplate(name string) (*Template, error) {
	ix := slices.IndexFunc(m.Templates, func(t *Template) bool {
		return t.Name == name
	})
	if ix < 0 {
		return nil, fmt.Errorf("%s: template not found", name)
	}
	return m.Templates[ix].Clone(), nil
}

func (m *Mode) mainTemplate() (*Template, error) {
	ix := slices.IndexFunc(m.Templates, func(t *Template) bool {
		return t.isRoot()
	})
	if ix >= 0 {
		return m.Templates[ix], nil
	}
	return nil, fmt.Errorf("main template not found")
}

func (m *Mode) matchTemplate(node xml.Node, env *Env) (*Template, error) {
	type TemplateMatch struct {
		*Template
		Position int
		Priority float64
	}
	var results []*TemplateMatch
	for i, t := range m.Templates {
		if t.isRoot() || t.Name != "" || t.Match == "" {
			continue
		}
		expr, err := env.CompileQuery(t.Match)
		if err != nil {
			continue
		}
		ok, prio := templateMatch(expr, node)
		if !ok {
			continue
		}
		match := TemplateMatch{
			Template: t,
			Position: i,
			Priority: float64(prio) + t.Priority,
		}
		results = append(results, &match)
	}
	if len(results) > 0 {
		if len(results) > 1 && m.MultiMatch == MultiMatchFail {
			return nil, fmt.Errorf("%s: more than one template match", node.QualifiedName())
		}
		if m.MultiMatch == MultiMatchLast {
			return results[len(results)-1].Template.Clone(), nil
		}
		slices.SortFunc(results, func(m1, m2 *TemplateMatch) int {
			if m1.Priority == m2.Priority {
				return m1.Position - m2.Position
			}
			if m1.Priority > m2.Priority {
				return -1
			}
			return 1
		})
		return results[0].Template.Clone(), nil
	}
	return m.noMatch(node)
}

func (m *Mode) noMatch(node xml.Node) (*Template, error) {
	switch m.NoMatch {
	case NoMatchDeepCopy:
	case NoMatchShallowCopy:
	case NoMatchDeepSkip:
	case NoMatchShallowSkip:
	case NoMatchTextOnlyCopy:
	case NoMatchFail:
		return nil, fmt.Errorf("%s: no template match", node.QualifiedName())
	default:
		return nil, fmt.Errorf("%s: no template match", node.QualifiedName())
	}
	return nil, errImplemented
}

type Stylesheet struct {
	DefaultMode           string
	WrapRoot              bool
	StrictModeDeclaration bool

	namespace string
	Mode      string
	Modes     []*Mode
	AttrSet   []*AttributeSet

	output []*Output
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
		Context:   contextDir,
		namespace: xsltNamespacePrefix,
		Env:       Empty(),
		Tracer:    NoopTracer(),
	}
	sheet.Modes = append(sheet.Modes, unnamedMode())
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
		ix = slices.IndexFunc(root.Attrs, func(a xml.Attribute) bool {
			return a.Name == "default-mode"
		})
		if ix >= 0 {
			mode := namedMode(sheet.DefaultMode)
			mode.Default = true
			sheet.Modes = append(sheet.Modes, mode)
			sheet.DefaultMode = root.Attrs[ix].Value()
		}
	}

	if err := sheet.init(doc); err != nil {
		return nil, err
	}
	return &sheet, nil
}

func (s *Stylesheet) Find(name, mode string) (*Template, error) {
	ix := slices.IndexFunc(s.Modes, func(m *Mode) bool {
		return m.Name == mode
	})
	if ix >= 0 {
		tpl, err := s.Modes[ix].callTemplate(name)
		if err != nil {
			return nil, err
		}
		return tpl, nil
	}
	for _, s := range s.Others {
		if tpl, err := s.Find(name, mode); err == nil {
			return tpl, nil
		}
	}
	return nil, fmt.Errorf("template %s not found", name)
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
	ix := slices.IndexFunc(s.Modes, func(m *Mode) bool {
		return m.Name == mode
	})
	if ix >= 0 {
		tpl, err := s.Modes[ix].matchTemplate(node, s.Env)
		return tpl, err
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
	s.Start()
	defer s.Done()
	tpl, err := s.getMainTemplate()
	if err != nil {
		return nil, err
	}
	nodes, err := tpl.Execute(s.createContext(doc))
	if err == nil {
		var root xml.Node
		if len(nodes) != 1 {
			if !s.WrapRoot {
				return nil, fmt.Errorf("main template returns more than one node")
			}
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
	for _, m := range other.Modes {
		ix := slices.IndexFunc(s.Modes, func(c *Mode) bool {
			return c.Name == m.Name
		})
		if ix < 0 {
			s.Modes = append(s.Modes, m)
		} else {
			if err := s.Modes[ix].Merge(m); err != nil {
				return err
			}
		}
	}
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

func (s *Stylesheet) SetParam(ident string, expr xpath.Expr) {
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
		Mode:        s.Mode,
		Size:        1,
		Index:       1,
		Stylesheet:  s,
		Env:         env,
	}
	return &ctx
}

func (s *Stylesheet) init(doc xml.Node) error {
	if doc.Type() != xml.TypeDocument {
		return fmt.Errorf("document expected")
	}
	var (
		top  = doc.(*xml.Document)
		root = top.Root()
	)
	r, err := getElementFromNode(root)
	if err != nil {
		return err
	}
	for _, n := range r.Nodes {
		var err error
		switch name := n.QualifiedName(); name {
		case s.getQualifiedName("include"):
			err = s.includeSheet(n)
		case s.getQualifiedName("import"):
			err = s.importSheet(n)
		case s.getQualifiedName("output"):
			err = s.loadOutput(n)
		case s.getQualifiedName("param"):
			err = s.loadParam(n)
		case s.getQualifiedName("attribute-set"):
			err = s.loadAttributeSet(n)
		case s.getQualifiedName("template"):
			err = s.loadTemplate(n)
		case s.getQualifiedName("mode"):
			err = s.loadMode(n)
		default:
			err = fmt.Errorf("%s: unexpected element", name)
		}
		if err != nil {
			return err
		}
	}
	if len(s.output) == 0 {
		s.output = append(s.output, defaultOutput())
	}
	return nil
}

func (s *Stylesheet) simplified(root xml.Node) error {
	elem, err := getElementFromNode(root)
	if err != nil {
		return err
	}
	list := elem.Namespaces()
	ok := slices.ContainsFunc(list, func(ns xml.NS) bool {
		return ns.Prefix == xsltNamespacePrefix && ns.Uri == xsltNamespaceUri
	})
	if !ok {
		return fmt.Errorf("simplified stylesheet should declared the xsl namespace")
	}
	elem.RemoveAttribute(xml.QualifiedName(xsltNamespacePrefix, "xmlns"))
	tpl, err := NewTemplate(elem)
	if err != nil {
		return err
	}
	tpl.Match = "/"
	ix := slices.IndexFunc(s.Modes, func(m *Mode) bool {
		return m.Unnamed()
	})
	if ix >= 0 {
		return s.Modes[ix].Append(tpl)
	}
	return fmt.Errorf("unnamed mode not found")
}

func (s *Stylesheet) includeSheet(node xml.Node) error {
	ctx := s.createContext(nil)
	return includeSheet(ctx.WithXsl(node))
}

func (s *Stylesheet) importSheet(node xml.Node) error {
	ctx := s.createContext(nil)
	return importSheet(ctx.WithXsl(node))
}

func (s *Stylesheet) loadAttributeSet(node xml.Node) error {
	elem, err := getElementFromNode(node)
	if err != nil {
		return err
	}
	ident, err := getAttribute(elem, "name")
	if err != nil {
		return err
	}
	as := AttributeSet{
		Name: ident,
	}
	for _, n := range elem.Nodes {
		n, err := getElementFromNode(n)
		if err != nil {
			return err
		}
		if n.QualifiedName() != s.getQualifiedName("attribute") {
			return fmt.Errorf("xsl:attribute element expected")
		}
		ident, err := getAttribute(n, "name")
		if err != nil {
			return err
		}
		attr := xml.NewAttribute(xml.LocalName(ident), n.Value())
		as.Attrs = append(as.Attrs, attr)
	}
	s.AttrSet = append(s.AttrSet, &as)
	return nil
}

func (s *Stylesheet) loadMode(node xml.Node) error {
	elem, err := getElementFromNode(node)
	if err != nil {
		return err
	}
	var m Mode
	for _, a := range elem.Attrs {
		switch attr := a.Value(); a.Name {
		case "name":
			m.Name = attr
		case "on-no-match":
			m.NoMatch = NoMatchFail
		case "on-multiple-match":
			m.MultiMatch = MultiMatchFail
		case "warning-on-no-match":
		case "warning-on-multiple-match":
		default:
		}
	}
	ix := slices.IndexFunc(s.Modes, func(o *Mode) bool {
		return o.Name == m.Name
	})
	if ix < 0 {
		s.Modes = append(s.Modes, &m)
	} else if s.Modes[ix].Unnamed() || s.Modes[ix].Default {
		s.Modes[ix].NoMatch = m.NoMatch
		s.Modes[ix].MultiMatch = m.MultiMatch
	} else {
		return fmt.Errorf("%s mode already defined", m.Name)
	}
	return nil
}

func (s *Stylesheet) loadParam(node xml.Node) error {
	elem, err := getElementFromNode(node)
	if err != nil {
		return err
	}
	ident, err := getAttribute(elem, "name")
	if err != nil {
		return err
	}
	if query, err := getAttribute(elem, "select"); err == nil {
		expr, err := s.CompileQuery(query)
		if err != nil {
			return err
		}
		s.Define(ident, expr)
	} else {
		if len(elem.Nodes) != 1 {
			return fmt.Errorf("only one node expected")
		}
		n := cloneNode(elem.Nodes[0])
		s.Define(ident, xpath.NewValueFromNode(n))
	}
	return nil
}

func (s *Stylesheet) loadOutput(node xml.Node) error {
	elem, err := getElementFromNode(node)
	if err != nil {
		return err
	}
	var out Output
	for _, a := range elem.Attrs {
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
	return nil
}

func (s *Stylesheet) loadTemplate(node xml.Node) error {
	elem, err := getElementFromNode(node)
	if err != nil {
		return err
	}

	tpl, err := NewTemplate(elem)
	if err != nil {
		return err
	}
	ix := slices.IndexFunc(s.Modes, func(m *Mode) bool {
		return m.Name == tpl.Mode
	})
	if ix < 0 {
		mode := namedMode(tpl.Mode)
		err = mode.Append(tpl)
		if err == nil {
			s.Modes = append(s.Modes, mode)
		}
	} else {
		err = s.Modes[ix].Append(tpl)
	}
	return err
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

func (s *Stylesheet) resolve(ident string) (xpath.Expr, error) {
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
	ix := slices.IndexFunc(s.Modes, func(m *Mode) bool {
		return m.Name == s.Mode
	})
	if ix >= 0 {
		return s.Modes[ix].mainTemplate()
	}
	return nil, fmt.Errorf("main template not found")
}

type Template struct {
	Name     string
	Match    string
	Mode     string
	Priority float64

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
			p, err := strconv.ParseFloat(attr, 64)
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

func (t *Template) BuiltinRule() bool {
	return false
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

func templateMatch(expr xpath.Expr, node xml.Node) (bool, int) {
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
			ok := slices.ContainsFunc(items, func(i xpath.Item) bool {
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

func importSheet(ctx *Context) error {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return ctx.errorWithContext(err)
	}
	file, err := getAttribute(elem, "href")
	if err != nil {
		return ctx.errorWithContext(err)
	}
	return ctx.ImportSheet(file)
}

func includeSheet(ctx *Context) error {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return ctx.errorWithContext(err)
	}
	file, err := getAttribute(elem, "href")
	if err != nil {
		return ctx.errorWithContext(err)
	}
	return ctx.IncludeSheet(file)
}
