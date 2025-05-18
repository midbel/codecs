package main

import (
	"flag"

	"github.com/midbel/codecs/xslt"
)

type TransformCmd struct {
	Context string
}

func (c TransformCmd) Run(args []string) error {
	set := flag.NewFlagSet("transform", flag.ContinueOnError)

	set.StringVar(&c.Context, "d", "", "context directory")

	if err := set.Parse(args); err != nil {
		return err
	}

	_, err := xslt.Load(flag.Arg(0), c.Context)
	if err != nil {
		return err
	}
	return nil
}
