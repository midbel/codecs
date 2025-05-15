package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

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

type ExecuteFunc func(*Context, xml.Node) (xml.Sequence, error)

var executers map[xml.QName]ExecuteFunc

func init() {
	wrap := func(exec ExecuteFunc) ExecuteFunc {
		fn := func(ctx *Context, node xml.Node) (xml.Sequence, error) {
			return exec(ctx.Self(), node)
		}
		return fn
	}
	executers = map[xml.QName]ExecuteFunc{
		xml.QualifiedName("for-each", xsltNamespacePrefix):        wrap(executeForeach),
		xml.QualifiedName("value-of", xsltNamespacePrefix):        executeValueOf,
		xml.QualifiedName("call-template", xsltNamespacePrefix):   wrap(executeCallTemplate),
		xml.QualifiedName("apply-templates", xsltNamespacePrefix): wrap(executeApplyTemplates),
		xml.QualifiedName("apply-imports", xsltNamespacePrefix):   wrap(executeApplyImport),
		xml.QualifiedName("if", xsltNamespacePrefix):              wrap(executeIf),
		xml.QualifiedName("choose", xsltNamespacePrefix):          wrap(executeChoose),
		xml.QualifiedName("where-populated", xsltNamespacePrefix): executeWherePopulated,
		xml.QualifiedName("on-empty", xsltNamespacePrefix):        executeOnEmpty,
		xml.QualifiedName("on-not-empty", xsltNamespacePrefix):    executeOnNotEmpty,
		xml.QualifiedName("try", xsltNamespacePrefix):             wrap(executeTry),
		xml.QualifiedName("variable", xsltNamespacePrefix):        executeVariable,
		xml.QualifiedName("result-document", xsltNamespacePrefix): executeResultDocument,
		xml.QualifiedName("source-document", xsltNamespacePrefix): executeSourceDocument,
		xml.QualifiedName("import", xsltNamespacePrefix):          executeImport,
		xml.QualifiedName("include", xsltNamespacePrefix):         executeInclude,
		xml.QualifiedName("with-param", xsltNamespacePrefix):      executeWithParam,
		xml.QualifiedName("copy", xsltNamespacePrefix):            executeCopy,
		xml.QualifiedName("copy-of", xsltNamespacePrefix):         executeCopyOf,
		xml.QualifiedName("sequence", xsltNamespacePrefix):        executeSequence,
		xml.QualifiedName("element", xsltNamespacePrefix):         executeElement,
		xml.QualifiedName("attribute", xsltNamespacePrefix):       executeAttribute,
		xml.QualifiedName("text", xsltNamespacePrefix):            executeText,
		xml.QualifiedName("comment", xsltNamespacePrefix):         executeComment,
		xml.QualifiedName("message", xsltNamespacePrefix):         executeMessage,
		xml.QualifiedName("fallback", xsltNamespacePrefix):        executeFallback,
		xml.QualifiedName("merge", xsltNamespacePrefix):           executeMerge,
		xml.QualifiedName("for-each-group", xsltNamespacePrefix):  executeForeachGroup,
	}
}

func main() {
	var (
		quiet  = flag.Bool("q", false, "quiet")
		mode   = flag.String("m", "", "mode")
		file   = flag.String("f", "", "file")
		dir    = flag.String("d", ".", "context directory")
		params = make(map[string]xml.Expr)
	)
	flag.Func("p", "template parameter", func(str string) error {
		ident, query, ok := strings.Cut(str, "=")
		if !ok {
			return fmt.Errorf("invalid parameter")
		}
		expr, err := xml.CompileString(query)
		if err == nil {
			params[ident] = expr
		}
		return err
	})
	flag.Parse()

	doc, err := loadDocument(flag.Arg(1))
	if err != nil {
		fmt.Fprintln(os.Stderr, "fail to load document:", err)
		os.Exit(2)
	}

	sheet, err := Load(flag.Arg(0), *dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fail to load stylesheet", err)
		os.Exit(2)
	}
	sheet.Mode = *mode

	for ident, expr := range params {
		sheet.SetParam(ident, expr)
	}

	var w io.Writer = os.Stdout
	if *quiet {
		w = io.Discard
	} else if *file != "" {
		f, err := os.Create(*file)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
		defer f.Close()
		w = f
	}
	if err := sheet.Generate(w, doc); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type OutputSettings struct {
	Name       string
	Method     string
	Encoding   string
	Version    string
	Indent     bool
	OmitProlog bool
}

func defaultOutput() *OutputSettings {
	out := &OutputSettings{
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

var unnamedMode = Mode{
	NoMatch:    MatchFail,
	MultiMatch: MatchFail,
}

func (m Mode) Unnamed() bool {
	return m.Name == ""
}

type AttributeSet struct {
	Name  string
	Attrs []xml.Attribute
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

func (e *Env) ExecuteQuery(query string, datum xml.Node) (xml.Sequence, error) {
	return e.ExecuteQueryWithNS(query, "", datum)
}

func (e *Env) ExecuteQueryWithNS(query, namespace string, datum xml.Node) (xml.Sequence, error) {
	if query == "" {
		i := xml.NewNodeItem(datum)
		return []xml.Item{i}, nil
	}
	q, err := e.CompileQueryWithNS(query, namespace)
	if err != nil {
		return nil, err
	}
	return q.Find(datum)
}

func (e *Env) queryXSL(query string, datum xml.Node) (xml.Sequence, error) {
	return e.ExecuteQueryWithNS(query, e.Namespace, datum)
}

func (e *Env) CompileQuery(query string) (xml.Expr, error) {
	return e.CompileQueryWithNS(query, "")
}

func (e *Env) CompileQueryWithNS(query, namespace string) (xml.Expr, error) {
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

func (e *Env) TestNode(query string, datum xml.Node) (bool, error) {
	items, err := e.ExecuteQuery(query, datum)
	if err != nil {
		return false, err
	}
	return isTrue(items), nil
}

func (e *Env) Merge(other *Env) {
	if m, ok := e.Vars.(interface{ Merge(xml.Environ[xml.Expr]) }); ok {
		m.Merge(other.Vars)
	}
	if m, ok := e.Params.(interface{ Merge(xml.Environ[xml.Expr]) }); ok {
		m.Merge(other.Params)
	}
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
	expr, err := e.CompileQuery(value)
	if err == nil {
		e.DefineExprParam(param, expr)
	}
	return err
}

func (e *Env) EvalParam(param, query string, datum xml.Node) error {
	items, err := e.ExecuteQuery(query, datum)
	if err == nil {
		e.DefineExprParam(param, xml.NewValueFromSequence(items))
	}
	return err
}

func (e *Env) DefineExprParam(param string, expr xml.Expr) {
	e.Params.Define(param, expr)
}

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

type Stylesheet struct {
	namespace   string
	Mode        string
	currentMode *Mode
	Modes       []*Mode
	AttrSet     []*AttributeSet

	output    []*OutputSettings
	Templates []*Template
	*Env

	Context string
	Others  []*Stylesheet
}

func Load(file, contextDir string) (*Stylesheet, error) {
	doc, err := loadDocument(file)
	if err != nil {
		return nil, err
	}
	if contextDir == "" {
		contextDir = filepath.Dir(file)
	}
	sheet := Stylesheet{
		Context:     contextDir,
		currentMode: &unnamedMode,
		namespace:   xsltNamespacePrefix,
		Env:         Empty(),
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

func includesSheet(sheet *Stylesheet, doc xml.Node) error {
	items, err := sheet.queryXSL("/stylesheet/include | /transform/include", doc)
	if err != nil {
		return err
	}
	ctx := sheet.createContext(nil)
	for _, i := range items {
		_, err := executeInclude(ctx, i.Node())
		if err != nil {
			return err
		}
	}
	return nil
}

func importsSheet(sheet *Stylesheet, doc xml.Node) error {
	items, err := sheet.queryXSL("/stylesheet/import | /transform/import", doc)
	if err != nil {
		return err
	}
	ctx := sheet.createContext(nil)
	for _, i := range items {
		_, err := executeImport(ctx, i.Node())
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Stylesheet) createContext(node xml.Node) *Context {
	env := Enclosed(s)
	env.Namespace = s.namespace
	ctx := Context{
		CurrentNode: node,
		Size:        1,
		Index:       1,
		Stylesheet:  s,
		Env:         env,
	}
	return &ctx
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
			out  OutputSettings
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
	if err := includesSheet(s, doc); err != nil {
		return err
	}
	if err := importsSheet(s, doc); err != nil {
		return err
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

func (s *Stylesheet) GetOutput(name string) *OutputSettings {
	ix := slices.IndexFunc(s.output, func(o *OutputSettings) bool {
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

func (s *Stylesheet) getQualifiedName(name string) string {
	qn := xml.QualifiedName(name, s.namespace)
	return qn.QualifiedName()
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
		ok, prio := isTemplateMatch(expr, node)
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

func (s *Stylesheet) getMode(mode string) *Mode {
	ix := slices.IndexFunc(s.Modes, func(m *Mode) bool {
		return m.Name == mode
	})
	if ix < 0 {
		return &unnamedMode
	}
	return s.Modes[ix]
}

func (s *Stylesheet) Generate(w io.Writer, doc *xml.Document) error {
	result, err := s.Execute(doc)
	if err != nil {
		return err
	}
	return s.writeDocument(w, "", result.(*xml.Document))
}

func (s *Stylesheet) Execute(doc xml.Node) (xml.Node, error) {
	ix := slices.IndexFunc(s.Templates, func(t *Template) bool {
		return t.isRoot() && t.Mode == s.CurrentMode()
	})
	if ix < 0 {
		return nil, fmt.Errorf("main template not found")
	}

	// if d, ok := doc.(*xml.Document); ok {
	// 	doc = d.Root()
	// }

	var (
		tpl = s.Templates[ix]
		ctx = s.createContext(doc)
	)
	root, err := tpl.Execute(ctx)
	if err == nil {
		var doc xml.Document
		doc.Nodes = append(doc.Nodes, root...)
		return &doc, nil
	}
	return nil, err
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
		el := n.(*xml.Element)
		if el == nil {
			return nil, fmt.Errorf("expected xml element for param")
		}
		ident, err := getAttribute(el, "name")
		if err != nil {
			return nil, err
		}
		if query, err := getAttribute(el, "select"); err == nil {
			err = tpl.env.DefineParam(ident, query)
		} else {
			var seq xml.Sequence
			for i := range el.Nodes {
				seq.Append(xml.NewNodeItem(el.Nodes[i]))
			}
			tpl.env.DefineExprParam(ident, xml.NewValueFromSequence(seq))
		}
		if err != nil {
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
		if err := t.execute(ctx, c); err != nil {
			if errors.Is(err, errSkip) {
				continue
			}
			return nil, err
		}
		nodes = append(nodes, c)
	}
	return nodes, nil
}

func (t *Template) createContext(other *Context, node xml.Node) *Context {
	other = other.Sub(node)
	other.Env.Merge(t.env)
	return other
}

func (t *Template) execute(ctx *Context, node xml.Node) error {
	_, err := transformNode(ctx, node)
	return err
}

func (t *Template) isRoot() bool {
	return t.Match == "/"
}

func processAVT(ctx *Context, node xml.Node) error {
	el := node.(*xml.Element)
	for i, a := range el.Attrs {
		var (
			value = a.Value()
			str   strings.Builder
		)
		for q, ok := range iterAVT(value) {
			if !ok {
				str.WriteString(q)
				continue
			}
			items, err := ctx.ExecuteQuery(q, ctx.CurrentNode)
			if err != nil {
				return err
			}
			for i := range items {
				str.WriteString(toString(items[i]))
			}
		}
		el.Attrs[i].Datum = str.String()
	}
	return nil
}

func iterAVT(str string) iter.Seq2[string, bool] {
	fn := func(yield func(string, bool) bool) {
		var offset int
		for {
			var (
				ix  = strings.IndexRune(str[offset:], '{')
				ptr = offset
			)
			if ix < 0 {
				yield(str[offset:], false)
				break
			}
			offset += ix + 1
			ix = strings.IndexRune(str[offset:], '}')
			if ix < 0 {
				yield(str[offset-1:], false)
				break
			}
			if !yield(str[ptr:offset-1], false) {
				break
			}
			if !yield(str[offset:offset+ix], true) {
				break
			}
			offset += ix + 1
		}
	}
	return fn
}

func transformNode(ctx *Context, node xml.Node) (xml.Sequence, error) {
	elem, ok := node.(*xml.Element)
	if !ok {
		return nil, fmt.Errorf("node: xml element expected (got %s)", elem.QualifiedName())
	}
	fn, ok := executers[elem.QName]
	if ok {
		if fn == nil {
			return nil, fmt.Errorf("%s not yet implemented", elem.QualifiedName())
		}
		return fn(ctx, node)
	}
	return processNode(ctx, node)
}

func processNode(ctx *Context, node xml.Node) (xml.Sequence, error) {
	var (
		elem  = node.(*xml.Element)
		nodes = slices.Clone(elem.Nodes)
	)
	if err := processAVT(ctx, node); err != nil {
		return nil, err
	}
	if err := ctx.SetAttributes(node); err != nil {
		return nil, err
	}
	res := xml.NewSequence()
	for i := range nodes {
		if nodes[i].Type() != xml.TypeElement {
			continue
		}
		seq, err := transformNode(ctx, nodes[i])
		if err != nil {
			return nil, err
		}
		res = slices.Concat(res, seq)
	}
	return res, nil
}

func executeImport(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	file, err := getAttribute(el, "href")
	if err != nil {
		return nil, err
	}
	return nil, ctx.ImportSheet(file)
}

func executeInclude(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	file, err := getAttribute(el, "href")
	if err != nil {
		return nil, err
	}
	return nil, ctx.IncludeSheet(file)
}

func executeSourceDocument(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	file, err := getAttribute(el, "href")
	if err != nil {
		return nil, err
	}
	doc, err := loadDocument(filepath.Join(ctx.Context, file))
	if err != nil {
		return nil, err
	}
	var (
		nodes []xml.Node
		sub   = ctx.Sub(doc)
	)
	for _, n := range slices.Clone(el.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		if _, err := transformNode(sub, c); err != nil {
			return nil, err
		}
		nodes = append(nodes, c)
	}
	return nil, insertNodes(el, nodes...)
}

func executeResultDocument(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)

	var doc xml.Document
	for _, n := range slices.Clone(el.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		if _, err := transformNode(ctx, c); err != nil {
			return nil, err
		}
		doc.Nodes = append(doc.Nodes, c)
	}

	file, err := getAttribute(el, "href")
	if err != nil {
		return nil, err
	}
	format, _ := getAttribute(el, "format")
	if err := writeDocument(file, format, &doc, ctx.Stylesheet); err != nil {
		return nil, err
	}
	if err := removeSelf(node); err != nil {
		return nil, err
	}
	return nil, errSkip
}

func executeVariable(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	ident, err := getAttribute(el, "name")
	if err != nil {
		return nil, err
	}
	if value, err := getAttribute(el, "select"); err == nil {
		query, err := ctx.CompileQuery(value)
		if err != nil {
			return nil, err
		}
		ctx.Define(ident, query)
	} else {
		var res xml.Sequence
		for _, n := range slices.Clone(el.Nodes) {
			c := cloneNode(n)
			if c == nil {
				continue
			}
			seq, err := transformNode(ctx, c)
			if err != nil {
				return nil, err
			}
			res = slices.Concat(res, seq)
		}
		ctx.Define(ident, xml.NewValueFromSequence(res))
	}
	return nil, removeSelf(node)
}

func executeWithParam(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	ident, err := getAttribute(el, "name")
	if err != nil {
		return nil, err
	}
	if query, err := getAttribute(el, "select"); err == nil {
		ctx.EvalParam(ident, query, ctx.CurrentNode)
	} else {
		var res xml.Sequence
		for _, n := range slices.Clone(el.Nodes) {
			c := cloneNode(n)
			if c == nil {
				continue
			}
			seq, err := transformNode(ctx, c)
			if err != nil {
				return nil, err
			}
			res = slices.Concat(res, seq)
		}
		ctx.DefineExprParam(ident, xml.NewValueFromSequence(res))
	}
	return nil, removeSelf(node)
}

func executeApplyImport(ctx *Context, node xml.Node) (xml.Sequence, error) {
	return executeApply(ctx, node, ctx.MatchImport)
}

func executeApplyTemplates(ctx *Context, node xml.Node) (xml.Sequence, error) {
	return executeApply(ctx, node, ctx.Match)
}

func executeCallTemplate(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	name, err := getAttribute(el, "name")
	if err != nil {
		return nil, err
	}
	mode, _ := getAttribute(el, "mode")
	tpl, err := ctx.Find(name, mode)
	if err != nil {
		return nil, ctx.NotFound(node, err, mode)
	}
	sub := tpl.createContext(ctx, ctx.CurrentNode)
	if err := applyParams(sub, node); err != nil {
		return nil, err
	}
	parent := node.Parent().(*xml.Element)
	for _, n := range slices.Clone(tpl.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		parent.Append(c)
		if _, err := transformNode(sub, c); err != nil {
			return nil, err
		}
	}
	return nil, removeSelf(node)
}

func executeForeachGroup(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	query, err := getAttribute(el, "select")
	if err != nil {
		return nil, err
	}
	parent, ok := el.Parent().(*xml.Element)
	if !ok {
		return nil, fmt.Errorf("for-each-group: xml element expected as parent")
	}

	items, err := ctx.ExecuteQuery(query, ctx.CurrentNode)
	if err != nil || len(items) == 0 {
		return nil, err
	}

	key, err := getAttribute(el, "group-by")
	if err != nil {
		return nil, err
	}
	grpby, err := ctx.CompileQuery(key)
	if err != nil {
		return nil, err
	}
	groups := make(map[string][]xml.Item)
	for i := range items {
		is, err := grpby.Find(items[i].Node())
		if err != nil {
			return nil, err
		}
		key := is[0].Value().(string)
		groups[key] = append(groups[key], items[i])
	}

	for key, items := range groups {
		currentGrp := func(_ xml.Context, _ []xml.Expr) (xml.Sequence, error) {
			return items, nil
		}
		currentKey := func(_ xml.Context, _ []xml.Expr) (xml.Sequence, error) {
			i := xml.NewLiteralItem(key)
			return []xml.Item{i}, nil
		}
		ctx.Builtins.Define("current-group", currentGrp)
		ctx.Builtins.Define("fn:current-group", currentGrp)
		ctx.Builtins.Define("current-grouping-key", currentKey)
		ctx.Builtins.Define("fn:current-grouping-key", currentKey)

		for _, n := range el.Nodes {
			c := cloneNode(n)
			if c == nil {
				continue
			}
			parent.Append(c)
			if _, err := transformNode(ctx, c); err != nil {
				return nil, err
			}
		}
	}
	return nil, removeSelf(node)
}

func executeMerge(ctx *Context, node xml.Node) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeForeach(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	query, err := getAttribute(el, "select")
	if err != nil {
		return nil, err
	}

	items, err := ctx.ExecuteQuery(query, ctx.CurrentNode)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, removeSelf(node)
	}
	it, err := applySort(ctx, node, items)
	if err != nil {
		return nil, err
	}

	parent, ok := node.Parent().(*xml.Element)
	if !ok {
		return nil, fmt.Errorf("for-each: xml element expected as parent")
	}
	for i := range it {
		value := i.Node()
		for _, n := range el.Nodes {
			c := cloneNode(n)
			if c == nil {
				continue
			}
			parent.Append(c)
			if _, err := transformNode(ctx.Sub(value), c); err != nil {
				return nil, err
			}
		}
	}
	return nil, removeSelf(node)
}

func executeTry(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	items, err := ctx.queryXSL("./catch[last()]", node)
	if err != nil {
		return nil, err
	}
	if len(items) > 1 {
		return nil, fmt.Errorf("only one catch element is allowed")
	}
	if _, err := processNode(ctx, el); err != nil {
		if len(items) > 0 {
			catch := items[0].Node()
			if err := removeNode(node, catch); err != nil {
				return nil, err
			}
			return processNode(ctx.Self(), catch)
		}
		return nil, err
	}
	return nil, nil
}

func executeIf(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	test, err := getAttribute(el, "test")
	if err != nil {
		return nil, err
	}
	ok, err := ctx.TestNode(test, ctx.CurrentNode)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, removeSelf(el)
	}
	if _, err = processNode(ctx, node); err != nil {
		return nil, err
	}
	return nil, insertNodes(el, el.Nodes...)
}

func executeChoose(ctx *Context, node xml.Node) (xml.Sequence, error) {
	items, err := ctx.queryXSL("/when", ctx.CurrentNode)
	if err != nil {
		return nil, err
	}
	for i := range items {
		n := items[i].Node().(*xml.Element)
		test, err := getAttribute(n, "test")
		if err != nil {
			return nil, err
		}
		ok, err := ctx.TestNode(test, ctx.CurrentNode)
		if err != nil {
			return nil, err
		}
		if ok {
			if _, err := processNode(ctx, n); err != nil {
				return nil, err
			}
			var (
				pt = n.Parent()
				gp = pt.Parent()
			)
			if i, ok := gp.(interface{ InsertNodes(int, []xml.Node) error }); ok {
				return nil, i.InsertNodes(pt.Position(), n.Nodes)
			}
			return nil, nil
		}
	}

	if items, err = ctx.queryXSL("otherwise", node); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	n := items[0].Node().(*xml.Element)
	if _, err := processNode(ctx, n); err != nil {
		return nil, err
	}
	var (
		pt = n.Parent()
		gp = pt.Parent()
	)
	if i, ok := gp.(interface{ InsertNodes(int, []xml.Node) error }); ok {
		return nil, i.InsertNodes(pt.Position(), n.Nodes)
	}
	return nil, nil
}

func executeValueOf(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	query, err := getAttribute(el, "select")
	if err != nil {
		return nil, err
	}
	sep, err := getAttribute(el, "separator")
	if err != nil {
		sep = " "
	}
	items, err := ctx.ExecuteQuery(query, ctx.CurrentNode)
	if err != nil || len(items) == 0 {
		return nil, removeSelf(node)
	}

	var str strings.Builder
	for i := range items {
		if i > 0 {
			str.WriteString(sep)
		}
		str.WriteString(toString(items[i]))
	}
	text := xml.NewText(str.String())
	return nil, replaceNode(node, text)
}

func executeCopy(ctx *Context, node xml.Node) (xml.Sequence, error) {
	return executeCopyOf(ctx, node)
}

func executeCopyOf(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	query, err := getAttribute(el, "select")
	if err != nil {
		return nil, err
	}
	items, err := ctx.ExecuteQuery(query, ctx.CurrentNode)
	if err != nil {
		return nil, err
	}
	var list []xml.Node
	for i := range items {
		c := cloneNode(items[i].Node())
		if c != nil {
			list = append(list, c)
		}
	}
	return nil, insertNodes(el, list...)
}

func executeMessage(ctx *Context, node xml.Node) (xml.Sequence, error) {
	var (
		parts []string
		el    = node.(*xml.Element)
	)
	for _, n := range el.Nodes {
		parts = append(parts, n.Value())
	}
	fmt.Fprintln(os.Stderr, strings.Join(parts, ""))

	if quit, err := getAttribute(el, "terminate"); err == nil && quit == "yes" {
		return nil, ErrTerminate
	}
	return nil, nil
}

func executeWherePopulated(ctx *Context, node xml.Node) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeOnEmpty(ctx *Context, node xml.Node) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeOnNotEmpty(ctx *Context, node xml.Node) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeSequence(ctx *Context, node xml.Node) (xml.Sequence, error) {
	elem := node.(*xml.Element)
	query, err := getAttribute(elem, "select")
	if err != nil {
		return nil, err
	}
	return ctx.ExecuteQuery(query, ctx.CurrentNode)
}

func executeElement(ctx *Context, node xml.Node) (xml.Sequence, error) {
	el := node.(*xml.Element)
	ident, err := getAttribute(el, "name")
	if err != nil {
		return nil, err
	}
	qn, err := xml.ParseName(ident)
	if err != nil {
		return nil, err
	}
	var (
		curr  = xml.NewElement(qn)
		nodes = slices.Clone(el.Nodes)
	)
	if err := replaceNode(el, curr); err != nil {
		return nil, err
	}
	for i := range nodes {
		c := cloneNode(nodes[i])
		if c == nil {
			continue
		}
		curr.Append(c)
		if _, err := transformNode(ctx, c); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func executeAttribute(ctx *Context, node xml.Node) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeText(ctx *Context, node xml.Node) (xml.Sequence, error) {
	text := xml.NewText(node.Value())
	return nil, replaceNode(node, text)
}

func executeComment(ctx *Context, node xml.Node) (xml.Sequence, error) {
	comment := xml.NewComment(node.Value())
	return nil, replaceNode(node, comment)
}

func executeFallback(ctx *Context, node xml.Node) (xml.Sequence, error) {
	return nil, errImplemented
}

func iterItems(items []xml.Item, orderBy, orderDir string) (iter.Seq[xml.Item], error) {
	expr, err := xml.CompileString(orderBy)
	if err != nil {
		return nil, err
	}
	getString := func(is []xml.Item) string {
		if len(is) == 0 {
			return ""
		}
		val := is[0].Value()
		return val.(string)
	}
	var compare func(string, string) bool
	if orderDir == "descending" {
		compare = func(str1, str2 string) bool {
			return strings.Compare(str1, str2) >= 0
		}
	} else {
		compare = func(str1, str2 string) bool {
			return strings.Compare(str1, str2) < 0
		}
	}
	fn := func(yield func(xml.Item) bool) {
		is := slices.Clone(items)
		sort.Slice(is, func(i, j int) bool {
			x1, err1 := expr.Find(is[i].Node())
			x2, err2 := expr.Find(is[j].Node())
			if err1 != nil || err2 != nil {
				return false
			}
			return compare(getString(x1), getString(x2))
		})
		for i := range is {
			if !yield(is[i]) {
				break
			}
		}
	}
	return fn, nil
}

func getNodesForTemplate(ctx *Context, node xml.Node) ([]xml.Node, error) {
	var (
		el  = node.(*xml.Element)
		res []xml.Node
	)
	if query, err := getAttribute(el, "select"); err == nil {
		items, err := ctx.ExecuteQuery(query, ctx.CurrentNode)
		if err != nil {
			return nil, err
		}
		for i := range items {
			res = append(res, items[i].Node())
		}
	} else {
		res = []xml.Node{cloneNode(ctx.CurrentNode)}
	}
	return res, nil
}

func applyParams(ctx *Context, node xml.Node) error {
	el := node.(*xml.Element)
	for _, n := range slices.Clone(el.Nodes) {
		if n.QualifiedName() != ctx.getQualifiedName("with-param") {
			return fmt.Errorf("%s: invalid child node %s", node.QualifiedName(), n.QualifiedName())
		}
		if _, err := transformNode(ctx, n); err != nil {
			return err
		}
	}
	return nil
}

func applySort(ctx *Context, node xml.Node, items []xml.Item) (iter.Seq[xml.Item], error) {
	sorts, err := ctx.queryXSL("./sort[1]", node)
	if err != nil {
		return nil, err
	}
	if len(sorts) == 0 {
		return slices.Values(items), nil
	}
	tmp := sorts[0].Node()
	if err := removeSelf(tmp); err != nil {
		return nil, err
	}
	elem, ok := tmp.(*xml.Element)
	if !ok {
		return nil, fmt.Errorf("sort: expected xml element")
	}
	query, err := getAttribute(elem, "select")
	if err != nil {
		return nil, err
	}
	order, _ := getAttribute(elem, "order")
	return iterItems(items, query, order)
}

type matchFunc func(xml.Node, string) (*Template, error)

func executeApply(ctx *Context, node xml.Node, match matchFunc) (xml.Sequence, error) {
	nodes, err := getNodesForTemplate(ctx, node)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, removeNode(node, node)
	}
	var (
		el      = node.(*xml.Element)
		mode, _ = getAttribute(el, "mode")
		results []xml.Node
	)
	for _, datum := range nodes {
		tpl, err := match(datum, mode)
		if err != nil {
			for i := range nodes {
				sub := ctx.Sub(nodes[i])
				if err = sub.NotFound(node, err, mode); err != nil {
					return nil, err
				}
			}
			return nil, err
		}
		sub := tpl.createContext(ctx, datum)
		if err := applyParams(sub, node); err != nil {
			return nil, err
		}
		res, err := tpl.Execute(sub)
		if err != nil {
			return nil, err
		}
		results = slices.Concat(results, res)
	}
	return nil, insertNodes(node, results...)
}

func isTrue(items []xml.Item) bool {
	if len(items) == 0 {
		return false
	}
	var ok bool
	if !items[0].Atomic() {
		return true
	}
	switch res := items[0].Value().(type) {
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

func cloneNode(n xml.Node) xml.Node {
	cloner, ok := n.(xml.Cloner)
	if !ok {
		return nil
	}
	return cloner.Clone()
}

func getAttribute(el *xml.Element, ident string) (string, error) {
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == ident
	})
	if ix < 0 {
		return "", fmt.Errorf("%s: missing attribute %q", el.QualifiedName(), ident)
	}
	return el.Attrs[ix].Value(), nil
}

func loadDocument(file string) (*xml.Document, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	p := xml.NewParser(r)
	return p.Parse()
}

func writeDoctypeHTML(w io.Writer) error {
	_, err := io.WriteString(w, "<!DOCTYPE html>")
	return err
}

func isTemplateMatch(expr xml.Expr, node xml.Node) (bool, int) {
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

func writeDocument(file, format string, doc *xml.Document, style *Stylesheet) error {
	w, err := os.Create(file)
	if err != nil {
		return err
	}
	defer w.Close()

	return style.writeDocument(w, format, doc)
}

func removeNode(elem, node xml.Node) error {
	if node == nil {
		return nil
	}
	return removeAt(elem, node.Position())
}

func removeAt(elem xml.Node, pos int) error {
	p := elem.Parent()
	r, ok := p.(interface{ RemoveNode(int) error })
	if !ok {
		return fmt.Errorf("node can not be removed from parent element of %s", elem.QualifiedName())
	}
	return r.RemoveNode(pos)
}

func removeSelf(elem xml.Node) error {
	return removeNode(elem, elem)
}

func replaceNode(elem, node xml.Node) error {
	if node == nil {
		return nil
	}
	p := elem.Parent()
	r, ok := p.(interface{ ReplaceNode(int, xml.Node) error })
	if !ok {
		return fmt.Errorf("node can not be replaced from parent element of %s", elem.QualifiedName())
	}
	return r.ReplaceNode(elem.Position(), node)
}

func insertNodes(elem xml.Node, nodes ...xml.Node) error {
	if len(nodes) == 0 {
		return nil
	}
	p := elem.Parent()
	i, ok := p.(interface{ InsertNodes(int, []xml.Node) error })
	if !ok {
		return fmt.Errorf("nodes can not be inserted to parent element of %s", elem.QualifiedName())
	}
	return i.InsertNodes(elem.Position(), nodes)
}

func toString(item xml.Item) string {
	var v string
	switch x := item.Value().(type) {
	case time.Time:
		v = x.Format("2006-01-02")
	case float64:
		v = strconv.FormatFloat(x, 'f', -1, 64)
	case []byte:
	case string:
		v = x
	default:
	}
	return v
}
