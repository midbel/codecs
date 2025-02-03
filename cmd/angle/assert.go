package main

import (
	"flag"
)

type AssertCmd struct{}

func (a AssertCmd) Run(args []string) error {
	set := flag.NewFlagSet("assert", flag.ContinueOnError)
	if err := set.Parse(args); err != nil {
		return err
	}
	return nil
}
