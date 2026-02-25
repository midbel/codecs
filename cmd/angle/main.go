package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/midbel/cli"
)

var errFail = errors.New("fail")

var (
	summary = "angle helps to manipulate xml documents"
	help    = ""
)

func main() {
	var (
		set  = cli.NewFlagSet("angle")
		root = prepare()
	)
	root.SetSummary(summary)
	root.SetHelp(help)
	if err := set.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			root.Help()
			os.Exit(2)
		}
	}
	err := root.Execute(set.Args())
	if err != nil {
		if s, ok := err.(cli.SuggestionError); ok && len(s.Others) > 0 {
			fmt.Fprintln(os.Stderr, "similar command(s)")
			for _, n := range s.Others {
				fmt.Fprintln(os.Stderr, "-", n)
			}
		}
		if !errors.Is(err, errFail) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

func prepare() *cli.CommandTrie {
	root := cli.New()
	root.Register([]string{"format"}, &formatCmd)
	root.Register([]string{"exec"}, &queryCmd)
	root.Register([]string{"query"}, &queryCmd)
	root.Register([]string{"query", "execute"}, &queryCmd)
	root.Register([]string{"query", "debug"}, &debugCmd)
	root.Register([]string{"assert"}, &assertCmd)
	root.Register([]string{"assert", "execute"}, &assertCmd)
	root.Register([]string{"assert", "info"}, &infoSchemaCmd)
	root.Register([]string{"assert", "compile"}, &compileCmd)
	root.Register([]string{"transform"}, &transformCmd)
	root.Register([]string{"compare"}, &compareCmd)
	root.Register([]string{"diff"}, &diffCmd)
	root.Register([]string{"sort"}, &sortCmd)

	return root
}
