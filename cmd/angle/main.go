package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

var errFail = errors.New("fail")

type Command interface {
	Run([]string) error
}

var allCommands = map[string]Command{
	"format":    &FormatCmd{},
	"fmt":       &FormatCmd{},
	"query":     &QueryCmd{},
	"search":    &QueryCmd{},
	"find":      &QueryCmd{},
	"assert":    &AssertCmd{},
	"transform": &TransformCmd{},
}

func main() {
	flag.Parse()

	cmd, ok := allCommands[flag.Arg(0)]
	if !ok {
		fmt.Fprintf(os.Stderr, "%s: unknown command", flag.Arg(0))
		fmt.Fprintln(os.Stderr)
		os.Exit(2)
	}
	args := flag.Args()
	if err := cmd.Run(args[1:]); err != nil {
		if !errors.Is(err, errFail) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
