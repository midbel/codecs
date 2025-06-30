package xslt

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/midbel/codecs/alpha"
	"github.com/midbel/codecs/environ"
	"github.com/midbel/codecs/xml"
	"github.com/midbel/codecs/xpath"
)

const (
	XslVersion        = "3"
	XslVendor         = "angle"
	XslVendorUrl      = "angle"
	XslProduct        = "angle"
	XslProductVersion = "0.0.0"
)

const (
	xsltNamespaceUri    = "http://www.w3.org/1999/XSL/Transform"
	xsltNamespacePrefix = "xsl"
)

var (
	errImplemented = errors.New("not implemented")
	errUndefined   = errors.New("undefined")
	errSkip        = errors.New("skip")
	errBreak       = errors.New("break")
	errIterate     = errors.New("next-iteration")
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
		Version:  xml.SupportedVersion,
		Encoding: xml.SupportedEncoding,
		Indent:   false,
	}
	return out
}

type Executer interface {
	Execute(*Context) ([]xml.Node, error)
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

const (
	currentMode = "#current"
	defaultMode = "#default"
)

type Mode struct {
	Name       string
	Default    bool
	NoMatch    NoMatchMode
	MultiMatch MultiMatchMode

	Templates []*Template
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

func (m *Mode) callTemplate(name string) (Executer, error) {
	ix := slices.IndexFunc(m.Templates, func(t *Template) bool {
		return t.Name == name
	})
	if ix < 0 {
		return nil, fmt.Errorf("%s: template not found", name)
	}
	return m.Templates[ix].Clone(), nil
}

func (m *Mode) mainTemplate() (Executer, error) {
	ix := slices.IndexFunc(m.Templates, func(t *Template) bool {
		return t.isRoot()
	})
	if ix >= 0 {
		return m.Templates[ix], nil
	}
	return nil, fmt.Errorf("main template not found")
}

func (m *Mode) matchTemplate(node xml.Node, env *Env) (Executer, error) {
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

func (m *Mode) noMatch(node xml.Node) (Executer, error) {
	var exec Executer
	switch m.NoMatch {
	case NoMatchTextOnlyCopy:
		exec = textOnlyCopy{}
	case NoMatchDeepCopy:
		exec = deepCopy{}
	case NoMatchShallowCopy:
		exec = shallowCopy{}
	case NoMatchDeepSkip:
		exec = deepSkip{}
	case NoMatchShallowSkip:
		exec = shallowSkip{}
	case NoMatchFail:
		return nil, fmt.Errorf("%s: no template match", node.QualifiedName())
	default:
		return nil, fmt.Errorf("%s: no template match", node.QualifiedName())
	}
	return exec, nil
}

type textOnlyCopy struct{}

func (c textOnlyCopy) Execute(ctx *Context) ([]xml.Node, error) {
	switch ctx.ContextNode.Type() {
	case xml.TypeElement:
		var (
			list []xml.Node
			elem = ctx.ContextNode.(*xml.Element)
		)
		for i := range elem.Nodes {
			others, err := ctx.WithXpath(elem.Nodes[i]).ApplyTemplate()
			if err != nil {
				return nil, err
			}
			list = slices.Concat(list, others)
		}
		return list, nil
	case xml.TypeDocument:
		var (
			list []xml.Node
			doc  = ctx.ContextNode.(*xml.Document)
		)
		for i := range doc.Nodes {
			others, err := ctx.WithXpath(doc.Nodes[i]).ApplyTemplate()
			if err != nil {
				return nil, err
			}
			list = slices.Concat(list, others)
		}
		return list, nil
	case xml.TypeText:
		node := xml.NewText(ctx.ContextNode.Value())
		return []xml.Node{node}, nil
	default:
		return nil, nil
	}
}

type deepCopy struct{}

func (_ deepCopy) Execute(ctx *Context) ([]xml.Node, error) {
	node := cloneNode(ctx.ContextNode)
	return []xml.Node{node}, nil
}

type shallowCopy struct{}

func (_ shallowCopy) Execute(ctx *Context) ([]xml.Node, error) {
	if ctx.ContextNode.Type() == xml.TypeDocument {
		doc := ctx.ContextNode.(*xml.Document)
		return ctx.WithXpath(doc.Root()).ApplyTemplate()
	}
	elem, err := getElementFromNode(ctx.ContextNode)
	if err != nil {
		return nil, err
	}
	clone, ok := elem.Copy().(*xml.Element)
	if !ok {
		return nil, nil
	}
	for _, n := range slices.Clone(elem.Nodes) {
		others, err := ctx.WithXpath(n).ApplyTemplate()
		if err != nil {
			return nil, err
		}
		for i := range others {
			clone.Append(others[i])
		}
	}
	return []xml.Node{clone}, nil
}

type deepSkip struct{}

func (_ deepSkip) Execute(ctx *Context) ([]xml.Node, error) {
	return nil, nil
}

type shallowSkip struct{}

func (_ shallowSkip) Execute(ctx *Context) ([]xml.Node, error) {
	var list []xml.Node
	switch ctx.ContextNode.Type() {
	case xml.TypeDocument:
		doc := ctx.ContextNode.(*xml.Document)
		for _, n := range doc.Nodes {
			res, err := ctx.WithXpath(n).ApplyTemplate()
			if err != nil {
				return nil, err
			}
			list = slices.Concat(list, res)
		}
	case xml.TypeElement:
		elem := ctx.ContextNode.(*xml.Element)
		for _, n := range elem.Nodes {
			res, err := ctx.WithXpath(n).ApplyTemplate()
			if err != nil {
				return nil, err
			}
			list = slices.Concat(list, res)
		}
	default:
	}
	return list, nil
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
	namer  alpha.Namer
	static *Env
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
		static:    Empty(),
		Env:       Empty(),
		Tracer:    NoopTracer(),
		namer:     alpha.Compose(alpha.NewLowerString(3), alpha.NewNumberString(2)),
	}

	sheet.defineBuiltins()

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

func (s *Stylesheet) Find(name, mode string) (Executer, error) {
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

func (s *Stylesheet) MatchImport(node xml.Node, mode string) (Executer, error) {
	for _, s := range s.Others {
		if tpl, err := s.Match(node, mode); err == nil {
			return tpl, err
		}
	}
	return nil, fmt.Errorf("no template found matching given node (%s)", node.QualifiedName())
}

func (s *Stylesheet) Match(node xml.Node, mode string) (Executer, error) {
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
		if len(nodes) > 1 {
			if !s.WrapRoot {
				return nil, fmt.Errorf("main template returns more than one node")
			}
			elem := xml.NewElement(xml.LocalName("angle"))
			for i := range nodes {
				elem.Append(nodes[i])
			}
			root = elem
		} else if len(nodes) == 1 {
			root = nodes[0]
		} else {
			root = xml.NewElement(xml.LocalName("angle"))
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
	fmt.Println(ident, ix)
	if err := elem.RemoveAttr(elem.Attrs[ix].Position()); err != nil {
		return err
	}
	for _, a := range s.AttrSet[ix].Attrs {
		elem.SetAttribute(a)
	}
	return nil
}

func (s *Stylesheet) SetParam(ident string, expr xpath.Expr) {
	s.Env.DefineExprParam(ident, expr)
}

func (s *Stylesheet) GetOutput(name string) *Output {
	ix := slices.IndexFunc(s.output, func(o *Output) bool {
		return o.Name == name
	})
	if ix < 0 && name == "" {
		return defaultOutput()
	}
	return s.output[ix]
}

func (s *Stylesheet) CurrentMode() string {
	return s.Mode
}

func (s *Stylesheet) createContext(node xml.Node) *Context {
	return &Context{
		ContextNode: node,
		Mode:        s.Mode,
		Size:        1,
		Index:       1,
		Stylesheet:  s,
		Env:         Enclosed(s),
	}
}

func (s *Stylesheet) init(doc xml.Node) error {
	if doc.Type() != xml.TypeDocument {
		return fmt.Errorf("document expected")
	}
	var (
		top  = doc.(*xml.Document)
		root = top.Root()
	)
	if root != nil && root.LocalName() != "stylesheet" && root.LocalName() != "transform" {
		r, err := s.simplified(root)
		if err != nil {
			return err
		}
		root = r
	}
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
		case s.getQualifiedName("variable"):
			err = s.loadVariable(n)
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

func (s *Stylesheet) defineBuiltins() {
	s.static.Builtins.Define("system-property", s.getSystemProperty)
}

func (s *Stylesheet) simplified(root xml.Node) (xml.Node, error) {
	elem, err := getElementFromNode(root)
	if err != nil {
		return nil, err
	}
	list := elem.Namespaces()
	ok := slices.ContainsFunc(list, func(ns xml.NS) bool {
		return ns.Prefix == xsltNamespacePrefix && ns.Uri == xsltNamespaceUri
	})
	if !ok {
		return nil, fmt.Errorf("simplified stylesheet should declared the xsl namespace")
	}
	elem.RemoveAttribute(xml.QualifiedName(xsltNamespacePrefix, "xmlns"))

	var (
		name  = xml.QualifiedName("template", xsltNamespacePrefix)
		tpl   = xml.NewElement(name)
		match = xml.NewAttribute(xml.LocalName("match"), "/")
	)
	tpl.SetAttribute(match)

	ix := slices.IndexFunc(elem.Nodes, func(n xml.Node) bool {
		ns, _, ok := strings.Cut(n.QualifiedName(), ":")
		return !ok && ns != xsltNamespacePrefix
	})
	top := xml.NewElement(elem.QName)
	if ix >= 0 {
		tpl.Nodes = append(tpl.Nodes, elem.Nodes[ix:]...)
	} else {
		ix = 0
	}
	ok = slices.ContainsFunc(elem.Nodes[:ix], func(n xml.Node) bool {
		return n.QualifiedName() == name.QualifiedName()
	})
	if ok {
		return nil, fmt.Errorf("simplified root can not contains xsl template")
	}
	top.Nodes = append(top.Nodes, elem.Nodes[:ix]...)
	top.Nodes = append(top.Nodes, tpl)
	return top, nil
}

func (s *Stylesheet) makeIdent() string {
	id, _ := s.namer.Next()
	return id
}

func (s *Stylesheet) getSystemProperty(ctx xpath.Context, args []xpath.Expr) (xpath.Sequence, error) {
	if len(args) > 1 {
		return nil, fmt.Errorf("invalid number of arguments")
	}
	items, err := args[0].Find(ctx)
	if err != nil {
		return nil, err
	}
	if items.Empty() {
		return items, nil
	}
	str, ok := items[0].Value().(string)
	if !ok {
		return nil, nil
	}
	switch str {
	case s.getQualifiedName("version"):
		str = XslVersion
	case s.getQualifiedName("vendor"):
		str = XslVendor
	case s.getQualifiedName("vendor-url"):
		str = XslVendorUrl
	case s.getQualifiedName("product-name"):
		str = XslProduct
	case s.getQualifiedName("product-version"):
		str = XslProductVersion
	default:
		return nil, fmt.Errorf("%s: unknown system property", str)
	}
	return xpath.Singleton(str), nil
}

func (s *Stylesheet) useWhen(node *xml.Element) (bool, error) {
	query, err := getAttribute(node, "use-when")
	if err != nil {
		return true, nil
	}
	items, err := s.static.ExecuteQuery(query, node)
	if err != nil {
		return false, err
	}
	return items.True(), nil
}

func (s *Stylesheet) includeSheet(node xml.Node) error {
	elem, err := getElementFromNode(node)
	if err != nil {
		return err
	}
	if ok, _ := s.useWhen(elem); !ok {
		return nil
	}
	ctx := s.createContext(nil)
	return includeSheet(ctx.WithXsl(node))
}

func (s *Stylesheet) importSheet(node xml.Node) error {
	elem, err := getElementFromNode(node)
	if err != nil {
		return err
	}
	if ok, _ := s.useWhen(elem); !ok {
		return nil
	}
	ctx := s.createContext(nil)
	return importSheet(ctx.WithXsl(node))
}

func (s *Stylesheet) loadAttributeSet(node xml.Node) error {
	elem, err := getElementFromNode(node)
	if err != nil {
		return err
	}
	if ok, _ := s.useWhen(elem); !ok {
		return nil
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
	if ok, _ := s.useWhen(elem); !ok {
		return nil
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

func (s *Stylesheet) loadVariable(node xml.Node) error {
	elem, err := getElementFromNode(node)
	if err != nil {
		return err
	}
	if ok, _ := s.useWhen(elem); !ok {
		return nil
	}
	ident, err := getAttribute(elem, "name")
	if err != nil {
		return err
	}
	var static bool
	if yes, err := getAttribute(elem, "static"); err == nil && yes == "yes" {
		static = true
	}
	if query, err := getAttribute(elem, "select"); err == nil {
		if len(elem.Nodes) > 0 {
			return fmt.Errorf("select attribute can not be used with children")
		}
		if static {
			return s.static.Eval(ident, query, elem)
		}
		expr, err := s.CompileQuery(query)
		if err != nil {
			return err
		}
		s.Define(ident, expr)
	} else {
		seq, err := executeConstructor(s.createContext(elem), elem.Nodes, 0)
		if err != nil {
			return err
		}
		s.Define(ident, xpath.NewValueFromSequence(seq))
	}
	return nil
}

func (s *Stylesheet) loadParam(node xml.Node) error {
	elem, err := getElementFromNode(node)
	if err != nil {
		return err
	}
	if ok, _ := s.useWhen(elem); !ok {
		return nil
	}
	ident, err := getAttribute(elem, "name")
	if err != nil {
		return err
	}
	var static bool
	if yes, err := getAttribute(elem, "static"); err == nil && yes == "yes" {
		static = true
	}
	if query, err := getAttribute(elem, "select"); err == nil {
		if len(elem.Nodes) > 0 {
			return fmt.Errorf("select attribute can not be used with children")
		}
		if static {
			return s.static.EvalParam(ident, query, elem)
		}
		return s.DefineParam(ident, query)
	} else {
		seq, err := executeConstructor(s.createContext(elem), elem.Nodes, 0)
		if err != nil {
			return err
		}
		s.DefineExprParam(ident, xpath.NewValueFromSequence(seq))
	}
	return nil
}

func (s *Stylesheet) loadOutput(node xml.Node) error {
	elem, err := getElementFromNode(node)
	if err != nil {
		return err
	}
	if ok, _ := s.useWhen(elem); !ok {
		return nil
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
	if ok, _ := s.useWhen(elem); !ok {
		return nil
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

func (s *Stylesheet) getMainTemplate() (Executer, error) {
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

	env    *Env
	tunnel []string
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
		if err := tpl.defineParamFromNode(n); err != nil {
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

func (t *Template) Call(ctx *Context) ([]xml.Node, error) {
	ctx = ctx.Last()
	if a, ok := ctx.Env.Params.(interface {
		Attach(environ.Environ[xpath.Expr])
	}); ok {
		a.Attach(t.env)
	}
	return t.Execute(ctx)
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

func (t *Template) EmptyContext(other *Context) *Context {
	ctx := other.Nest()
	// re-set the template.env
	return ctx
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

func (t *Template) defineParamFromNode(node xml.Node) error {
	elem, err := getElementFromNode(node)
	if err != nil {
		return err
	}
	ident, err := getAttribute(elem, "name")
	if err != nil {
		return err
	}
	if query, err := getAttribute(elem, "select"); err == nil {
		if len(elem.Nodes) > 0 {
			return fmt.Errorf("using select and children nodes is not allowed")
		}
		err = t.env.DefineParam(ident, query)
	} else {
		var seq xpath.Sequence
		for i := range elem.Nodes {
			seq.Append(xpath.NewNodeItem(elem.Nodes[i]))
		}
		t.env.DefineExprParam(ident, xpath.NewValueFromSequence(seq))
	}
	return err
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
