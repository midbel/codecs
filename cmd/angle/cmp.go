package main

import (
	"flag"
	"fmt"
)

// var compareCmd = Command {
// 	Name: "compare",
// 	Alias: []string{"cmp"},
// 	Summary: "compare two xml documents",
// }

// var sortCmd = Command {
// 	Name: "sort",
// 	Summary: "sort nodes in xml documents",
// }

type CompareCmd struct{}

func (c CompareCmd) Run(args []string) error {
	set := flag.NewFlagSet("compare", flag.ExitOnError)
	if err := set.Parse(args); err != nil {
		return err
	}
	return fmt.Errorf("not yet implemented")
}

type SortCmd struct{}

func (c SortCmd) Run(args []string) error {
	set := flag.NewFlagSet("sort", flag.ExitOnError)
	if err := set.Parse(args); err != nil {
		return err
	}
	return fmt.Errorf("not yet implemented")
}
