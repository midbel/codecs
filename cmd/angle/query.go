package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

type QueryCmd struct {
	Root  string
	Noout bool
	ParserOptions
}

const queryInfo = "query took %s - %d nodes matching %q"

func (q QueryCmd) Run(args []string) error {
	var (
		set     = flag.NewFlagSet("query", flag.ContinueOnError)
		options WriterOptions
	)
	set.StringVar(&q.Root, "root", "", "rename root element")
	set.BoolVar(&q.Noout, "noout", false, "suppress output - default is to print the result nodes")
	if err := set.Parse(args); err != nil {
		return err
	}
	doc, err := parseDocument(set.Arg(1), q.ParserOptions)
	if err != nil {
		return err
	}
	now := time.Now()
	doc, err = doc.Query(set.Arg(0))
	if err != nil {
		return err
	}
	elapsed := time.Since(now)
	if !q.Noout {
		doc.SetRootName(q.Root)
		if err := writeDocument(doc, "", options); err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout)
	}
	fmt.Fprintf(os.Stdout, queryInfo, elapsed, doc.GetNodesCount(), set.Arg(0))
	fmt.Fprintln(os.Stdout)
	return nil
}
