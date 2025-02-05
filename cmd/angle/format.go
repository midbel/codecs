package main

import (
	"flag"
)

type FormatCmd struct {
	OutFile string
	Query   string
	WriterOptions
}

func (f FormatCmd) Run(args []string) error {
	set := flag.NewFlagSet("format", flag.ContinueOnError)

	set.BoolVar(&f.NoNamespace, "no-namespace", false, "don't write xml namespace into the output document")
	set.BoolVar(&f.NoProlog, "no-prolog", false, "don't write the xml prolog into the output document")
	set.BoolVar(&f.NoComment, "no-comment", false, "dont't write the comment present in the input document")
	set.BoolVar(&f.Compact, "compact", false, "write compact output")
	set.StringVar(&f.CaseType, "case-type", "", "rewrite element/attribute name to given case family")
	set.StringVar(&f.OutFile, "f", "", "specify the path to the file where the document will be written")
	set.StringVar(&f.Query, "q", "", "")

	if err := set.Parse(args); err != nil {
		return err
	}

	doc, err := parseDocument(set.Arg(0), true)
	if err != nil {
		return err
	}
	doc, err = doc.Query(f.Query)
	if err != nil {
		return err
	}
	return writeDocument(doc, f.OutFile, f.WriterOptions)
}
