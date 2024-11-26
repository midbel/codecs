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

	p := relax.Parse(r)
	el, err := p.Parse()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printR(el)
	// scan := relax.Scan(r)
	// for {
	// 	tok := scan.Scan()
	// 	fmt.Println(tok)
	// 	if tok.Type == relax.EOF || tok.Type == relax.Invalid {
	// 		break
	// 	}
	// }
}

func printAs(attrs []*relax.Attribute) {
	if len(attrs) == 0 {
		return
	}
	for i := range attrs {
		fmt.Printf(">> %+v\n", attrs[i])
	}
}

func printR(el *relax.Element) {
	fmt.Printf("%+v\n", el)
	printAs(el.Attributes)
	for i := range el.Elements {
		printR(el.Elements[i])
	}
}
