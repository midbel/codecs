package sch

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/midbel/codecs/xml"
	"github.com/midbel/codecs/xpath"
)

var ErrAssert = errors.New("assertion error")

type Result struct {
	Pattern string
	Ident   string
	Message string
	Severe  bool
	Pass    int
	Fail    int
	Total   int
}

const (
	LevelFatal = "fatal"
	LevelWarn  = "warning"
)

type Schema struct {
	Title string

	phases   map[string][]string
	patterns []*Pattern
	mode     string

	eval *xpath.Evaluator
}

func Default() *Schema {
	s := Schema{
		phases: make(map[string][]string),
		eval:   xpath.NewEvaluator(),
	}
	return &s
}

func Open(file string) (*Schema, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return New(r)
}

func New(r io.Reader) (*Schema, error) {
	return parseSchema(r)
}

func (s *Schema) Run(node xml.Node) ([]Result, error) {
	return s.runPhases(node, nil)
}

func (s *Schema) RunPhase(phase string, node xml.Node) ([]Result, error) {
	if phase == "" {
		return s.Run(node)
	}
	phases, ok := s.phases[phase]
	if !ok {
		return nil, nil
	}
	return s.runPhases(node, phases)
}

func (s *Schema) runPhases(node xml.Node, phases []string) ([]Result, error) {
	var list []Result
	for _, p := range s.patterns {
		ok := slices.Contains(phases, p.Ident)
		if !ok && len(phases) > 0 {
			continue
		}
		res, err := p.Run(node)
		if err != nil {
			return nil, err
		}
		list = slices.Concat(list, res)
	}
	return list, nil
}

func (s *Schema) xslMode() bool {
	return strings.HasPrefix(s.mode, "xslt")
}

type Pattern struct {
	Ident string
	Title string
	Rules []*Rule
}

func (p *Pattern) Run(node xml.Node) ([]Result, error) {
	var list []Result
	for _, r := range p.Rules {
		res, err := r.Run(node)
		if err != nil {
			return nil, err
		}
		for i := range res {
			res[i].Pattern = p.Ident
		}
		list = slices.Concat(list, res)
	}
	return list, nil
}

type Rule struct {
	Query xpath.Expr
	Tests []*Assert
}

func (r *Rule) Run(node xml.Node) ([]Result, error) {
	seq, err := r.Query.Find(node)
	if err != nil || seq.Empty() {
		return nil, err
	}
	var list []Result
	for _, t := range r.Tests {
		res := Result{
			Ident:   t.Ident,
			Severe:  t.Flag == LevelFatal,
			Total:   seq.Len(),
			Message: t.Message,
		}
		for i := range seq {
			err := t.Run(seq[i].Node())
			if err != nil && !errors.Is(err, ErrAssert) {
				return nil, err
			}
			if err == nil {
				res.Pass++
			} else {
				res.Fail++
			}
		}
		list = append(list, res)
	}
	return list, nil
}

type Assert struct {
	Ident   string
	Flag    string
	Test    xpath.Expr
	Message string
}

func (r *Assert) Run(node xml.Node) error {
	seq, err := r.Test.Find(node)
	if err != nil {
		return err
	}
	if !seq.True() {
		return ErrAssert
	}
	return nil
}

func parseSchema(r io.Reader) (*Schema, error) {
	doc, err := xml.ParseReader(r)
	if err != nil {
		return nil, err
	}
	return createSchemaFromDocument(doc)
}

func createSchemaFromDocument(doc *xml.Document) (*Schema, error) {
	var (
		sch  = Default()
		root = doc.Root()
	)
	if root == nil {
		return nil, fmt.Errorf("empty document")
	}
	el, err := getElementFromNode(root)
	if err != nil {
		return nil, err
	}
	if mode, err := getAttribute(el, "queryBinding"); err == nil {
		sch.mode = mode
	}
	for _, n := range el.Nodes {
		sub, err := getElementFromNode(n)
		if err != nil {
			return nil, err
		}
		switch name := n.LocalName(); name {
		case "title":
			sch.Title = n.Value()
		case "ns":
			err = loadNsFromElement(sch, sub)
		case "phase":
			err = loadPhaseFromElement(sch, sub)
			if err != nil {
				return nil, err
			}
		case "pattern":
			err = loadPatternFromElement(sch, sub)
		default:
			return nil, fmt.Errorf("unexpected element %s", name)
		}
		if err != nil {
			return nil, err
		}
	}
	return sch, nil
}

func loadValueFromElement(sch *Schema, el *xml.Element) (string, xpath.Expr, error) {
	ident, err := getAttribute(el, "name")
	if err != nil {
		return "", nil, err
	}
	query, err := getAttribute(el, "value")
	if err != nil {
		return "", nil, err
	}
	expr, err := sch.eval.Create(query)
	return ident, expr, err
}

func loadPatternFromElement(sch *Schema, el *xml.Element) error {
	ident, err := getAttribute(el, "id")
	if err != nil {
		return err
	}
	pat := Pattern{
		Ident: ident,
	}

	var ix int
	if el.Nodes[ix].LocalName() == "title" {
		ix++
	}
	for ; ix < len(el.Nodes); ix++ {
		n := el.Nodes[ix]
		if n.Type() == xml.TypeComment {
			continue
		}
		if n.LocalName() != "rule" {
			return fmt.Errorf("expected rule element instead of %s", n.LocalName())
		}
		sub, err := getElementFromNode(n)
		if err != nil {
			return err
		}
		rule, err := loadRuleFromElement(sub, sch)
		if err != nil {
			return err
		}
		pat.Rules = append(pat.Rules, rule)
	}
	sch.patterns = append(sch.patterns, &pat)
	return nil
}

func loadRuleFromElement(el *xml.Element, sch *Schema) (*Rule, error) {
	context, err := getAttribute(el, "context")
	if err != nil {
		return nil, err
	}
	query, err := sch.eval.Create(context)
	if err != nil {
		return nil, err
	}
	rule := Rule{
		Query: query,
	}
	if sch.xslMode() {
		rule.Query = xpath.FromRoot(rule.Query)
	}
	for _, n := range el.Nodes {
		if n.Type() == xml.TypeComment {
			continue
		}
		if n.LocalName() != "assert" {
			return nil, fmt.Errorf("expected assert element instead of %s", n.LocalName())
		}
		sub, err := getElementFromNode(n)
		if err != nil {
			return nil, err
		}
		ass, err := loadAssertFromElement(sub, sch)
		if err != nil {
			return nil, err
		}
		rule.Tests = append(rule.Tests, ass)
	}
	return &rule, nil
}

func loadAssertFromElement(el *xml.Element, sch *Schema) (*Assert, error) {
	var (
		ass Assert
		err error
	)
	if ass.Ident, err = getAttribute(el, "id"); err != nil {
		return nil, err
	}
	query, err := getAttribute(el, "test")
	if err != nil {
		return nil, err
	}
	ass.Test, err = sch.eval.Create(query)
	if err != nil {
		return nil, err
	}
	if ass.Flag, err = getAttribute(el, "flag"); err != nil {
		return nil, err
	}
	ass.Message = el.Value()
	return &ass, nil
}

func loadNsFromElement(sch *Schema, el *xml.Element) error {
	prefix, err := getAttribute(el, "prefix")
	if err != nil {
		return err
	}
	uri, err := getAttribute(el, "uri")
	if err != nil {
		return err
	}
	sch.eval.RegisterNS(prefix, uri)
	return nil
}

func loadPhaseFromElement(sch *Schema, el *xml.Element) error {
	ident, err := getAttribute(el, "id")
	if err != nil {
		return err
	}
	for _, n := range el.Nodes {
		if n.LocalName() != "active" {
			return fmt.Errorf("expected active element")
		}
		sub, err := getElementFromNode(n)
		if err != nil {
			return err
		}
		name, err := getAttribute(sub, "pattern")
		if err != nil {
			return err
		}
		sch.phases[ident] = append(sch.phases[ident], name)
	}
	return nil
}

func getAttribute(el *xml.Element, ident string) (string, error) {
	ix := slices.IndexFunc(el.Attrs, func(a xml.Attribute) bool {
		return a.Name == ident
	})
	if ix < 0 {
		return "", fmt.Errorf("%s: attribute not available", ident)
	}
	return el.Attrs[ix].Value(), nil
}

func getElementFromNode(node xml.Node) (*xml.Element, error) {
	el, ok := node.(*xml.Element)
	if !ok {
		return nil, fmt.Errorf("element expected")
	}
	return el, nil
}
