package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/codecs/relax"
)

func main() {
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer r.Close()

	schema, err := relax.Parse(r).Parse()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	relax.Print(os.Stdout, schema)
}
