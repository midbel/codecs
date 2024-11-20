package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/midbel/codecs/xml"
)

func main() {
	options := struct {
		Query        string
		NoTrimSpace  bool
		NoOmitProlog bool
		Compact      bool
	}{}
	flag.StringVar(&options.Query, "q", "", "search for element in document")
	flag.BoolVar(&options.NoTrimSpace, "t", false, "trim space")
	flag.BoolVar(&options.NoOmitProlog, "p", false, "omit prolog")
	flag.BoolVar(&options.Compact, "c", false, "write compact output")
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer r.Close()

	p := xml.NewParser(r)
	p.TrimSpace = !options.NoTrimSpace
	p.OmitProlog = !options.NoOmitProlog

	doc, err := p.Parse()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if doc, err = search(doc, options.Query); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(122)
	}
	ws := xml.NewWriter(os.Stdout)
	ws.Compact = options.Compact
	if err := ws.Write(doc); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(121)
	}
}

func search(doc *xml.Document, query string) (*xml.Document, error) {
	if query == "" {
		return doc, nil
	}
	expr, err := xml.Compile(strings.NewReader(query))
	if err != nil {
		return nil, err
	}

	list, err := expr.Next(doc.Root())
	if err != nil {
		return nil, err
	}
	el := xml.NewElement(xml.LocalName("result"))
	el.Nodes = list.Nodes()

	return xml.NewDocument(el), nil
}
