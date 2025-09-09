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
	Root       string
	Noout      bool
	Limit      int
	Depth int
	Text bool
	ParserOptions
}

const queryInfo = "query took %s - %d nodes matching %q"

func (q QueryCmd) Run(args []string) error {
	set := flag.NewFlagSet("query", flag.ContinueOnError)
	set.IntVar(&q.Limit, "limit", 0, "limit number of results returned by query")
	set.StringVar(&q.Root, "root", "", "rename root element")
	set.BoolVar(&q.Noout, "quiet", false, "suppress output - default is to print the result nodes")
	set.BoolVar(&q.StrictNS, "strict-ns", false, "strict namespace checking")
	set.BoolVar(&q.OmitProlog, "omit-prolog", false, "omit xml prolog")
	set.IntVar(&q.Depth, "print-depth", 0, "print depth")
	set.BoolVar(&q.Text, "text", false, "print only value of node")
	if err := set.Parse(args); err != nil {
		return err
	}
	doc, err := parseDocument(set.Arg(1), q.ParserOptions)
	if err != nil {
		return err
	}
	now := time.Now()
	query, err := xpath.Build(set.Arg(0))
	if err != nil {
		return err
	}
	results, err := query.Find(doc)
	if err != nil {
		return err
	}
	elapsed := time.Since(now)
	if !q.Noout {
		if q.Depth >= 0 && !q.Text {
			printNodes(results, q.Depth)
		} else if q.Text {
			printValues(results)
		}
	}
	fmt.Fprintf(os.Stdout, queryInfo, elapsed, results.Len(), set.Arg(0))
	fmt.Fprintln(os.Stdout)
	if results.Len() == 0 {
		return errFail
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
	fmt.Fprintln(os.Stdout)
}