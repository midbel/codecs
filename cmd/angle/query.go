package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/midbel/codecs/xml"
	"github.com/midbel/codecs/xpath"
)

type QueryCmd struct {
	Quiet   bool
	Limit   int
	Depth   int
	Text    bool
	Options []xpath.Option
	ParserOptions

	query string
	files []string
}

const queryInfo = "query took %s - %d nodes matching %q"

func (q *QueryCmd) Run(args []string) error {
	if err := q.parseArgs(args); err != nil {
		return err
	}
	now := time.Now()
	results, err := q.run()
	if err != nil {
		return err
	}
	elapsed := time.Since(now)
	if !q.Quiet {
		if q.Depth >= 0 && !q.Text {
			printNodes(results, q.Depth)
		} else if q.Text {
			printValues(results)
		}
	}
	fmt.Fprintf(os.Stdout, queryInfo, elapsed, results.Len(), q.query)
	fmt.Fprintln(os.Stdout)
	if results.Len() == 0 {
		return errFail
	}
	return nil
}

func (q *QueryCmd) run() (xpath.Sequence, error) {
	query, err := xpath.BuildWith(q.query, q.Options...)
	if err != nil {
		return nil, err
	}
	var res xpath.Sequence
	for _, f := range q.files {
		doc, err := parseDocument(f, q.ParserOptions)
		if err != nil {
			return nil, err
		}
		results, err := query.Find(doc)
		if err != nil {
			return nil, err
		}
		res.Concat(results)
		if res.Len() > q.Limit {
			break
		}
	}
	return res, nil
}

func (q *QueryCmd) parseArgs(args []string) error {
	set := flag.NewFlagSet("query", flag.ContinueOnError)
	set.BoolVar(&q.Quiet, "quiet", false, "suppress output")
	set.BoolVar(&q.StrictNS, "strict-ns", false, "strict namespace checking")
	set.BoolVar(&q.OmitProlog, "omit-prolog", false, "omit xml prolog")
	set.IntVar(&q.Limit, "limit", 0, "limit number of results returned by query")
	set.IntVar(&q.Depth, "level", 0, "print n level of matching node")
	set.BoolVar(&q.Text, "text", false, "print only the value of matching node")
	set.Func("config", "context configuration", q.configure)
	err := set.Parse(args)
	if err == nil {
		q.query = set.Arg(0)
		q.files = set.Args()[1:]
	}
	return err
}

func (q *QueryCmd) configure(file string) error {
	doc, err := xml.ParseFile(file)
	if err != nil {
		return err
	}
	config := []func(*xml.Document) error{
		q.configureNS,
		q.configureEnforceNS,
		q.configureElementNS,
		q.configureTypeNS,
		q.configureFuncNS,
		q.configureVars,
	}
	for _, fn := range config {
		if err := fn(doc); err != nil {
			return err
		}
	}
	return nil
}

const (
	queryEnforceNS  = "/angle/namespaces/@enforce"
	queryNamespace  = "/angle/namespaces/namespace[@prefix]"
	queryElementNS  = "/angle/namespaces/namespace[@target=\"element\"]"
	queryTypeNS     = "/angle/namespaces/namespace[@target=\"type\"]"
	queryFuncNS     = "/angle/namespaces/namespace[@target=\"function\"]"
	queryVariable   = "/angle/variables/variable[@name]"
	prefixAttrName  = "prefix"
	nameAttrName    = "name"
	enforceAttrName = "enforce"
)

func (q *QueryCmd) configureNS(doc *xml.Document) error {
	ns, err := xpath.Find(doc, queryNamespace)
	if err != nil {
		return err
	}
	for i := range ns {
		el, ok := ns[i].Node().(*xml.Element)
		if !ok {
			continue
		}
		var (
			a = el.GetAttribute(prefixAttrName)
			o = xpath.WithNamespace(a.Value(), el.Value())
		)
		q.Options = append(q.Options, o)
	}
	return nil
}

func (q *QueryCmd) configureEnforceNS(doc *xml.Document) error {
	ns, err := xpath.Find(doc, queryEnforceNS)
	if err != nil {
		return err
	}
	if !ns.Singleton() {
		return fmt.Errorf("only one namespaces with \"enforce\" attribute expected")
	}
	attr, ok := ns[0].Node().(*xml.Attribute)
	if ok && attr.LocalName() == enforceAttrName && attr.Value() == "true" {
		q.Options = append(q.Options, xpath.WithEnforceNS())
	}
	return nil
}

func (q *QueryCmd) configureElementNS(doc *xml.Document) error {
	ns, err := xpath.Find(doc, queryElementNS)
	if err != nil {
		return err
	}
	if ns.Empty() {
		return nil
	}
	if !ns.Singleton() {
		return fmt.Errorf("only one namespace with target \"element\" expected")
	}
	el, ok := ns[0].Node().(*xml.Element)
	if !ok {
		return nil
	}
	q.Options = append(q.Options, xpath.WithElementNS(el.Value()))
	return nil
}

func (q *QueryCmd) configureTypeNS(doc *xml.Document) error {
	ns, err := xpath.Find(doc, queryElementNS)
	if err != nil {
		return err
	}
	if ns.Empty() {
		return nil
	}
	if !ns.Singleton() {
		return fmt.Errorf("only one namespace with target \"type\" expected")
	}
	el, ok := ns[0].Node().(*xml.Element)
	if !ok {
		return nil
	}
	q.Options = append(q.Options, xpath.WithTypeNS(el.Value()))
	return nil
}

func (q *QueryCmd) configureFuncNS(doc *xml.Document) error {
	ns, err := xpath.Find(doc, queryFuncNS)
	if err != nil {
		return err
	}
	if ns.Empty() {
		return nil
	}
	if !ns.Singleton() {
		return fmt.Errorf("only one namespace with target \"function\" expected")
	}
	el, ok := ns[0].Node().(*xml.Element)
	if !ok {
		return nil
	}
	q.Options = append(q.Options, xpath.WithTypeNS(el.Value()))
	return nil
}

func (q *QueryCmd) configureVars(doc *xml.Document) error {
	vs, err := xpath.Find(doc, queryVariable)
	if err != nil {
		return err
	}
	for i := range vs {
		el, ok := vs[i].Node().(*xml.Element)
		if !ok {
			continue
		}
		var (
			a = el.GetAttribute(nameAttrName)
			o = xpath.WithVariable(a.Value(), el.Value())
		)
		q.Options = append(q.Options, o)
	}
	return nil
}

func printValues(results xpath.Sequence) {
	for i := range results {
		n := results[i].Node()
		fmt.Fprintln(os.Stdout, n.Value())
	}
}

func printNodes(results xpath.Sequence, depth int) {
	for i := range results {
		n := results[i].Node()
		fmt.Fprint(os.Stdout, xml.WriteNodeDepth(n, depth+1))
	}
	if len(results) > 0 {
		fmt.Fprintln(os.Stdout)
	}
}
