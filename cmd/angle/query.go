package main

import (
	"flag"
)

type QueryCmd struct{
	Root string
}

func (q QueryCmd) Run(args []string) error {
	set := flag.NewFlagSet("query", flag.ContinueOnError)
	set.StringVar(&q.Root, "root", "", "rename root element")
	if err := set.Parse(args); err != nil {
		return err
	}
	doc, err := parseDocument(set.Arg(1), true)
	if err != nil {
		return err
	}
	doc, err = doc.Query(set.Arg(0))
	if err != nil {
		return err
	}
	doc.SetRootName(q.Root)
	var options WriterOptions
	return writeDocument(doc, "", options)
}
