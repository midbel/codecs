package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/midbel/codecs/relax"
	"github.com/midbel/codecs/xml"
)

func main() {
	flag.Parse()

	schema, err := parseSchema(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "parsing schema:", err)
		os.Exit(21)
	}
	doc, err := parseDocument(flag.Arg(1))
	if err != nil {
		fmt.Println(flag.Arg(1))
		fmt.Fprintln(os.Stderr, "parsing document:", err)
		os.Exit(11)
	}
	printPattern(schema, 0)
	fmt.Println(doc.Root())
}

func parseSchema(file string) (relax.Pattern, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	p := relax.Parse(r)
	return p.Parse()
}

func parseDocument(file string) (*xml.Document, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	p := xml.NewParser(r)
	p.TrimSpace = true
	p.OmitProlog = false

	return p.Parse()
}

func printPattern(pattern relax.Pattern, depth int) {
	var prefix string
	if depth > 1 {
		prefix = strings.Repeat(">", depth)
	}
	switch p := pattern.(type) {
	case relax.Grammar:
		printPattern(p.Start, depth)
		for k, p := range p.List {
			fmt.Println(k)
			printPattern(p, depth+1)
		}
	case relax.Element:
		fmt.Println(prefix, "element:", p.Local)
		for i := range p.Patterns {
			printPattern(p.Patterns[i], depth+1)
		}
	case relax.Attribute:
		fmt.Println(prefix, "attribute:", p.Local)
	case relax.Text:
		fmt.Println(prefix, "text")
	case relax.Empty:
		fmt.Println(prefix, "empty")
	case relax.Link:
		fmt.Println(prefix, "link", p.Ident)
	default:
		fmt.Println(prefix, "unknown")
	}
}
