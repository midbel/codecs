package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

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

	doc, err := xml.NewParser(r).Parse()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	if path := flag.Arg(1); path != "" {
		expr, err := xml.Compile(strings.NewReader(path))
		if err != nil {
			fmt.Fprintln(os.Stderr, "invalid expression:", err)
			os.Exit(1)
		}
		list, err := expr.Next(doc.Root())
		if err != nil {
			fmt.Println(os.Stderr, err)
			return
		}
		el := xml.NewElement(xml.LocalName("result"))
		el.Nodes = list.Nodes()

		doc = xml.NewDocument(el)
	}
	if err := doc.Write(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(121)
	}
}
