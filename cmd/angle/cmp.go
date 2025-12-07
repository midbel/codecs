package main

import (
	"fmt"

	"github.com/midbel/cli"
)

var compareCmd = cli.Command{
	Name:    "compare",
	Alias:   []string{"cmp"},
	Summary: "compare two xml documents",
	Handler: &CompareCmd{},
}

var sortCmd = cli.Command{
	Name:    "sort",
	Summary: "sort nodes in xml documents",
	Handler: &SortCmd{},
}

type CompareCmd struct{}

func (c *CompareCmd) Run(args []string) error {
	set := cli.NewFlagSet("compare")
	if err := set.Parse(args); err != nil {
		return err
	}
	return fmt.Errorf("not yet implemented")
}

type SortCmd struct{}

func (c *SortCmd) Run(args []string) error {
	set := cli.NewFlagSet("sort")
	if err := set.Parse(args); err != nil {
		return err
	}
	return fmt.Errorf("not yet implemented")
}
