package main

import (
	"flag"
)

type QueryCmd struct{}

func (q QueryCmd) Run(args []string) error {
	set := flag.NewFlagSet("query", flag.ContinueOnError)
	if err := set.Parse(args); err != nil {
		return err
	}
	return nil
}
