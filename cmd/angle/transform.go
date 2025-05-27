package main

import (
	"flag"
	"io"
	"os"

	"github.com/midbel/codecs/xslt"
)

type TransformCmd struct {
	Context string
	Trace   bool
	Quiet   bool
	File    string
	ParserOptions
}

func (c TransformCmd) Run(args []string) error {
	set := flag.NewFlagSet("transform", flag.ContinueOnError)
	set.BoolVar(&c.Trace, "t", false, "trace")
	set.BoolVar(&c.Quiet, "q", false, "quiet")
	set.StringVar(&c.Context, "d", "", "context directory")
	set.StringVar(&c.File, "f", "", "output file")
	set.BoolVar(&c.StrictNS, "strict-ns", false, "strict namespace checking")
	set.BoolVar(&c.KeepEmpty, "keep-empty", false, "keep empty element")
	set.BoolVar(&c.OmitProlog, "omit-prolog", false, "omit xml prolog")

	if err := set.Parse(args); err != nil {
		return err
	}

	doc, err := parseDocument(set.Arg(1), c.ParserOptions)
	if err != nil {
		return err
	}

	sheet, err := xslt.Load(set.Arg(0), c.Context)
	if err != nil {
		return err
	}
	if c.Trace {
		sheet.Tracer = xslt.Stdout()
	}
	var w io.Writer = os.Stdout
	if c.Quiet {
		w = io.Discard
	} else if c.File != "" {
		f, err := os.Create(c.File)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	return sheet.Generate(w, doc)
}
