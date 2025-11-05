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

type QueryCmd struct {
	Quiet     bool
	Limit     int
	Depth     int
	Text      bool
	CopyNS    bool
	XmlSpaces []xml.NS
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
	var (
		eval = xpath.NewEvaluator()
		res  xpath.Sequence
	)
	for _, n := range q.XmlSpaces {
		eval.RegisterNS(n.Prefix, n.Uri)
	}
	query, err := eval.Create(q.query)
	if err != nil {
		return nil, err
	}
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
	set.BoolVar(&q.StrictNS, "strict-namespace", false, "strict namespace checking")
	set.BoolVar(&q.OmitProlog, "omit-prolog", false, "omit xml prolog")
	set.IntVar(&q.Limit, "limit", 0, "limit number of results returned by query")
	set.IntVar(&q.Depth, "level", 0, "print n level of matching node")
	set.IntVar(&q.Depth, "depth", 0, "print n level of matching node")
	set.BoolVar(&q.Text, "text", false, "print only the value of matching node")
	set.BoolVar(&q.CopyNS, "copy-namespace", false, "copy namespaces from document to xpath engine")
	set.Func("var", "declare variable", func(str string) error {
		return nil
	})
	set.Func("xml-namespace", "declare namespace", func(str string) error {
		prefix, uri, ok := strings.Cut(str, ":")
		if !ok {
			return fmt.Errorf("not a valid namespace")
		}
		ns := xml.NS{
			Prefix: prefix,
			Uri:    uri,
		}
		q.XmlSpaces = append(q.XmlSpaces, ns)
		return nil
	})
	// set.Func("config", "context configuration", q.configure)
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
