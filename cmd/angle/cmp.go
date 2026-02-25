package main

import (
	"fmt"

	"github.com/midbel/cli"
	"github.com/midbel/codecs/xml"
)

var compareCmd = cli.Command{
	Name:    "compare",
	Alias:   []string{"cmp"},
	Summary: "compare two xml documents",
	Handler: &CompareCmd{},
	Usage:   "compare [-o] <file1> <file2>",
}

var diffCmd = cli.Command{
	Name:    "diff",
	Summary: "show difference between two xml documents",
	Handler: &DiffCmd{},
	Usage:   "diff <file1> <file2>",
}

var sortCmd = cli.Command{
	Name:    "sort",
	Summary: "sort nodes in xml documents",
	Handler: &SortCmd{},
}

type CompareCmd struct{}

func (c *CompareCmd) Run(args []string) error {
	var (
		mode    = xml.CmpUnordered
		set     = cli.NewFlagSet("compare")
		ordered = set.Bool("o", false, "ordered comparison")
	)
	if err := set.Parse(args); err != nil {
		return err
	}
	if *ordered {
		mode = xml.CmpOrdered
	}
	_, err := xml.Compare(set.Arg(0), set.Arg(1), mode)
	return err
}

type DiffCmd struct{}

func (c *DiffCmd) Run(args []string) error {
	set := cli.NewFlagSet("diff")
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
