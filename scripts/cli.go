package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/codecs/cmd/cli"
)

func debug(args []string) error {
	fmt.Println("debug", args)
	return nil
}

func exec(args []string) error {
	fmt.Println("execute", args)
	return nil
}

func assert(args []string) error {
	fmt.Println("assert", args)
	return nil
}

func info(args []string) error {
	fmt.Println("info", args)
	return nil
}

func format(args []string) error {
	fmt.Println("format", args)
	return nil
}

func main() {
	flag.Parse()

	trie := cli.New()
	trie.Register([]string{"execute"}, exec)
	trie.Register([]string{"exec"}, exec)
	trie.Register([]string{"query", "find"}, exec)
	trie.Register([]string{"query", "debug"}, debug)
	trie.Register([]string{"format"}, format)
	trie.Register([]string{"assert"}, assert)
	trie.Register([]string{"assert", "info"}, info)

	if err := trie.Execute(flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if s, ok := err.(cli.SuggestionError); ok {
			fmt.Fprintln(os.Stderr, "similar command(s)")
			for _, n := range s.Others {
				fmt.Fprintln(os.Stderr, "-", n)
			}
		}
		os.Exit(1)
	}
}
