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

var (
	errImplemented = errors.New("not implemented")
	errUndefined   = errors.New("undefined")
	errSkip        = errors.New("skip")
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
		xml.QualifiedName("for-each", "xsl"):        wrap(executeForeach),
		xml.QualifiedName("value-of", "xsl"):        executeValueOf,
		xml.QualifiedName("call-template", "xsl"):   wrap(executeCallTemplate),
		xml.QualifiedName("apply-templates", "xsl"): wrap(executeApplyTemplates),
		xml.QualifiedName("apply-imports", "xsl"):   wrap(executeApplyImport),
		xml.QualifiedName("if", "xsl"):              wrap(executeIf),
		xml.QualifiedName("choose", "xsl"):          wrap(executeChoose),
		xml.QualifiedName("where-populated", "xsl"): executeWithParam,
		xml.QualifiedName("on-empty", "xsl"):        executeOnEmpty,
		xml.QualifiedName("on-not-empty", "xsl"):    executeOnNotEmpty,
		xml.QualifiedName("try", "xsl"):             wrap(executeTry),
		xml.QualifiedName("variable", "xsl"):        executeVariable,
		xml.QualifiedName("result-document", "xsl"): executeResultDocument,
		xml.QualifiedName("source-document", "xsl"): executeSourceDocument,
		xml.QualifiedName("import", "xsl"):          executeImport,
		xml.QualifiedName("include", "xsl"):         executeInclude,
		xml.QualifiedName("with-param", "xsl"):      executeWithParam,
		xml.QualifiedName("copy", "xsl"):            executeCopy,
		xml.QualifiedName("copy-of", "xsl"):         executeCopyOf,
		xml.QualifiedName("sequence", "xsl"):        executeSequence,
		xml.QualifiedName("element", "xsl"):         executeElement,
		xml.QualifiedName("attribute", "xsl"):       executeAttribute,
		xml.QualifiedName("text", "xsl"):            executeText,
		xml.QualifiedName("comment", "xsl"):         executeComment,
		xml.QualifiedName("message", "xsl"):         executeMessage,
		xml.QualifiedName("fallback", "xsl"):        executeFallback,
		xml.QualifiedName("merge", "xsl"):           executeMerge,
		xml.QualifiedName("for-each-group", "xsl"):  executeForeachGroup,
	}
}

func main() {
	var (
		quiet  = flag.Bool("q", false, "quiet")
		mode   = flag.String("m", "", "mode")
		file   = flag.String("f", "", "file")
		dir    = flag.String("d", ".", "context directory")
		params []string
	)
	flag.Func("p", "template parameter", func(str string) error {
		_, _, ok := strings.Cut(str, "=")
		if !ok {
			return fmt.Errorf("invalid parameter")
		}
		params = append(params, str)
		return nil
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

	for _, p := range params {
		k, v, _ := strings.Cut(p, "=")
		sheet.DefineParam(k, v)
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
	}
	if err = sheet.loadTemplates(doc); err != nil {
		return nil, err
	}

	if err := sheet.loadOutput(doc); err != nil {
		return nil, err
	}
	if err = sheet.loadParams(doc); err != nil {
		return nil, err
	}
	if err = sheet.loadAttributeSet(doc); err != nil {
		return nil, err
	}
	if err := includesSheet(&sheet, doc); err != nil {
		return nil, err
	}
	if err := importsSheet(&sheet, doc); err != nil {
		return nil, err
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
	query, err := sheet.CompileQuery("/xsl:stylesheet/xsl:include")
	if err != nil {
		return err
	}
	items, err := query.Find(doc)
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
	query, err := sheet.CompileQuery("/xsl:stylesheet/xsl:import")
	if err != nil {
		return err
	}
	items, err := query.Find(doc)
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
	query, err := s.CompileQuery("/xsl:stylesheet/xsl:attribute-set")
	if err != nil {
		return err
	}
	items, err := query.Find(doc)
	if err != nil {
		return err
	}
	for i := range items {
		n := items[i].Node().(*xml.Element)
		if n == nil {
			continue
		}
		ix := slices.IndexFunc(n.Attrs, func(a xml.Attribute) bool {
			return a.Name == "name"
		})
		if ix < 0 {
			return fmt.Errorf("attribute-set: missing name attribute")
		}
		as := AttributeSet{
			Name: n.Attrs[ix].Value(),
		}
		for _, n := range n.Nodes {
			n := n.(*xml.Element)
			if n == nil {
				continue
			}
			ix := slices.IndexFunc(n.Attrs, func(a xml.Attribute) bool {
				return a.Name == "name"
			})
			if ix < 0 {
				return fmt.Errorf("attribute: missing name attribute")
			}
			attr := xml.NewAttribute(xml.LocalName(n.Attrs[ix].Value()), n.Value())
			as.Attrs = append(as.Attrs, attr)
		}
		s.AttrSet = append(s.AttrSet, &as)
	}
	return nil
}

func (s *Stylesheet) loadModes(doc xml.Node) error {
	query, err := s.CompileQuery("/xsl:stylesheet/xsl:mode")
	if err != nil {
		return err
	}
	items, err := query.Find(doc)
	if err != nil {
		return err
	}
	s.Modes = append(s.Modes, &unnamedMode)
	if len(items) == 0 {
		return nil
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
	query, err := s.CompileQuery("/xsl:stylesheet/xsl:param")
	if err != nil {
		return err
	}
	items, err := query.Find(doc)
	if err != nil {
		return err
	}
	for i := range items {
		n := items[i].Node().(*xml.Element)
		if n == nil {
			continue
		}
		ix := slices.IndexFunc(n.Attrs, func(a xml.Attribute) bool {
			return a.Name == "name"
		})
		if ix < 0 {
			return fmt.Errorf("param: missing name attribute")
		}
		ident := n.Attrs[ix].Value()
		ix = slices.IndexFunc(n.Attrs, func(a xml.Attribute) bool {
			return a.Name == "select"
		})
		if ix >= 0 {
			expr, err := s.CompileQuery(n.Attrs[ix].Value())
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
	items, err := s.ExecuteQuery("/xsl:stylesheet/xsl:output", doc)
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

func (s *Stylesheet) loadTemplates(doc xml.Node) error {
	items, err := s.ExecuteQuery("/xsl:stylesheet/xsl:template", doc)
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
	q, err := s.CompileQuery(query)
	if err != nil {
		return nil, err
	}
	e, ok := q.(interface {
		FindWithEnv(xml.Node, xml.Environ[xml.Expr]) ([]xml.Item, error)
	})
	if ok {
		return e.FindWithEnv(datum, s)
	}
	return q.Find(datum)
}

func (s *Stylesheet) CompileQuery(query string) (xml.Expr, error) {
	q, err := xml.Build(query)
	if err != nil {
		return nil, err
	}
	q.Environ = s
	q.Builtins = xml.DefaultBuiltin()
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
	s.params.Define(param, expr)
	return nil
}

func (s *Stylesheet) GetOutput(name string) *OutputSettings {
	ix := slices.IndexFunc(s.output, func(o *OutputSettings) bool {
		return o.Name == name
	})
	if ix < 0 && name != "" {
		return s.GetOutput("")
	}
	return s.output[ix]
}

func (s *Stylesheet) CurrentMode() string {
	return s.Mode
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
	hasMatch := func(expr xml.Expr) (bool, int) {
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
		ok, prio := hasMatch(expr)
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
		if n.QualifiedName() != "xsl:param" {
			break
		}
		e := n.(*xml.Element)
		if e == nil {
			continue
		}
		ix := slices.IndexFunc(e.Attrs, func(a xml.Attribute) bool {
			return a.Name == "name"
		})
		if ix < 0 {
			return fmt.Errorf("param: missing name attribute")
		}
		ident := e.Attrs[ix].Value()

		ix = slices.IndexFunc(e.Attrs, func(a xml.Attribute) bool {
			return a.Name == "select"
		})
		if ix >= 0 {
			if err := s.DefineParam(ident, e.Attrs[ix].Value()); err != nil {
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
	fn, ok := executers[el.QName]
	if ok {
		if fn == nil {
			return fmt.Errorf("%s not yet implemented", el.QName)
		}
		return fn(node, datum, style)
	}
	return processNode(node, datum, style)
}

func processNode(node, datum xml.Node, style *Stylesheet) error {
	var (
		el    = node.(*xml.Element)
		nodes = slices.Clone(el.Nodes)
	)
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.QName == xml.QualifiedName("use-attribute-sets", "xsl")
	})
	if ix >= 0 {
		ax := slices.IndexFunc(style.AttrSet, func(set *AttributeSet) bool {
			return set.Name == el.Attrs[ix].Value()
		})
		if ax < 0 {
			return fmt.Errorf("attribute-set not defined")
		}
		for _, a := range style.AttrSet[ax].Attrs {
			el.SetAttribute(a)
		}
		el.RemoveAttr(el.Attrs[ix].Position())
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
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "name"
	})
	if ix < 0 {
		return fmt.Errorf("variable: missing required name attribute")
	}
	ident := el.Attrs[ix].Value()
	ix = slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "select"
	})
	if ix >= 0 {
		query, err := style.CompileQuery(el.Attrs[ix].Value())
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
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "href"
	})
	if ix < 0 {
		return fmt.Errorf("import: missing href attribute")
	}
	return style.ImportSheet(el.Attrs[ix].Value())
}

func executeInclude(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "href"
	})
	if ix < 0 {
		return fmt.Errorf("include: missing href attribute")
	}
	return style.IncludeSheet(el.Attrs[ix].Value())
}

func executeSourceDocument(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "href"
	})
	if ix < 0 {
		return fmt.Errorf("source-document: missing href attribute")
	}
	doc, err := loadDocument(filepath.Join(style.Context, el.Attrs[ix].Value()))
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
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "href"
	})
	var (
		file string
		outn string
	)
	if ix < 0 {
		return fmt.Errorf("result-document: missing href attribute")
	}
	file = el.Attrs[ix].Value()

	ix = slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "format"
	})
	if ix >= 0 {
		outn = el.Attrs[ix].Value()
	}
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
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "select"
	})
	if ix >= 0 {
		query, err := style.CompileQuery(el.Attrs[ix].Value())
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
	ix = slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "mode"
	})
	mode := style.Mode
	if ix >= 0 {
		mode = el.Attrs[ix].Value()
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
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "name"
	})
	if ix < 0 {
		return fmt.Errorf("call-template: missing name attribute")
	}
	var (
		name = el.Attrs[ix].Value()
		mode = style.Mode
	)
	ix = slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "mode"
	})
	if ix >= 0 {
		mode = el.Attrs[ix].Value()
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

	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "name"
	})
	if ix < 0 {
		return fmt.Errorf("with-param: missing name attribute")
	}
	ident := el.Attrs[ix].Value()

	ix = slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "select"
	})
	if ix < 0 {
		return fmt.Errorf("with-param: missing select attribute")
	}
	return style.DefineParam(ident, el.Attrs[ix].Value())
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
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "test"
	})
	if ix < 0 {
		return fmt.Errorf("if: missing test attribute")
	}
	ok, err := style.TestNode(el.Attrs[ix].Value(), datum)
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
		x := slices.IndexFunc(n.Attrs, func(a xml.Attribute) bool {
			return a.Name == "test"
		})
		if x < 0 {
			return fmt.Errorf("choose: missing test attribute")
		}
		ok, err := style.TestNode(n.Attrs[x].Value(), datum)
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
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "select"
	})
	if ix < 0 {
		return fmt.Errorf("for-each: missing select attribute")
	}
	parent, ok := el.Parent().(*xml.Element)
	if !ok {
		return fmt.Errorf("for-each: xml element expected as parent")
	}
	parent.RemoveNode(el.Position())

	items, err := style.ExecuteQuery(el.Attrs[ix].Value(), datum)
	if err != nil || len(items) == 0 {
		return err
	}

	var it iter.Seq[xml.Item]
	if el.Nodes[0].QualifiedName() == "xsl:sort" {
		x := el.Nodes[0].(*xml.Element)
		ix := slices.IndexFunc(x.Attrs, func(a xml.Attribute) bool {
			return a.Name == "select"
		})
		if ix < 0 {
			return fmt.Errorf("xsl:sort: missing select attribute")
		}
		var (
			query = x.Attrs[ix].Value()
			order string
		)
		ix = slices.IndexFunc(x.Attrs, func(a xml.Attribute) bool {
			return a.Name == "order"
		})
		if ix >= 0 {
			order = x.Attrs[ix].Value()
		}
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
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "select"
	})
	if ix < 0 {
		return fmt.Errorf("value-of: missing select attribute")
	}
	parent, ok := el.Parent().(*xml.Element)
	items, err := style.ExecuteQuery(el.Attrs[ix].Value(), datum)
	if err != nil || len(items) == 0 {
		return parent.RemoveNode(el.Position())
	}
	text := xml.NewText(items[0].Node().Value())
	if !ok {
		return fmt.Errorf("value-of: xml element expected as parent")
	}
	parent.ReplaceNode(el.Position(), text)
	return nil
}

func executeCopy(node, datum xml.Node, style *Stylesheet) error {
	return errImplemented
}

func executeCopyOf(node, datum xml.Node, style *Stylesheet) error {
	return errImplemented
}

func executeSequence(node, datum xml.Node, style *Stylesheet) error {
	return errImplemented
}

func executeMessage(node, datum xml.Node, style *Stylesheet) error {
	return errImplemented
}

func executeElement(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "name"
	})
	if ix < 0 {
		return fmt.Errorf("element: missing name attribute")
	}
	qn, err := xml.ParseName(el.Attrs[ix].Value())
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
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "select"
	})
	if ix < 0 {
		return fmt.Errorf("for-each-group: missing select attribute")
	}
	parent, ok := el.Parent().(*xml.Element)
	if !ok {
		return fmt.Errorf("for-each-group: xml element expected as parent")
	}
	parent.RemoveNode(el.Position())

	items, err := style.ExecuteQuery(el.Attrs[ix].Value(), datum)
	if err != nil || len(items) == 0 {
		return err
	}

	ix = slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "group-by"
	})
	if ix < 0 {
		return fmt.Errorf("for-each-group: missing group-by attribute")
	}
	grpby, err := style.CompileQuery(el.Attrs[ix].Value())
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
