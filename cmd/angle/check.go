package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/midbel/codecs/relax"
	"github.com/midbel/codecs/xml"
)

type CheckCmd struct {
	FailFast bool
}

func (c CheckCmd) Run(args []string) error {
	set := flag.NewFlagSet("format", flag.ContinueOnError)
	set.BoolVar(&c.FailFast, "fail-fast", false, "stop checking files as soon as first error is encountered")
	if err := set.Parse(args); err != nil {
		return err
	}

	schema, err := parseSchema(set.Arg(0))
	if err != nil {
		return err
	}
	args = set.Args()
	for doc, err := range iterDocuments(args[1:]) {
		if err != nil {
			if errors.Is(err, ErrDocument) {
				fmt.Fprintf(os.Stderr, "%s: %w", doc.File, ErrDocument)
				continue
			}
			return err
		}
		if err := schema.Validate(doc.Root()); err != nil {
			if err, ok := err.(relax.NodeError); ok {
				fmt.Fprintln(os.Stderr, xml.WriteNode(err.Node))
				fmt.Fprintln(os.Stderr)
			}
			err = fmt.Errorf("%s: document does not conform to given schema", doc.File)
			if c.FailFast {
				return err
			}
			fmt.Fprintln(os.Stderr, err)
		} else {
			fmt.Fprintf(os.Stdout, "%s: document is valid", doc.File)
			fmt.Fprintln(os.Stdout)
		}
	}
	return nil
}

func parseSchema(file string) (relax.Pattern, error) {
	if file == "" {
		return relax.Valid(), nil
	}
	r, err := openFile(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	p := relax.Parse(r)
	return p.Parse()
}
