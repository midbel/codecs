package main

import (
	"errors"
	"fmt"
	"strings"

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
		print   = set.Bool("p", false, "print diverging nodes")
	)
	if err := set.Parse(args); err != nil {
		return err
	}
	if *ordered {
		mode = xml.CmpOrdered
	}
	res, err := xml.Compare(set.Arg(0), set.Arg(1), mode)
	if errors.Is(err, xml.ErrCompare) && *print {
		str := xml.WriteNodeDepth(res.Source, 0)
		fmt.Println(">>>", strings.TrimSpace(str))
		
		if res.Target != nil {
			str = xml.WriteNodeDepth(res.Target, 0)
			fmt.Println("<<<", strings.TrimSpace(str))
		}
	}
	return err
}

type SortCmd struct{}

func (c *SortCmd) Run(args []string) error {
	set := cli.NewFlagSet("sort")
	if err := set.Parse(args); err != nil {
		return err
	}
	return fmt.Errorf("not yet implemented")
}
