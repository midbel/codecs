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
		os.Exit(1)
	}
	defer r.Close()

	sax := xml.NewReader(r)
	sax.Any(func(r *xml.Reader, elem xml.E) error {
		fmt.Println(r.Path())
		return nil
	})

	if err := sax.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
