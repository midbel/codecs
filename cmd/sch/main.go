package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/codecs/sch"
	"github.com/midbel/codecs/xml"
)

func main() {
	flag.Parse()
	schema, err := sch.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	doc, err := parseDocument(flag.Arg(1))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
}

func parseDocument(file string) (*xml.Document, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	doc, err := xml.NewParser(r).Parse()
	return doc, err
}
