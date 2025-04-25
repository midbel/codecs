package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/midbel/codecs/xml"
)

var (
	errImplemented = errors.New("not implemented")
	errUndefined   = errors.New("undefined")
)

type executeFunc func(xml.Node, xml.Node, *Stylesheet) error

var executers map[xml.QName]executeFunc

func init() {
	executers = map[xml.QName]executeFunc{
		xml.QualifiedName("for-each", "xsl"):        executeForeach,
		xml.QualifiedName("value-of", "xsl"):        executeValueOf,
		xml.QualifiedName("call-template", "xsl"):   executeCallTemplate,
		xml.QualifiedName("apply-templates", "xsl"): executeApplyTemplates,
		xml.QualifiedName("if", "xsl"):              executeIf,
		xml.QualifiedName("choose", "xsl"):          executeChoose,
		xml.QualifiedName("variable", "xsl"):        executeVariable,
	}
}

func main() {
	var (
		quiet = flag.Bool("q", false, "quiet")
		mode  = flag.String("m", "", "mode")
	)
	flag.Parse()

	doc, err := loadDocument(flag.Arg(1))
	if err != nil {
		fmt.Fprintln(os.Stderr, "fail to load document:", err)
		os.Exit(2)
	}

	sheet, err := Load(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "fail to load stylesheet", err)
		os.Exit(2)
	}
	sheet.Mode = *mode

	result, err := sheet.Execute(doc)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var w io.Writer = os.Stdout
	if *quiet {
		w = io.Discard
	}
	writer := xml.NewWriter(w)
	if sheet.OutputSettings != nil {
		if !sheet.OutputSettings.Indent {
			writer.WriterOptions |= xml.OptionCompact
		}
		if sheet.OutputSettings.OmitProlog {
			writer.WriterOptions |= xml.OptionNoProlog
		}
		if sheet.OutputSettings.Method == "html" {
			writer.PrologWriter = xml.PrologWriterFunc(writeDoctypeHTML)
		}
	}
	writer.Write(result.(*xml.Document))
}

type OutputSettings struct {
	Name       string
	Method     string
	Encoding   string
	Version    string
	Indent     bool
	OmitProlog bool
}

type Stylesheet struct {
	Mode string

	vars   xml.Environ[string]
	params xml.Environ[string]

	*OutputSettings
	Templates  []*Template
	Parameters map[string]string
}

func Load(file string) (*Stylesheet, error) {
	doc, err := loadDocument(file)
	if err != nil {
		return nil, err
	}
	sheet := Stylesheet{
		vars: xml.Empty[string](),
	}
	sheet.Templates, err = loadTemplates(doc)
	if err != nil {
		return nil, err
	}
	sheet.OutputSettings = &OutputSettings{
		Method:   "xml",
		Version:  "1.0",
		Encoding: "UTF-8",
		Indent:   true,
	}
	out, err := loadOutput(doc)
	if err != nil {
		return nil, err
	}
	if out != nil {
		sheet.OutputSettings = out
	}
	if sheet.params, err = loadParams(doc); err != nil {
		return nil, err
	}

	return &sheet, nil
}

func loadVariables(doc xml.Node) error {
	return nil
}

func loadParams(doc xml.Node) (xml.Environ[string], error) {
	query, err := xml.CompileString("/xsl:stylesheet/xsl:param")
	if err != nil {
		return nil, err
	}
	items, err := query.Find(doc)
	if err != nil {
		return nil, err
	}
	env := xml.Empty[string]()
	for i := range items {
		n := items[i].Node().(*xml.Element)
		if n == nil {
			continue
		}
		ix := slices.IndexFunc(n.Attrs, func(a xml.Attribute) bool {
			return a.Name == "name"
		})
		if ix < 0 {
			return nil, fmt.Errorf("param: missing name attribute")
		}
		ident := n.Attrs[ix].Value()
		ix = slices.IndexFunc(n.Attrs, func(a xml.Attribute) bool {
			return a.Name == "select"
		})
		if ix >= 0 {

		}
		env.Define(ident, "")
	}
	return env, nil
}

func loadOutput(doc xml.Node) (*OutputSettings, error) {
	query, err := xml.CompileString("/xsl:stylesheet/xsl:output")
	if err != nil {
		return nil, err
	}
	items, err := query.Find(doc)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	var (
		node = items[0].Node().(*xml.Element)
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
	return &out, nil
}

func loadTemplates(doc xml.Node) ([]*Template, error) {
	query, err := xml.CompileString("/xsl:stylesheet/xsl:template")
	if err != nil {
		return nil, err
	}
	items, err := query.Find(doc)
	if err != nil {
		return nil, err
	}
	var list []*Template
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
		list = append(list, &t)
	}
	return list, nil
}

func (s *Stylesheet) Find(name string) (*Template, error) {
	ix := slices.IndexFunc(s.Templates, func(t *Template) bool {
		return t.Name == name
	})
	if ix < 0 {
		return nil, fmt.Errorf("template %s not found", name)
	}
	return s.Templates[ix], nil
}

func (s *Stylesheet) Match(node xml.Node, withMode string) (*Template, error) {
	// match work in reverse
	// given a node, we should check that the node would be selected
	// from the xpath expression given in the match attribute of the
	// template
	isMatch := func(pattern string) (bool, int) {
		if pattern == "" {
			return false, -1
		}
		var (
			parts = strings.Split(pattern, "/")
			curr  = node
			rank  int
		)
		slices.Reverse(parts)
		for {
			if len(parts) == 0 {
				break
			}
			if curr.QualifiedName() != parts[0] {
				break
			}
			rank++
			curr = node.Parent()
			if curr == nil {
				break
			}
			parts = parts[1:]

		}
		return true, rank

	}
	if withMode == "" {
		withMode = s.Mode
	}
	for _, t := range s.Templates {
		if t.isRoot() || t.Mode != withMode {
			continue
		}
		if ok, _ := isMatch(t.Match); ok {
			return t.Clone(), nil
		}
	}
	return nil, fmt.Errorf("no template found matching given node (%s)", node.QualifiedName())
}

func (s *Stylesheet) Execute(doc xml.Node) (xml.Node, error) {
	ix := slices.IndexFunc(s.Templates, func(t *Template) bool {
		return t.isRoot() && t.Mode == s.Mode
	})
	if ix < 0 {
		return nil, fmt.Errorf("main template not found")
	}

	if d, ok := doc.(*xml.Document); ok {
		doc = d.Root()
	}

	root, err := s.Templates[ix].Execute(doc, s)
	if err == nil {
		if len(root) != 1 {
			return nil, fmt.Errorf("more than one root element returned")
		}
		return xml.NewDocument(root[0]), nil
	}
	return nil, err
}

type Template struct {
	Name     string
	Match    string
	Mode     string
	Fragment xml.Node

	params xml.Environ[string]
}

func (t *Template) Clone() *Template {
	var tpl Template
	tpl.Fragment = cloneNode(t.Fragment)
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

func executeParameter(node, datum xml.Node, style *Stylesheet) error {
	return errImplemented
}

func executeVariable(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "name"
	})
	if ix < 0 {
		return fmt.Errorf("variable: missing required name attribute")
	}
	return errImplemented
}

func executeApplyTemplates(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "select"
	})
	if ix >= 0 {
		query, err := xml.CompileString(el.Attrs[ix].Value())
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
	var (
		parent = el.Parent().(*xml.Element)
		frag   = tpl.Fragment.(*xml.Element)
	)
	for _, n := range slices.Clone(frag.Nodes) {
		c := cloneNode(n)
		if c == nil {
			continue
		}
		parent.ReplaceNode(el.Position(), c)
		if err := transformNode(c, datum, style); err != nil {
			return err
		}
	}
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
	tpl, err := style.Find(el.Attrs[ix].Value())
	if err != nil {
		return err
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

func executeIf(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "test"
	})
	if ix < 0 {
		return fmt.Errorf("if: missing test attribute")
	}
	ok, err := testNode(el.Attrs[ix].Value(), datum)
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
	parent.Nodes = parent.Nodes[:0]

	expr, err := xml.CompileString(el.Attrs[ix].Value())
	if err != nil {
		return err
	}
	items, err := expr.Find(datum)
	if err != nil {
		return err
	}

	for i := range items {
		value := items[i].Node()
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

func executeValueOf(node, datum xml.Node, style *Stylesheet) error {
	el := node.(*xml.Element)
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == "select"
	})
	if ix < 0 {
		return fmt.Errorf("value-of: missing select attribute")
	}
	expr, err := xml.CompileString(el.Attrs[ix].Value())
	if err != nil {
		return err
	}
	items, err := expr.Find(datum)
	if err != nil || len(items) == 0 {
		return err
	}
	text := xml.NewText(items[0].Node().Value())
	parent, ok := el.Parent().(*xml.Element)
	if !ok {
		return fmt.Errorf("value-of: xml element expected as parent")
	}
	parent.Nodes = parent.Nodes[:0]
	parent.Append(text)
	return nil
}
func testNode(expr string, datum xml.Node) (bool, error) {
	query, err := xml.CompileString(expr)
	if err != nil {
		return false, err
	}
	items, err := query.Find(datum)
	if err != nil {
		return false, err
	}
	return isTrue(items), nil
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
