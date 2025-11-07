package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/midbel/codecs/xml"
	"github.com/midbel/codecs/xpath"
)

type DebugCmd struct{}

func (q *DebugCmd) Run(args []string) error {
	var (
		set    = flag.NewFlagSet("debug", flag.ExitOnError)
		rooted = flag.Bool("r", false, "from root")
	)
	if err := set.Parse(args); err != nil {
		return err
	}
	cp := xpath.NewCompiler(strings.NewReader(set.Arg(0)))

	expr, err := cp.Compile()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *rooted {
		expr = xpath.FromRoot(expr)
	}
	str := xpath.Debug(expr)
	fmt.Println(str)
	return nil
}

type QueryCmd struct {
	Quiet bool
	Limit int
	Depth int
	Text  bool
	ParserOptions

	query string
	files []string

	eval *xpath.Evaluator
}

const queryInfo = "query took %s - %d nodes matching %q"

func (q *QueryCmd) Run(args []string) error {
	q.eval = xpath.NewEvaluator()
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
	query, err := q.eval.Create(q.query)
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

func (q *QueryCmd) configure(file string) error {
	doc, err := xml.ParseFile(file)
	if err != nil {
		return err
	}
	config := []func(*xml.Document) error{
		q.configureNS,
		q.configureElemNS,
		q.configureFuncNS,
		q.configureTypeNS,
		q.configureVars,
		q.enableExtensions,
	}
	for _, fn := range config {
		if err := fn(doc); err != nil {
			return err
		}
	}
	return nil
}

func (q *QueryCmd) enableExtensions(doc *xml.Document) error {
	seq, err := q.eval.Find("/angle/extensions/extension", doc)
	if err != nil {
		return err
	}
	if seq.Empty() {
		return nil
	}
	return nil
}

func (q *QueryCmd) configureNS(doc *xml.Document) error {
	seq, err := q.eval.Find("/angle/namespaces/namespace[@prefix]", doc)
	if err != nil {
		return err
	}
	for _, n := range seq {
		el, ok := n.Node().(*xml.Element)
		if !ok {
			continue
		}
		a := el.GetAttribute("prefix")
		q.eval.RegisterNS(a.Value(), el.Value())
	}
	return nil
}

func (q *QueryCmd) configureElemNS(doc *xml.Document) error {
	seq, err := q.eval.Find("/angle/namespaces/namespace[@target='element']", doc)
	if err != nil {
		return err
	}
	if seq.Empty() {
		return nil
	}
	if !seq.Singleton() {
		return fmt.Errorf("only one namespace with target element expected")
	}
	el := seq.First()
	q.eval.SetElemNS(el.Node().Value())
	return nil
}

func (q *QueryCmd) configureFuncNS(doc *xml.Document) error {
	seq, err := q.eval.Find("/angle/namespaces/namespace[@target='function']", doc)
	if err != nil {
		return err
	}
	if seq.Empty() {
		return nil
	}
	if !seq.Singleton() {
		return fmt.Errorf("only one namespace with target function expected")
	}
	el := seq.First()
	q.eval.SetFuncNS(el.Node().Value())
	return nil
}

func (q *QueryCmd) configureTypeNS(doc *xml.Document) error {
	seq, err := q.eval.Find("/angle/namespaces/namespace[@target='type']", doc)
	if err != nil {
		return err
	}
	if seq.Empty() {
		return nil
	}
	if !seq.Singleton() {
		return fmt.Errorf("only one namespace with target type expected")
	}
	el := seq.First()
	q.eval.SetTypeNS(el.Node().Value())
	return nil
}

func (q *QueryCmd) configureVars(doc *xml.Document) error {
	seq, err := q.eval.Find("/angle/variables/variable", doc)
	if err != nil {
		return err
	}
	for _, n := range seq {
		el, ok := n.Node().(*xml.Element)
		if !ok {
			continue
		}
		a := el.GetAttribute("name")
		q.eval.Define(a.Value(), el.Value())
	}
	return nil
}

func (q *QueryCmd) parseArgs(args []string) error {
	set := flag.NewFlagSet("query", flag.ContinueOnError)
	set.BoolVar(&q.Quiet, "quiet", false, "suppress output")
	set.BoolVar(&q.StrictNS, "strict-namespace", false, "strict namespace checking")
	set.BoolVar(&q.OmitProlog, "omit-prolog", false, "omit xml prolog")
	set.IntVar(&q.Limit, "limit", 0, "limit number of results returned by query")
	set.IntVar(&q.Depth, "level", 0, "print n level of matching node")
	set.IntVar(&q.Depth, "depth", 0, "print n level of matching node")
	set.BoolVar(&q.Text, "text", false, "print only the value of matching node")
	// set.BoolVar(&q.CopyNS, "copy-namespace", false, "copy namespaces from document to xpath engine")
	set.Func("var", "declare variable", func(str string) error {
		return nil
	})
	set.Func("xml-namespace", "declare namespace", func(str string) error {
		prefix, uri, ok := strings.Cut(str, ":")
		if !ok {
			return fmt.Errorf("not a valid namespace")
		}
		q.eval.RegisterNS(prefix, uri)
		return nil
	})
	set.Func("config", "configuration file", q.configure)
	err := set.Parse(args)
	if err == nil {
		q.query = set.Arg(0)
		q.files = set.Args()[1:]
	}
	return err
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
