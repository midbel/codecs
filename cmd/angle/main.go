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

func main() {
	flag.Parse()

	var command Command
	switch cmd := flag.Arg(0); cmd {
	case "format", "fmt":
		var f FormatCmd
		command = f
	case "query", "search":
		var q QueryCmd
		command = q
	case "validate", "valid", "check":
		var c CheckCmd
		command = c
	case "assert":
		var a AssertCmd
		command = a
	case "transform":
		var a TransformCmd
		command = a
	default:
		fmt.Fprintf(os.Stderr, "%s is not a known command", cmd)
		fmt.Fprintln(os.Stderr)
		os.Exit(2)
	}
	args := flag.Args()
	if err := command.Run(args[1:]); err != nil {
		if !errors.Is(err, errFail) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
