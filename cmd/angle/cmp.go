package main

import (
	"fmt"
)

type CompareCmd struct{}

func (c CompareCmd) Run(args []string) error {
	set := flag.NewFlagSet("compare", flag.ExitOnError)
	if err := set.Parse(args); err != nil {
		return err
	}
	return fmt.Errorf("not yet implemented")
}
