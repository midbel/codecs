package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

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

	p := relax.Parse(r)
	a, err := p.Parse()
	if err != nil {
		fmt.Fprintln(os.Stderr, "parsing error:", err)
		os.Exit(21)
	}
	fmt.Println(a)
	printPattern(a, 0)
	// scan := relax.Scan(r)
	// for {
	// 	tok := scan.Scan()
	// 	fmt.Println(tok)
	// 	if tok.Type == relax.EOF || tok.Type == relax.Invalid {
	// 		break
	// 	}
	// }

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