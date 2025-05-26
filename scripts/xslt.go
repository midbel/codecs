package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"maps"
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
	errEmpty       = errors.New("empty sequence")
	ErrTerminate   = errors.New("terminate")
)

type ExecuteFunc func(*Context) (xml.Sequence, error)

var executers map[xml.QName]ExecuteFunc

func init() {
	nest := func(exec ExecuteFunc) ExecuteFunc {
		fn := func(ctx *Context) (xml.Sequence, error) {
			ctx.Enter(ctx)
			defer ctx.Leave(ctx)
			seq, err := exec(ctx.Nest())
			if err != nil {
				ctx.Error(ctx, err)
			}
			return seq, err
		}
		return fn
	}
	trace := func(exec ExecuteFunc) ExecuteFunc {
		fn := func(ctx *Context) (xml.Sequence, error) {
			ctx.Enter(ctx)
			defer ctx.Leave(ctx)

			seq, err := exec(ctx)
			if err != nil {
				ctx.Error(ctx, err)
			}
			return seq, err
		}
		return fn
	}
	executers = map[xml.QName]ExecuteFunc{
		xml.QualifiedName("for-each", xsltNamespacePrefix):        nest(executeForeach),
		xml.QualifiedName("value-of", xsltNamespacePrefix):        trace(executeValueOf),
		xml.QualifiedName("call-template", xsltNamespacePrefix):   nest(executeCallTemplate),
		xml.QualifiedName("apply-templates", xsltNamespacePrefix): nest(executeApplyTemplates),
		xml.QualifiedName("apply-imports", xsltNamespacePrefix):   nest(executeApplyImport),
		xml.QualifiedName("if", xsltNamespacePrefix):              nest(executeIf),
		xml.QualifiedName("choose", xsltNamespacePrefix):          nest(executeChoose),
		xml.QualifiedName("where-populated", xsltNamespacePrefix): trace(executeWherePopulated),
		xml.QualifiedName("on-empty", xsltNamespacePrefix):        trace(executeOnEmpty),
		xml.QualifiedName("on-not-empty", xsltNamespacePrefix):    trace(executeOnNotEmpty),
		xml.QualifiedName("try", xsltNamespacePrefix):             nest(executeTry),
		xml.QualifiedName("variable", xsltNamespacePrefix):        trace(executeVariable),
		xml.QualifiedName("result-document", xsltNamespacePrefix): trace(executeResultDocument),
		xml.QualifiedName("source-document", xsltNamespacePrefix): nest(executeSourceDocument),
		xml.QualifiedName("import", xsltNamespacePrefix):          trace(executeImport),
		xml.QualifiedName("include", xsltNamespacePrefix):         trace(executeInclude),
		xml.QualifiedName("with-param", xsltNamespacePrefix):      trace(executeWithParam),
		xml.QualifiedName("copy", xsltNamespacePrefix):            trace(executeCopy),
		xml.QualifiedName("copy-of", xsltNamespacePrefix):         trace(executeCopyOf),
		xml.QualifiedName("sequence", xsltNamespacePrefix):        trace(executeSequence),
		xml.QualifiedName("element", xsltNamespacePrefix):         trace(executeElement),
		xml.QualifiedName("attribute", xsltNamespacePrefix):       trace(executeAttribute),
		xml.QualifiedName("text", xsltNamespacePrefix):            trace(executeText),
		xml.QualifiedName("comment", xsltNamespacePrefix):         trace(executeComment),
		xml.QualifiedName("message", xsltNamespacePrefix):         trace(executeMessage),
		xml.QualifiedName("fallback", xsltNamespacePrefix):        trace(executeFallback),
		xml.QualifiedName("merge", xsltNamespacePrefix):           trace(executeMerge),
		xml.QualifiedName("for-each-group", xsltNamespacePrefix):  trace(executeForeachGroup),
	}
}

func main() {
	var (
		quiet  = flag.Bool("q", false, "quiet")
		mode   = flag.String("m", "", "mode")
		file   = flag.String("f", "", "file")
		dir    = flag.String("d", ".", "context directory")
		trace  = flag.Bool("t", false, "trace")
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
	if *trace {
		sheet.Tracer = Stdout()
	}

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

type Tracer interface {
	Enter(*Context)
	Leave(*Context)
	Error(*Context, error)
}

func NoopTracer() Tracer {
	return discardTracer{}
}

type discardTracer struct{}

func (_ discardTracer) Enter(_ *Context) {}

func (_ discardTracer) Leave(_ *Context) {}

func (_ discardTracer) Error(_ *Context, _ error) {}

type stdioTracer struct {
	logger *slog.Logger
}

func Stdout() Tracer {
	return stdioTracer{
		logger: stdioLogger(os.Stdout),
	}
}

func Stderr() Tracer {
	return stdioTracer{
		logger: stdioLogger(os.Stderr),
	}
}

func stdioLogger(w io.Writer) *slog.Logger {
	opts := slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	return slog.New(slog.NewTextHandler(w, &opts))
}

func (t stdioTracer) Println(msg string) {
	t.logger.Info(msg)
}

func (t stdioTracer) Enter(ctx *Context) {
	args := []any{
		"instruction",
		ctx.XslNode.QualifiedName(),
		"node",
		ctx.ContextNode.QualifiedName(),
		"depth",
		ctx.Depth,
	}
	t.logger.Debug("start instruction", args...)
}

func (t stdioTracer) Leave(ctx *Context) {
	args := []any{
		"instruction",
		ctx.XslNode.QualifiedName(),
		"node",
		ctx.ContextNode.QualifiedName(),
		"depth",
		ctx.Depth,
	}
	t.logger.Debug("done instruction", args...)
}

func (t stdioTracer) Error(ctx *Context, err error) {
	t.logger.Error("error while processing instruction", "node", ctx.ContextNode.QualifiedName(), "depth", ctx.Depth, "err", err.Error())
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
	Depth     int
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
		Depth:     e.Depth + 1,
	}
}

func (e *Env) ExecuteQuery(query string, datum xml.Node) (xml.Sequence, error) {
	return e.ExecuteQueryWithNS(query, "", datum)
}

func (e *Env) ExecuteQueryWithNS(query, namespace string, datum xml.Node) (xml.Sequence, error) {
	if query == "" {
		i := xml.NewNodeItem(datum)
		return xml.Singleton(i), nil
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

func (e *Env) Define(ident string, expr xml.Expr) {
	e.Vars.Define(ident, expr)
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
	XslNode     xml.Node
	ContextNode xml.Node

	Index int
	Size  int
	Mode  string

	Depth int

	*Stylesheet
	*Env
}

func (c *Context) errorWithContext(err error) error {
	if c.XslNode == nil {
		return err
	}
	return errorWithContext(c.XslNode.QualifiedName(), err)
}

func (c *Context) queryXSL(query string) (xml.Sequence, error) {
	return c.Env.queryXSL(query, c.XslNode)
}

func (c *Context) WithNodes(ctxNode, xslNode xml.Node) *Context {
	return c.clone(xslNode, ctxNode)
}

func (c *Context) WithXsl(xslNode xml.Node) *Context {
	return c.clone(xslNode, c.ContextNode)
}

func (c *Context) WithXpath(ctxNode xml.Node) *Context {
	return c.clone(c.XslNode, ctxNode)
}

func (c *Context) Nest() *Context {
	child := c.clone(c.XslNode, c.ContextNode)
	child.Env = child.Env.Sub()
	return child
}

func (c *Context) Copy() *Context {
	return c.clone(c.XslNode, c.ContextNode)
}

func (c *Context) clone(xslNode, ctxNode xml.Node) *Context {
	child := Context{
		XslNode:     xslNode,
		ContextNode: ctxNode,
		Index:       1,
		Size:        1,
		Stylesheet:  c.Stylesheet,
		Env:         c.Env,
		Depth:       c.Depth + 1,
	}
	return &child
}

func (c *Context) NotFound(err error, mode string) error {
	var tmp xml.Node
	switch mode := c.getMode(mode); mode.NoMatch {
	case MatchDeepCopy:
		tmp = cloneNode(c.ContextNode)
	case MatchShallowCopy:
		qn, err := xml.ParseName(c.ContextNode.QualifiedName())
		if err != nil {
			return err
		}
		tmp = xml.NewElement(qn)
		if el, ok := c.ContextNode.(*xml.Element); ok {
			a := tmp.(*xml.Element)
			for i := range el.Attrs {
				a.SetAttribute(el.Attrs[i])
			}
			tmp = a
		}
	case MatchTextOnlyCopy:
		tmp = xml.NewText(c.ContextNode.Value())
	case MatchFail:
		return err
	default:
		return err
	}
	return replaceNode(c.XslNode, tmp)
}

type Stylesheet struct {
	DefaultMode string

	namespace   string
	Mode        string
	currentMode *Mode
	Modes       []*Mode
	AttrSet     []*AttributeSet

	output    []*OutputSettings
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
		currentMode: &unnamedMode,
		namespace:   xsltNamespacePrefix,
		Env:         Empty(),
		Tracer:      NoopTracer(),
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

func (s *Stylesheet) LoadDocument(file string) (xml.Node, error) {
	file = filepath.Join(s.Context, file)
	return loadDocument(file)
}

func includesSheet(sheet *Stylesheet, doc xml.Node) error {
	items, err := sheet.queryXSL("/stylesheet/include | /transform/include", doc)
	if err != nil {
		return err
	}
	ctx := sheet.createContext(nil)
	for _, i := range items {
		_, err := executeInclude(ctx.WithXsl(i.Node()))
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
		_, err := executeImport(ctx.WithXsl(i.Node()))
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
		ContextNode: node,
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
	if doc, ok := doc.(*xml.Document); ok {
		root := doc.Root()
		if root.LocalName() != "stylesheet" && root.LocalName() != "transform" {
			return s.simplified(doc)
		}
	}
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
	if err == nil {
		tpl.Match = "/"
		s.Templates = append(s.Templates, tpl)
	}
	return err
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
	tpl, err := s.getMainTemplate()
	if err != nil {
		return nil, err
	}
	// if r, ok := doc.(*xml.Document); ok {
	// 	doc = r.Root()
	// }
	root, err := tpl.Execute(s.createContext(doc))
	if err == nil {
		var doc xml.Document
		doc.Nodes = append(doc.Nodes, root...)
		return &doc, nil
	}
	return nil, err
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
		if _, err := transformNode(ctx.WithXsl(c)); err != nil {
			if errors.Is(err, errSkip) {
				continue
			}
			return nil, err
		}
		nodes = append(nodes, c)
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

func processAVT(ctx *Context) error {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return err
	}
	for i, a := range elem.Attrs {
		var (
			value = a.Value()
			str   strings.Builder
		)
		for q, ok := range iterAVT(value) {
			if !ok {
				str.WriteString(q)
				continue
			}
			items, err := ctx.ExecuteQuery(q, ctx.ContextNode)
			if err != nil {
				return err
			}
			for i := range items {
				str.WriteString(toString(items[i]))
			}
		}
		elem.Attrs[i].Datum = str.String()
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

func transformNode(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	fn, ok := executers[elem.QName]
	if !ok {
		return processNode(ctx)
	}
	if fn == nil {
		return nil, fmt.Errorf("%s not yet implemented", elem.QualifiedName())
	}
	seq, err := fn(ctx)
	if err != nil {
		return nil, err
	}
	if seq.Len() > 0 {
		parent, err := getElementFromNode(elem.Parent())
		if err != nil {
			return nil, err
		}
		for _, i := range seq {
			parent.Append(i.Node())
		}
	}
	return nil, nil
}

func appendNode(ctx *Context) error {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return ctx.errorWithContext(err)
	}
	parent, err := getElementFromNode(elem.Parent())
	if err != nil {
		return ctx.errorWithContext(err)
	}
	for _, n := range slices.Clone(elem.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		parent.Append(c)
		if _, err := transformNode(ctx.WithXsl(c)); err != nil {
			return err
		}
	}
	return nil
}

func processParam(node xml.Node, env *Env) error {
	elem, err := getElementFromNode(node)
	if err != nil {
		return fmt.Errorf("xml element expected")
	}
	ident, err := getAttribute(elem, "name")
	if err != nil {
		return err
	}
	if query, err := getAttribute(elem, "select"); err == nil {
		err = env.DefineParam(ident, query)
	} else {
		var seq xml.Sequence
		for i := range elem.Nodes {
			seq.Append(xml.NewNodeItem(elem.Nodes[i]))
		}
		env.DefineExprParam(ident, xml.NewValueFromSequence(seq))
	}
	return err
}

func processNode(ctx *Context) (xml.Sequence, error) {
	ctx.Enter(ctx)
	defer ctx.Leave(ctx)

	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, err
	}
	if err := processAVT(ctx); err != nil {
		return nil, err
	}
	if err := ctx.SetAttributes(elem); err != nil {
		return nil, err
	}
	var (
		nodes = slices.Clone(elem.Nodes)
		res   = xml.NewSequence()
	)
	for i := range nodes {
		if nodes[i].Type() != xml.TypeElement {
			res.Append(xml.NewNodeItem(nodes[i]))
			continue
		}
		seq, err := transformNode(ctx.WithXsl(nodes[i]))
		if err != nil {
			return nil, err
		}
		res = slices.Concat(res, seq)
	}
	return res, nil
}

func executeImport(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	file, err := getAttribute(elem, "href")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	return nil, ctx.ImportSheet(file)
}

func executeInclude(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	file, err := getAttribute(elem, "href")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	return nil, ctx.IncludeSheet(file)
}

func executeSourceDocument(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	file, err := getAttribute(elem, "href")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	doc, err := ctx.LoadDocument(file)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	parent, err := getElementFromNode(elem.Parent())
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}

	var nodes []xml.Node
	for _, n := range slices.Clone(elem.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		parent.Append(c)
		if _, err := transformNode(ctx.WithNodes(doc, c)); err != nil {
			return nil, ctx.errorWithContext(err)
		}
		nodes = append(nodes, c)
	}
	return nil, removeSelf(elem)
}

func executeResultDocument(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}

	var doc xml.Document
	for _, n := range slices.Clone(elem.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		if _, err := transformNode(ctx.WithXsl(c)); err != nil {
			return nil, err
		}
		doc.Nodes = append(doc.Nodes, c)
	}

	file, err := getAttribute(elem, "href")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	format, _ := getAttribute(elem, "format")
	if err := writeDocument(file, format, &doc, ctx.Stylesheet); err != nil {
		return nil, ctx.errorWithContext(err)
	}
	if err := removeSelf(ctx.XslNode); err != nil {
		return nil, ctx.errorWithContext(err)
	}
	return nil, errSkip
}

func executeVariable(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	ident, err := getAttribute(elem, "name")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	var seq xml.Sequence
	if query, err1 := getAttribute(elem, "select"); err1 == nil {
		seq, err = ctx.ExecuteQuery(query, ctx.ContextNode)
	} else {
		for _, n := range slices.Clone(elem.Nodes) {
			c := cloneNode(n)
			if c == nil {
				continue
			}
			res, err := transformNode(ctx.WithXsl(c))
			if err != nil {
				return nil, ctx.errorWithContext(err)
			}
			seq = slices.Concat(res, res)
		}
	}
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	ctx.Define(ident, xml.NewValueFromSequence(seq))
	return nil, removeSelf(ctx.XslNode)
}

func executeWithParam(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	ident, err := getAttribute(elem, "name")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	if query, err := getAttribute(elem, "select"); err == nil {
		ctx.EvalParam(ident, query, ctx.ContextNode)
	} else {
		var res xml.Sequence
		for _, n := range slices.Clone(elem.Nodes) {
			c := cloneNode(n)
			if c == nil {
				continue
			}
			seq, err := transformNode(ctx.WithXsl(c))
			if err != nil {
				return nil, ctx.errorWithContext(err)
			}
			res = slices.Concat(res, seq)
		}
		ctx.DefineExprParam(ident, xml.NewValueFromSequence(res))
	}
	return nil, removeSelf(ctx.XslNode)
}

func executeApplyImport(ctx *Context) (xml.Sequence, error) {
	return executeApply(ctx, ctx.MatchImport)
}

func executeApplyTemplates(ctx *Context) (xml.Sequence, error) {
	return executeApply(ctx, ctx.Match)
}

func executeCallTemplate(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	name, err := getAttribute(elem, "name")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	mode, _ := getAttribute(elem, "mode")
	tpl, err := ctx.Find(name, mode)
	if err != nil {
		return nil, ctx.NotFound(err, mode)
	}
	sub := tpl.mergeContext(ctx)
	if err := applyParams(sub); err != nil {
		return nil, ctx.errorWithContext(err)
	}
	parent, err := getElementFromNode(elem.Parent())
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	for _, n := range slices.Clone(tpl.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		parent.Append(c)
		if _, err := transformNode(sub.WithXsl(c)); err != nil {
			return nil, ctx.errorWithContext(err)
		}
	}
	return nil, removeSelf(ctx.XslNode)
}

func executeForeachGroup(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	query, err := getAttribute(elem, "select")
	if err != nil {
		return nil, err
	}
	parent, err := getElementFromNode(elem.Parent())
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}

	items, err := ctx.ExecuteQuery(query, ctx.ContextNode)
	if err != nil || len(items) == 0 {
		return nil, err
	}

	key, err := getAttribute(elem, "group-by")
	if err != nil {
		return nil, err
	}
	grpby, err := ctx.CompileQuery(key)
	if err != nil {
		return nil, err
	}
	groups := make(map[string]xml.Sequence)
	for i := range items {
		is, err := grpby.Find(items[i].Node())
		if err != nil {
			return nil, err
		}
		key := is[0].Value().(string)
		groups[key] = append(groups[key], items[i])
	}

	for key, items := range groups {
		defineForeachGroupBuiltins(ctx, key, items)
		for _, n := range elem.Nodes {
			c := cloneNode(n)
			if c == nil {
				continue
			}
			parent.Append(c)
			if _, err := transformNode(ctx.WithXsl(c)); err != nil {
				return nil, err
			}
		}
	}
	return nil, removeSelf(ctx.XslNode)
}

type MergedItem struct {
	xml.Item
	Key    string
	Source string
}

func executeMerge(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	var (
		action xml.Node
		groups = make(map[string][]MergedItem)
	)

	for _, n := range elem.Nodes {
		if n.QualifiedName() != ctx.getQualifiedName("merge-source") {
			action = n
			break
		}
		el := n.(*xml.Element)
		ident, err := getAttribute(el, "name")
		if err != nil {
			return nil, err
		}
		var items xml.Sequence
		if query, err := getAttribute(el, "select"); err == nil {
			items, err = ctx.ExecuteQuery(query, ctx.ContextNode)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
		if len(el.Nodes) == 0 {
			return nil, fmt.Errorf("missing xsl:merge-key element")
		}
		if query, err := getAttribute(el.Nodes[0].(*xml.Element), "select"); err != nil {
			return nil, err
		} else {
			grp, err := ctx.CompileQuery(query)
			if err != nil {
				return nil, err
			}
			for i := range items {
				is, err := grp.Find(items[i].Node())
				if err != nil {
					return nil, err
				}
				mit := MergedItem{
					Item:   items[i],
					Source: ident,
					Key:    fmt.Sprint(is[0].Value()),
				}
				groups[mit.Key] = append(groups[mit.Key], mit)
			}
		}
	}
	if action.QualifiedName() != ctx.getQualifiedName("merge-action") {
		return nil, fmt.Errorf("merge-action expected")
	}
	elem, ok := action.(*xml.Element)
	if !ok {
		return nil, fmt.Errorf("merge-action: expected xml element")
	}

	keys := slices.Collect(maps.Keys(groups))
	slices.Sort(keys)
	for _, key := range keys {
		nested := ctx.Nest()
		defineMergeBuiltins(nested, key, groups[key])
		if err := appendNode(nested); err != nil {
			return nil, err
		}
	}
	return nil, removeSelf(ctx.XslNode)
}

func executeForeach(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	query, err := getAttribute(elem, "select")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}

	items, err := ctx.ExecuteQuery(query, ctx.ContextNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	if len(items) == 0 {
		return nil, removeSelf(ctx.XslNode)
	}
	it, err := applySort(ctx, items)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}

	parent, err := getElementFromNode(elem.Parent())
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	for i := range it {
		node := i.Node()
		for _, n := range elem.Nodes {
			c := cloneNode(n)
			if c == nil {
				continue
			}
			parent.Append(c)
			sub := ctx.WithNodes(node, c)
			if _, err := transformNode(sub); err != nil {
				return nil, err
			}
		}
	}
	return nil, removeSelf(ctx.XslNode)
}

func executeTry(ctx *Context) (xml.Sequence, error) {
	items, err := ctx.queryXSL("./catch[last()]")
	if err != nil {
		return nil, err
	}
	if len(items) > 1 {
		return nil, fmt.Errorf("only one catch element is allowed")
	}
	if _, err := processNode(ctx); err != nil {
		if len(items) > 0 {
			catch := items[0].Node()
			if err := removeNode(ctx.XslNode, catch); err != nil {
				return nil, err
			}
			return processNode(ctx.WithXsl(catch))
		}
		return nil, err
	}
	return nil, nil
}

func executeIf(ctx *Context) (xml.Sequence, error) {
	el := ctx.XslNode.(*xml.Element)
	test, err := getAttribute(el, "test")
	if err != nil {
		return nil, err
	}
	ok, err := ctx.TestNode(test, ctx.ContextNode)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, removeSelf(el)
	}
	if _, err = processNode(ctx); err != nil {
		return nil, err
	}
	return nil, insertNodes(el, el.Nodes...)
}

func executeChoose(ctx *Context) (xml.Sequence, error) {
	items, err := ctx.queryXSL("/when")
	if err != nil {
		return nil, err
	}
	for i := range items {
		n := items[i].Node().(*xml.Element)
		test, err := getAttribute(n, "test")
		if err != nil {
			return nil, err
		}
		ok, err := ctx.TestNode(test, ctx.ContextNode)
		if err != nil {
			return nil, err
		}
		if ok {
			if _, err := processNode(ctx); err != nil {
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

	if items, err = ctx.queryXSL("otherwise"); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	n := items[0].Node().(*xml.Element)
	if _, err := processNode(ctx); err != nil {
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

func executeValueOf(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	query, err := getAttribute(elem, "select")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	sep, err := getAttribute(elem, "separator")
	if err != nil {
		sep = " "
	}
	items, err := ctx.ExecuteQuery(query, ctx.ContextNode)
	if err != nil || len(items) == 0 {
		return nil, removeSelf(ctx.XslNode)
	}

	var str strings.Builder
	for i := range items {
		if i > 0 {
			str.WriteString(sep)
		}
		str.WriteString(toString(items[i]))
	}
	text := xml.NewText(str.String())
	return nil, replaceNode(ctx.XslNode, text)
}

func executeCopy(ctx *Context) (xml.Sequence, error) {
	return executeCopyOf(ctx)
}

func executeCopyOf(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	query, err := getAttribute(elem, "select")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	items, err := ctx.ExecuteQuery(query, ctx.ContextNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	var list []xml.Node
	for i := range items {
		c := cloneNode(items[i].Node())
		if c != nil {
			list = append(list, c)
		}
	}
	return nil, insertNodes(elem, list...)
}

func executeMessage(ctx *Context) (xml.Sequence, error) {
	var (
		parts []string
		el    = ctx.XslNode.(*xml.Element)
	)
	for _, n := range el.Nodes {
		parts = append(parts, n.Value())
	}
	if t, ok := ctx.Tracer.(interface{ Println(string) }); ok {
		t.Println(strings.Join(parts, ""))
	}

	if quit, err := getAttribute(el, "terminate"); err == nil && quit == "yes" {
		return nil, ErrTerminate
	}
	return nil, nil
}

func executeEvaluate(ctx *Context) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeAnalyzeString(ctx *Context) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeMatchingSubstring(ctx *Context) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeNonMatchingSubstring(ctx *Context) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeWherePopulated(ctx *Context) (xml.Sequence, error) {
	nested := ctx.Copy().Nest()

	elem, err := getElementFromNode(nested.XslNode)
	if err != nil {
		return nil, nested.errorWithContext(err)
	}
	var res xml.Sequence
	for _, n := range elem.Nodes {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		seq, err := transformNode(nested.WithXsl(c))
		if err != nil {
			return nil, nested.errorWithContext(err)
		}
		res = slices.Concat(res, seq)
	}
	return res, removeSelf(elem)
}

func executeOnEmpty(ctx *Context) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeOnNotEmpty(ctx *Context) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeSequence(ctx *Context) (xml.Sequence, error) {
	elem := ctx.XslNode.(*xml.Element)
	query, err := getAttribute(elem, "select")
	if err != nil {
		return nil, err
	}
	return ctx.ExecuteQuery(query, ctx.ContextNode)
}

func executeElement(ctx *Context) (xml.Sequence, error) {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	ident, err := getAttribute(elem, "name")
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	qn, err := xml.ParseName(ident)
	if err != nil {
		return nil, ctx.errorWithContext(err)
	}
	var (
		curr  = xml.NewElement(qn)
		nodes = slices.Clone(elem.Nodes)
	)
	if err := replaceNode(elem, curr); err != nil {
		return nil, err
	}
	for i := range nodes {
		c := cloneNode(nodes[i])
		if c == nil {
			continue
		}
		curr.Append(c)
		if _, err := transformNode(ctx.WithXsl(c)); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func executeAttribute(ctx *Context) (xml.Sequence, error) {
	return nil, errImplemented
}

func executeText(ctx *Context) (xml.Sequence, error) {
	text := xml.NewText(ctx.XslNode.Value())
	return nil, replaceNode(ctx.XslNode, text)
}

func executeComment(ctx *Context) (xml.Sequence, error) {
	comment := xml.NewComment(ctx.XslNode.Value())
	return nil, replaceNode(ctx.XslNode, comment)
}

func executeFallback(ctx *Context) (xml.Sequence, error) {
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

func getNodesForTemplate(ctx *Context) ([]xml.Node, error) {
	var (
		elem = ctx.XslNode.(*xml.Element)
		res  []xml.Node
	)
	if query, err := getAttribute(elem, "select"); err == nil {
		items, err := ctx.ExecuteQuery(query, ctx.ContextNode)
		if err != nil {
			return nil, err
		}
		for i := range items {
			res = append(res, items[i].Node())
		}
	} else {
		res = []xml.Node{cloneNode(ctx.ContextNode)}
	}
	return res, nil
}

func applyParams(ctx *Context) error {
	elem, err := getElementFromNode(ctx.XslNode)
	if err != nil {
		return ctx.errorWithContext(err)
	}
	for _, n := range slices.Clone(elem.Nodes) {
		if n.QualifiedName() != ctx.getQualifiedName("with-param") {
			return fmt.Errorf("%s: invalid child node %s", ctx.XslNode.QualifiedName(), n.QualifiedName())
		}
		if _, err := transformNode(ctx.WithXsl(n)); err != nil {
			return err
		}
	}
	return nil
}

func applySort(ctx *Context, items []xml.Item) (iter.Seq[xml.Item], error) {
	sorts, err := ctx.queryXSL("./sort[1]")
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

func executeApply(ctx *Context, match matchFunc) (xml.Sequence, error) {
	nodes, err := getNodesForTemplate(ctx)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, removeSelf(ctx.XslNode)
	}
	var (
		el      = ctx.XslNode.(*xml.Element)
		mode, _ = getAttribute(el, "mode")
		results []xml.Node
	)
	for _, datum := range nodes {
		tpl, err := match(datum, mode)
		if err != nil {
			for i := range nodes {
				sub := ctx.WithXpath(nodes[i])
				if err = sub.NotFound(err, mode); err != nil {
					return nil, err
				}
			}
			return nil, err
		}
		sub := tpl.mergeContext(ctx.WithXpath(datum))
		if err := applyParams(sub); err != nil {
			return nil, err
		}
		res, err := tpl.Execute(sub)
		if err != nil {
			return nil, err
		}
		results = slices.Concat(results, res)
	}
	return nil, insertNodes(ctx.XslNode, results...)
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

func defineForeachGroupBuiltins(nested *Context, key string, seq xml.Sequence) {
	currentGrp := func(_ xml.Context, _ []xml.Expr) (xml.Sequence, error) {
		return seq, nil
	}
	currentKey := func(_ xml.Context, _ []xml.Expr) (xml.Sequence, error) {
		i := xml.NewLiteralItem(key)
		return []xml.Item{i}, nil
	}

	nested.Builtins.Define("current-group", currentGrp)
	nested.Builtins.Define("fn:current-group", currentGrp)
	nested.Builtins.Define("current-grouping-key", currentKey)
	nested.Builtins.Define("fn:current-grouping-key", currentKey)
}

func defineMergeBuiltins(nested *Context, key string, items []MergedItem) {
	currentKey := func(_ xml.Context, _ []xml.Expr) (xml.Sequence, error) {
		return xml.Singleton(key), nil
	}
	currentGrp := func(ctx xml.Context, args []xml.Expr) (xml.Sequence, error) {
		if len(args) > 1 {
			return nil, fmt.Errorf("too many arguments")
		}
		var (
			seq xml.Sequence
			grp string
		)
		if len(args) == 1 {
			names, err := args[0].Find(ctx)
			if err != nil {
				return nil, err
			}
			if names.Empty() {
				return nil, fmt.Errorf("no group available")
			}
			grp = fmt.Sprint(names[0].Value())
		}
		for i := range items {
			if grp != "" && items[i].Source != grp {
				continue
			}
			seq.Append(items[i].Item)
		}
		return seq, nil
	}
	nested.Builtins.Define("current-merge-group", currentGrp)
	nested.Builtins.Define("fn:current-merge-group", currentGrp)
	nested.Builtins.Define("current-merge-key", currentKey)
	nested.Builtins.Define("fn:current-merge-key", currentKey)
}

func errorWithContext(ctx string, err error) error {
	return fmt.Errorf("%s: %w", ctx, err)
}

func cloneNode(n xml.Node) xml.Node {
	cloner, ok := n.(xml.Cloner)
	if !ok {
		return nil
	}
	return cloner.Clone()
}

func getElementFromNode(node xml.Node) (*xml.Element, error) {
	el, ok := node.(*xml.Element)
	if !ok {
		return nil, fmt.Errorf("%s: xml element expected", node.QualifiedName())
	}
	return el, nil
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
