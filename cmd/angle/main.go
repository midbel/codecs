package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/midbel/codecs/cmd/cli"
)

var errFail = errors.New("fail")

func main() {
	flag.Parse()

	var (
		root = prepare()
		err  = root.Execute(flag.Args())
	)
	if err != nil {
		if s, ok := err.(cli.SuggestionError); ok {
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
	var (
		root          = cli.New()
		fmtCmd        FormatCmd
		queryCmd      QueryCmd
		debugCmd      DebugCmd
		transformCmd  TransformCmd
		assertCmd     SchAssertCmd
		schCompileCmd SchCompileCmd
		schInfoCmd    SchInfoCmd
	)
	root.Register([]string{"format"}, &fmtCmd)
	root.Register([]string{"fmt"}, &fmtCmd)
	root.Register([]string{"exec"}, &queryCmd)
	root.Register([]string{"query"}, &queryCmd)
	root.Register([]string{"query", "execute"}, &queryCmd)
	root.Register([]string{"query", "debug"}, &debugCmd)
	root.Register([]string{"assert"}, &assertCmd)
	root.Register([]string{"assert", "execute"}, &assertCmd)
	root.Register([]string{"assert", "info"}, &schInfoCmd)
	root.Register([]string{"assert", "compile"}, &schCompileCmd)
	root.Register([]string{"transform"}, &transformCmd)

	return root
}
