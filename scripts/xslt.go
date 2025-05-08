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

type executeFunc func(xml.Node, xml.Node, *Stylesheet) error

var executers map[xml.QName]executeFunc

func init() {
	wrap := func(exec executeFunc) executeFunc {
		fn := func(node, datum xml.Node, sheet *Stylesheet) error {
			sheet.Enter()
			defer sheet.Leave()

			return exec(node, datum, sheet)
		}
		return fn
	}
	executers = map[xml.QName]executeFunc{
		xml.QualifiedName("for-each", xsltNamespacePrefix):        wrap(executeForeach),
		xml.QualifiedName("value-of", xsltNamespacePrefix):        executeValueOf,
		xml.QualifiedName("call-template", xsltNamespacePrefix):   wrap(executeCallTemplate),
		xml.QualifiedName("apply-templates", xsltNamespacePrefix): wrap(executeApplyTemplates),
		xml.QualifiedName("apply-imports", xsltNamespacePrefix):   wrap(executeApplyImport),
		xml.QualifiedName("if", xsltNamespacePrefix):              wrap(executeIf),
		xml.QualifiedName("choose", xsltNamespacePrefix):          wrap(executeChoose),
		xml.QualifiedName("where-populated", xsltNamespacePrefix): executeWithParam,
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
		sheet.DefineExprParam(ident, expr)
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

type MatchMode int8

const (
	MatchDeepCopy MatchMode = 1 << iota
	MatchShallowCopy
	MatchDeepSkip
	MatchShallowSkip
	MatchTextOnlyCopy
	MatchFail
)

type AttributeSet struct {
	Name  string
	Attrs []xml.Attribute
}

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

type Stylesheet struct {
	namespace   string
	Mode        string
	currentMode *Mode
	Modes       []*Mode
	AttrSet     []*AttributeSet

	vars     xml.Environ[xml.Expr]
	params   xml.Environ[xml.Expr]
	builtins xml.Environ[xml.BuiltinFunc]

	output    []*OutputSettings
	Templates []*Template

	Context  string
	Imported bool
	Others   []*Stylesheet
}

func Load(file, contextDir string) (*Stylesheet, error) {
	doc, err := loadDocument(file)
	if err != nil {
		return nil, err
	}
	sheet := Stylesheet{
		vars:        xml.Empty[xml.Expr](),
		params:      xml.Empty[xml.Expr](),
		builtins:    xml.DefaultBuiltin(),
		Context:     contextDir,
		currentMode: &unnamedMode,
		namespace:   xsltNamespacePrefix,
	}
	if err := sheet.init(doc); err != nil {
		return nil, err
	}

	root := doc.Root().(*xml.Element)
	if root != nil {
		all := root.Namespaces()
		ix := slices.IndexFunc(all, func(n xml.NS) bool {
			return n.Uri == xsltNamespaceUri
		})
		if ix >= 0 {
			sheet.namespace = all[ix].Prefix
		}
	}

	return &sheet, nil
}

func defaultOutput() *OutputSettings {
	out := &OutputSettings{
		Method:   "xml",
		Version:  "1.0",
		Encoding: "UTF-8",
		Indent:   true,
	}
	return out
}

func includesSheet(sheet *Stylesheet, doc xml.Node) error {
	items, err := sheet.queryXSL("/stylesheet/include", doc)
	if err != nil {
		return err
	}
	for _, i := range items {
		err := executeInclude(i.Node(), doc, sheet)
		if err != nil {
			return err
		}
	}
	return nil
}

func importsSheet(sheet *Stylesheet, doc xml.Node) error {
	items, err := sheet.queryXSL("/stylesheet/import", doc)
	if err != nil {
		return err
	}
	for _, i := range items {
		err := executeImport(i.Node(), doc, sheet)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Stylesheet) loadAttributeSet(doc xml.Node) error {
	items, err := s.queryXSL("/stylesheet/attribute-set", doc)
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
	items, err := s.queryXSL("/stylesheet/mode", doc)
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
			case "warning-on-multiple-match":
			default:
			}
		}
		s.Modes = append(s.Modes, &m)
	}
	return nil
}

func (s *Stylesheet) loadParams(doc xml.Node) error {
	items, err := s.queryXSL("/stylesheet/param", doc)
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
			s.params.Define(ident, expr)
		} else {
			if len(n.Nodes) != 1 {
				return fmt.Errorf("only one node expected")
			}
			n := cloneNode(n.Nodes[0])
			s.params.Define(ident, xml.NewValueFromNode(n))
		}
	}
	return nil
}

func (s *Stylesheet) loadOutput(doc xml.Node) error {
	items, err := s.queryXSL("/stylesheet/output", doc)
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
	items, err := s.queryXSL("/stylesheet/template", doc)
	if err != nil {
		return err
	}
	for _, el := range items {
		el, ok := el.Node().(*xml.Element)
		if !ok {
			continue
		}
		t := Template{
			Fragment: el,
		}
		for _, a := range el.Attrs {
			switch attr := a.Value(); a.Name {
			case "priority":
				p, err := strconv.Atoi(attr)
				if err != nil {
					return err
				}
				t.Priority = p
			case "name":
				t.Name = attr
			case "match":
				t.Match = attr
				if t.Match == "" {
					t.Match = "."
				}
			case "mode":
				t.Mode = attr
			default:
			}
		}
		s.Templates = append(s.Templates, &t)
	}
	return nil
}

func (s *Stylesheet) ExecuteQuery(query string, datum xml.Node) ([]xml.Item, error) {
	return s.ExecuteQueryWithNS(query, "", datum)
}

func (s *Stylesheet) ExecuteQueryWithNS(query, ns string, datum xml.Node) ([]xml.Item, error) {
	q, err := s.CompileQueryWithNS(query, ns)
	if err != nil {
		return nil, err
	}
	// e, ok := q.(interface {
	// 	FindWithEnv(xml.Node, xml.Environ[xml.Expr]) ([]xml.Item, error)
	// })
	// if ok {
	// 	return e.FindWithEnv(datum, s)
	// }
	return q.Find(datum)
}

func (s *Stylesheet) queryXSL(query string, datum xml.Node) ([]xml.Item, error) {
	return s.ExecuteQueryWithNS(query, s.namespace, datum)
}

func (s *Stylesheet) CompileQuery(query string) (xml.Expr, error) {
	return s.CompileQueryWithNS(query, "")
}

func (s *Stylesheet) CompileQueryWithNS(query, ns string) (xml.Expr, error) {
	q, err := xml.Build(query)
	if err != nil {
		return nil, err
	}
	q.Environ = s
	q.Builtins = xml.DefaultBuiltin()
	if ns != "" {
		q.UseNamespace(ns)
	}
	return q, nil
}

func (s *Stylesheet) TestNode(query string, datum xml.Node) (bool, error) {
	items, err := s.ExecuteQuery(query, datum)
	if err != nil {
		return false, err
	}
	return isTrue(items), nil
}

func (s *Stylesheet) Generate(w io.Writer, doc *xml.Document) error {
	result, err := s.Execute(doc)
	if err != nil {
		return err
	}
	writer := xml.NewWriter(w)
	if len(s.output) > 0 {
		out := s.output[0]
		if !out.Indent {
			writer.WriterOptions |= xml.OptionCompact
		}
		if out.OmitProlog {
			writer.WriterOptions |= xml.OptionNoProlog
		}
		if out.Method == "html" {
			writer.PrologWriter = xml.PrologWriterFunc(writeDoctypeHTML)
		}
	}
	writer.Write(result.(*xml.Document))
	return nil
}

func (s *Stylesheet) ImportSheet(file string) error {
	file = filepath.Join(s.Context, file)
	other, err := Load(file, s.Context)
	if err != nil {
		return err
	}
	other.Imported = true
	s.Others = append(s.Others, other)
	return nil
}

func (s *Stylesheet) IncludeSheet(file string) error {
	file = filepath.Join(s.Context, file)
	other, err := Load(file, s.Context)
	if err != nil {
		return err
	}
	s.Templates = append(s.Templates, other.Templates...)
	if m, ok := s.vars.(interface{ Merge(xml.Environ[xml.Expr]) }); ok {
		m.Merge(other.vars)
	}
	if m, ok := s.params.(interface{ Merge(xml.Environ[xml.Expr]) }); ok {
		m.Merge(other.params)
	}
	return nil
}

func (s *Stylesheet) Enter() {
	s.vars = xml.Enclosed[xml.Expr](s.vars)
	s.params = xml.Enclosed[xml.Expr](s.params)
}

func (s *Stylesheet) Leave() {
	if u, ok := s.vars.(interface{ Unwrap() xml.Environ[xml.Expr] }); ok {
		s.vars = u.Unwrap()
	}
	if u, ok := s.params.(interface{ Unwrap() xml.Environ[xml.Expr] }); ok {
		s.params = u.Unwrap()
	}
}

func (s *Stylesheet) Resolve(ident string) (xml.Expr, error) {
	expr, err := s.vars.Resolve(ident)
	if err == nil {
		return expr, nil
	}
	expr, err = s.params.Resolve(ident)
	if err == nil {
		return expr, nil
	}
	for i := range s.Others {
		e, err := s.Others[i].Resolve(ident)
		if err == nil {
			return e, nil
		}
	}
	return nil, nil
}

func (s *Stylesheet) Define(param string, expr xml.Expr) {
	s.vars.Define(param, expr)
}

func (s *Stylesheet) DefineParam(param, value string) error {
	expr, err := s.CompileQuery(value)
	if err != nil {
		return err
	}
	s.DefineExprParam(param, expr)
	return nil
}

func (s Stylesheet) EvalParam(param, query string, datum xml.Node) error {
	items, err := s.ExecuteQuery(query, datum)
	if err != nil {
		return err
	}
	if len(items) != 1 {
		return fmt.Errorf("invalid sequence")
	}
	s.DefineExprParam(param, xml.NewValue(items[0]))
	return nil
}

func (s *Stylesheet) DefineExprParam(param string, expr xml.Expr) {
	s.params.Define(param, expr)
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
	s.prepare(tpl)
	return tpl, nil
}

func (s *Stylesheet) Match(node xml.Node, mode string) (*Template, error) {
	if mode == "" {
		mode = s.CurrentMode()
	}
	type TemplateMatch struct {
		*Template
		Priority int
	}
	var results []*TemplateMatch
	for _, t := range s.Templates {
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
		s.prepare(tpl)
		return tpl, nil
	}
	for _, s := range s.Others {
		if tpl, err := s.Match(node, mode); err == nil {
			return tpl, err
		}
	}
	return nil, fmt.Errorf("no template found matching given node (%s)", node.QualifiedName())
}

func (s *Stylesheet) Execute(doc xml.Node) (xml.Node, error) {
	ix := slices.IndexFunc(s.Templates, func(t *Template) bool {
		return t.isRoot() && t.Mode == s.CurrentMode()
	})
	if ix < 0 {
		return nil, fmt.Errorf("main template not found")
	}

	if d, ok := doc.(*xml.Document); ok {
		doc = d.Root()
	}

	root, err := s.Templates[ix].Execute(doc, s)
	if err == nil {
		var doc xml.Document
		doc.Nodes = append(doc.Nodes, root...)
		return &doc, nil
	}
	return nil, err
}

func (s *Stylesheet) prepare(tpl *Template) error {
	el := tpl.Fragment.(*xml.Element)
	if el == nil {
		return fmt.Errorf("template: fragment expected xml element")
	}
	for _, n := range slices.Clone(el.Nodes) {
		if n.QualifiedName() != s.getQualifiedName("param") {
			break
		}
		e := n.(*xml.Element)
		if e == nil {
			continue
		}
		ident, err := getAttribute(e, "name")
		if err != nil {
			return err
		}
		if value, err := getAttribute(e, "select"); err == nil {
			if err := s.DefineParam(ident, value); err != nil {
				return err
			}
		}
		el.RemoveNode(n.Position())
	}
	return nil
}

type Template struct {
	Name     string
	Match    string
	Mode     string
	Priority int
	Fragment xml.Node
}

func (t *Template) Clone() *Template {
	var tpl Template
	tpl.Fragment = cloneNode(t.Fragment)
	tpl.Name = t.Name
	return &tpl
}

func (t *Template) Execute(datum xml.Node, style *Stylesheet) ([]xml.Node, error) {
	value, err := t.getData(datum)
	if err != nil {
		return nil, err
	}
	el, ok := t.Fragment.(*xml.Element)
	if !ok {
		return nil, fmt.Errorf("template: xml element expected")
	}
	var nodes []xml.Node
	for _, n := range slices.Clone(el.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		if err := t.execute(c, value, style); err != nil {
			if errors.Is(err, errSkip) {
				continue
			}
			return nil, err
		}
		nodes = append(nodes, c)
	}
	return nodes, nil
}

func (t *Template) execute(current, datum xml.Node, style *Stylesheet) error {
	return transformNode(current, datum, style)
}

func (t *Template) getData(datum xml.Node) (xml.Node, error) {
	if t.Match == "" {
		return datum, nil
	}
	query, err := xml.CompileString(t.Match)
	if err != nil {
		return nil, err
	}
	items, err := query.Find(datum)
	if err != nil {
		return nil, err
	}
	if len(items) != 1 {
		return nil, fmt.Errorf("template: too many result returned by query")
	}
	return items[0].Node(), nil
}

func (t *Template) isRoot() bool {
	return t.Match == "/"
}

func transformNode(node, datum xml.Node, style *Stylesheet) error {
	el, ok := node.(*xml.Element)
	if !ok {
		return fmt.Errorf("node: xml element expected (got %T)", el)
	}
	if el.Space == style.namespace {
		el.Space = xsltNamespacePrefix
		fn, ok := executers[el.QName]
		if ok {
			if fn == nil {
				return fmt.Errorf("%s not yet implemented", el.QName)
			}
			return fn(node, datum, style)
		}
	}
	return processNode(node, datum, style)
}

func processAVT(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	for i, a := range el.Attrs {
		var (
			value = a.Value()
			parts []string
		)
		for q, ok := range iterAVT(value) {
			if !ok {
				parts = append(parts, q)
				continue
			}
			items, err := style.ExecuteQuery(q, datum)
			if err != nil {
				return err
			}
			if len(items) != 1 {
				return fmt.Errorf("invalid sequence")
			}
			v := items[0].Value().(string)
			parts = append(parts, v)
		}
		el.Attrs[i].Datum = strings.Join(parts, "")
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

func processNode(node, datum xml.Node, style *Stylesheet) error {
	var (
		el    = node.(*xml.Element)
		nodes = slices.Clone(el.Nodes)
	)
	if err := processAVT(node, datum, style); err != nil {
		return err
	}
	if ident, err := getAttribute(el, "use-attribute-sets"); err == nil {
		ix := slices.IndexFunc(style.AttrSet, func(set *AttributeSet) bool {
			return set.Name == ident
		})
		if ix < 0 {
			return fmt.Errorf("attribute-set not defined")
		}
		for _, a := range style.AttrSet[ix].Attrs {
			el.SetAttribute(a)
		}
		// el.RemoveAttr(el.Attrs[ix].Position())
	}
	for i := range nodes {
		if nodes[i].Type() != xml.TypeElement {
			continue
		}
		err := transformNode(nodes[i], datum, style)
		if err != nil {
			return err
		}
	}
	return nil
}

func executeVariable(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	ident, err := getAttribute(el, "name")
	if err != nil {
		return err
	}
	if value, err := getAttribute(el, "select"); err == nil {
		query, err := style.CompileQuery(value)
		if err != nil {
			return err
		}
		style.Define(ident, query)
	} else {
		if len(el.Nodes) != 1 {
			return fmt.Errorf("only one node expected")
		}
		n := cloneNode(el.Nodes[0])
		style.Define(ident, xml.NewValueFromNode(n))
	}
	if r, ok := el.Parent().(interface{ RemoveNode(int) error }); ok {
		return r.RemoveNode(el.Position())
	}
	return nil
}

func executeImport(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	file, err := getAttribute(el, "href")
	if err != nil {
		return err
	}
	return style.ImportSheet(file)
}

func executeInclude(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	file, err := getAttribute(el, "href")
	if err != nil {
		return err
	}
	return style.IncludeSheet(file)
}

func executeSourceDocument(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	file, err := getAttribute(el, "href")
	if err != nil {
		return err
	}
	doc, err := loadDocument(filepath.Join(style.Context, file))
	if err != nil {
		return err
	}
	var nodes []xml.Node
	for _, n := range slices.Clone(el.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		if err := transformNode(n, doc, style); err != nil {
			return err
		}
		nodes = append(nodes, c)
	}
	if i, ok := el.Parent().(interface{ InsertNodes(int, []xml.Node) error }); ok {
		if err := i.InsertNodes(el.Position(), nodes); err != nil {
			return err
		}
	}
	return nil
}

func executeResultDocument(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	var (
		file string
		outn string
		err  error
	)
	file, err = getAttribute(el, "href")
	if err != nil {
		return err
	}
	outn, _ = getAttribute(el, "format")
	var doc xml.Document
	for _, n := range slices.Clone(el.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		if err := transformNode(c, datum, style); err != nil {
			return err
		}
		doc.Nodes = append(doc.Nodes, c)
	}

	if r, ok := el.Parent().(interface{ RemoveNode(int) error }); ok {
		if err := r.RemoveNode(el.Position()); err != nil {
			return err
		}
	}

	w, err := os.Create(file)
	if err != nil {
		return err
	}
	defer w.Close()

	writer := xml.NewWriter(w)
	if out := style.GetOutput(outn); out != nil {
		if !out.Indent {
			writer.WriterOptions |= xml.OptionCompact
		}
		if out.OmitProlog {
			writer.WriterOptions |= xml.OptionNoProlog
		}
		if out.Method == "html" && (out.Version == "5" || out.Version == "5.0") {
			writer.PrologWriter = xml.PrologWriterFunc(writeDoctypeHTML)
		}
	}
	writer.Write(&doc)
	return errSkip
}

func executeApplyImport(node, datum xml.Node, style *Stylesheet) error {
	return nil
}

func executeApplyTemplates(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	if value, err := getAttribute(el, "select"); err == nil {
		query, err := style.CompileQuery(value)
		if err != nil {
			return err
		}
		items, err := query.Find(datum)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			if r, ok := node.Parent().(interface{ RemoveNode(int) error }); ok {
				return r.RemoveNode(node.Position())
			}
		}
		datum = items[0].Node()
	}
	mode, err := getAttribute(el, "mode")
	if err != nil {
		mode = style.CurrentMode()
	}
	tpl, err := style.Match(datum, mode)
	if err != nil {
		return err
	}
	for _, n := range el.Nodes {
		if n.QualifiedName() != "xsl:with-param" {
			return fmt.Errorf("apply-templates: invalid child node %s", n.QualifiedName())
		}
		if err := transformNode(n, datum, style); err != nil {
			return err
		}
	}
	var (
		parent = el.Parent().(*xml.Element)
		frag   = tpl.Fragment.(*xml.Element)
		nodes  []xml.Node
	)
	for _, n := range slices.Clone(frag.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		if err := transformNode(c, datum, style); err != nil {
			return err
		}
		nodes = append(nodes, c)
	}
	parent.InsertNodes(el.Position(), nodes)
	return nil
}

func executeCallTemplate(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	name, err := getAttribute(el, "name")
	if err != nil {
		return err
	}
	mode, err := getAttribute(el, "mode")
	if err != nil {
		mode = style.CurrentMode()
	}
	tpl, err := style.Find(name, mode)
	if err != nil {
		return err
	}

	for _, n := range el.Nodes {
		if n.QualifiedName() != "xsl:with-param" {
			return fmt.Errorf("call-templates: invalid child node %s", n.QualifiedName())
		}
		if err := transformNode(n, datum, style); err != nil {
			return err
		}
	}

	nodes, err := tpl.Execute(datum, style)
	if err != nil {
		return err
	}
	if i, ok := el.Parent().(interface{ InsertNodes(int, []xml.Node) error }); ok {
		if err := i.InsertNodes(el.Position(), nodes); err != nil {
			return err
		}
	}
	return nil
}

func executeWithParam(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	ident, err := getAttribute(el, "name")
	if err != nil {
		return err
	}
	value, err := getAttribute(el, "select")
	if err != nil {
		return err
	}
	return style.EvalParam(ident, value, datum)
}

func executeTry(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	ix := slices.IndexFunc(el.Nodes, func(n xml.Node) bool {
		return n.QualifiedName() == "xsl:catch"
	})
	var catch xml.Node
	if ix != -1 && ix != len(el.Nodes)-1 {
		return fmt.Errorf("xsl:try: xsl:catch should be the last element")
	}
	if ix >= 0 {
		catch = el.Nodes[ix]
		el.RemoveNode(ix)
	}
	if err := processNode(el, datum, style); err != nil {
		if catch != nil {
			style.Enter()
			defer style.Leave()
			return processNode(el, datum, style)
		}
	}
	return nil
}

func executeWherePopulated(node, datum xml.Node, style *Stylesheet) error {
	return errImplemented
}

func executeOnEmpty(node, datum xml.Node, style *Stylesheet) error {
	return errImplemented
}

func executeOnNotEmpty(node, datum xml.Node, style *Stylesheet) error {
	return errImplemented
}

func executeIf(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	test, err := getAttribute(el, "test")
	if err != nil {
		return err
	}
	ok, err := style.TestNode(test, datum)
	if err != nil {
		return err
	}
	if !ok {
		if r, ok := el.Parent().(interface{ RemoveNode(int) error }); ok {
			return r.RemoveNode(el.Position())
		}
		return nil
	}
	if err = processNode(node, datum, style); err != nil {
		return err
	}
	if i, ok := el.Parent().(interface{ InsertNodes(int, []xml.Node) error }); ok {
		return i.InsertNodes(el.Position(), el.Nodes)
	}
	return nil
}

func executeChoose(node, datum xml.Node, style *Stylesheet) error {
	query, err := style.CompileQuery("./xsl:when")
	if err != nil {
		return err
	}
	items, err := query.Find(node)
	if err != nil {
		return err
	}
	for i := range items {
		n := items[i].Node().(*xml.Element)
		test, err := getAttribute(n, "test")
		if err != nil {
			return err
		}
		ok, err := style.TestNode(test, datum)
		if err != nil {
			return err
		}
		if ok {
			if err := processNode(n, datum, style); err != nil {
				return err
			}
			var (
				pt = n.Parent()
				gp = pt.Parent()
			)
			if i, ok := gp.(interface{ InsertNodes(int, []xml.Node) error }); ok {
				return i.InsertNodes(pt.Position(), n.Nodes)
			}
			return nil
		}
	}

	if query, err = style.CompileQuery("./xsl:otherwise"); err != nil {
		return err
	}
	if items, err = query.Find(node); err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	n := items[0].Node().(*xml.Element)
	if err := processNode(n, datum, style); err != nil {
		return err
	}
	var (
		pt = n.Parent()
		gp = pt.Parent()
	)
	if i, ok := gp.(interface{ InsertNodes(int, []xml.Node) error }); ok {
		return i.InsertNodes(pt.Position(), n.Nodes)
	}
	return nil
}

func executeForeach(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	query, err := getAttribute(el, "select")
	if err != nil {
		return err
	}
	parent, ok := el.Parent().(*xml.Element)
	if !ok {
		return fmt.Errorf("for-each: xml element expected as parent")
	}
	parent.RemoveNode(el.Position())

	items, err := style.ExecuteQuery(query, datum)
	if err != nil || len(items) == 0 {
		return err
	}

	var it iter.Seq[xml.Item]
	if el.Nodes[0].QualifiedName() == "xsl:sort" {
		x := el.Nodes[0].(*xml.Element)
		query, err := getAttribute(x, "select")
		if err != nil {
			return err
		}
		order, _ := getAttribute(x, "order")
		el.RemoveNode(0)
		it, err = foreachItems(items, query, order)
		if err != nil {
			return err
		}
	} else {
		it = slices.Values(items)
	}

	for i := range it {
		value := i.Node()
		for _, n := range el.Nodes {
			c := cloneNode(n)
			if c == nil {
				continue
			}
			parent.Append(c)
			if err := transformNode(c, value, style); err != nil {
				return err
			}
		}
	}
	return nil
}

func foreachItems(items []xml.Item, orderBy, orderDir string) (iter.Seq[xml.Item], error) {
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

func executeValueOf(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	query, err := getAttribute(el, "select")
	if err != nil {
		return err
	}
	sep, err := getAttribute(el, "separator")
	if err != nil {
		sep = " "
	}
	parent, ok := el.Parent().(*xml.Element)
	if !ok {
		return fmt.Errorf("value-of: xml element expected as parent")
	}
	items, err := style.ExecuteQuery(query, datum)
	if err != nil || len(items) == 0 {
		return parent.RemoveNode(el.Position())
	}

	var parts []string
	for i := range items {
		parts = append(parts, items[i].Node().Value())
	}

	text := xml.NewText(strings.Join(parts, sep))
	parent.ReplaceNode(el.Position(), text)
	return nil
}

func executeCopy(node, datum xml.Node, style *Stylesheet) error {
	return executeCopyOf(node, datum, style)
}

func executeCopyOf(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	query, err := getAttribute(el, "select")
	if err != nil {
		return err
	}
	items, err := style.ExecuteQuery(query, datum)
	if err != nil {
		return err
	}
	var list []xml.Node
	for i := range items {
		c := cloneNode(items[i].Node())
		if c != nil {
			list = append(list, c)
		}
	}
	parent, ok := el.Parent().(*xml.Element)
	if !ok {
		return fmt.Errorf("xml element expected")
	}
	return parent.InsertNodes(el.Position(), list)
}

func executeSequence(node, datum xml.Node, style *Stylesheet) error {
	return errImplemented
}

func executeMessage(node, datum xml.Node, style *Stylesheet) error {
	var (
		parts []string
		el    = node.(*xml.Element)
	)
	for _, n := range el.Nodes {
		parts = append(parts, n.Value())
	}
	fmt.Fprintln(os.Stderr, strings.Join(parts, ""))

	if quit, err := getAttribute(el, "terminate"); err == nil && quit == "yes" {
		return ErrTerminate
	}
	return nil
}

func executeElement(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	ident, err := getAttribute(el, "name")
	if err != nil {
		return err
	}
	qn, err := xml.ParseName(ident)
	if err != nil {
		return err
	}
	var (
		curr  = xml.NewElement(qn)
		nodes = slices.Clone(el.Nodes)
	)
	if r, ok := el.Parent().(interface{ ReplaceNode(int, xml.Node) error }); ok {
		r.ReplaceNode(el.Position(), curr)
	}
	for i := range nodes {
		c := cloneNode(nodes[i])
		if c == nil {
			continue
		}
		curr.Append(c)
		if err := transformNode(c, datum, style); err != nil {
			return err
		}
	}
	return nil
}

func executeAttribute(node, datum xml.Node, style *Stylesheet) error {
	return errImplemented
}

func executeText(node, datum xml.Node, style *Stylesheet) error {
	text := xml.NewText(node.Value())
	if r, ok := node.Parent().(interface{ ReplaceAt(int, xml.Node) error }); ok {
		return r.ReplaceAt(node.Position(), text)
	}
	return nil
}

func executeComment(node, datum xml.Node, style *Stylesheet) error {
	comment := xml.NewComment(node.Value())
	if r, ok := node.Parent().(interface{ ReplaceAt(int, xml.Node) error }); ok {
		return r.ReplaceAt(node.Position(), comment)
	}
	return nil
}

func executeFallback(node, datum xml.Node, style *Stylesheet) error {
	return errImplemented
}

func executeForeachGroup(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	query, err := getAttribute(el, "select")
	if err != nil {
		return err
	}
	parent, ok := el.Parent().(*xml.Element)
	if !ok {
		return fmt.Errorf("for-each-group: xml element expected as parent")
	}
	parent.RemoveNode(el.Position())

	items, err := style.ExecuteQuery(query, datum)
	if err != nil || len(items) == 0 {
		return err
	}

	key, err := getAttribute(el, "group-by")
	if err != nil {
		return err
	}
	grpby, err := style.CompileQuery(key)
	if err != nil {
		return err
	}
	groups := make(map[string][]xml.Item)
	for i := range items {
		is, err := grpby.Find(items[i].Node())
		if err != nil {
			return err
		}
		key := is[0].Value().(string)
		groups[key] = append(groups[key], items[i])
	}

	for key, items := range groups {
		currentGrp := func(_ xml.Context, _ []xml.Expr) ([]xml.Item, error) {
			return items, nil
		}
		currentKey := func(_ xml.Context, _ []xml.Expr) ([]xml.Item, error) {
			i := xml.NewLiteralItem(key)
			return []xml.Item{i}, nil
		}
		style.builtins.Define("current-group", currentGrp)
		style.builtins.Define("fn:current-group", currentGrp)
		style.builtins.Define("current-grouping-key", currentKey)
		style.builtins.Define("fn:current-grouping-key", currentKey)

		for _, n := range el.Nodes {
			c := cloneNode(n)
			if c == nil {
				continue
			}
			parent.Append(c)
			if err := transformNode(c, datum, style); err != nil {
				return err
			}
		}
	}
	return nil
}

func executeMerge(node, datum xml.Node, style *Stylesheet) error {
	return errImplemented
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
