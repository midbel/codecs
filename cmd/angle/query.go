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
	PrintDepth int
	ParserOptions
}

const queryInfo = "query took %s - %d nodes matching %q"

func (q QueryCmd) Run(args []string) error {
	set := flag.NewFlagSet("query", flag.ContinueOnError)
	set.StringVar(&q.Root, "root", "", "rename root element")
	set.BoolVar(&q.Noout, "noout", false, "suppress output - default is to print the result nodes")
	set.BoolVar(&q.StrictNS, "strict-ns", false, "strict namespace checking")
	set.BoolVar(&q.OmitProlog, "omit-prolog", false, "omit xml prolog")
	set.IntVar(&q.PrintDepth, "print-depth", 0, "print depth")
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
	if !q.Noout && q.PrintDepth >= 0 {
		for i := range results {
			n := results[i].Node()
			fmt.Fprint(os.Stdout, xml.WriteNodeDepth(n, q.PrintDepth+1))
		}
		fmt.Fprintln(os.Stdout)
	}
	fmt.Fprintf(os.Stdout, queryInfo, elapsed, results.Len(), set.Arg(0))
	fmt.Fprintln(os.Stdout)
	return nil
}
