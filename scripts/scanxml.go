package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/codecs/xml"
)

func main() {
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer r.Close()

	scan := xml.Scan(r)
	for {
		tok := scan.Scan()
		fmt.Println(tok)
		if tok.Type == xml.EOF || tok.Type == xml.Invalid {
			break
		}
	}
}
