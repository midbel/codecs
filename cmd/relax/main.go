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
		os.Exit(2)
	}
	defer r.Close()

	scan := relax.Scan(r)
	for {
		tok := scan.Scan()
		fmt.Println(tok)
		if tok.Type == relax.EOF || tok.Type == relax.Invalid {
			break
		}
	}
}
