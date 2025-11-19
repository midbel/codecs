package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/midbel/codecs/xslt"
)

func main() {
	flag.Parse()
	scan := xslt.Scan(strings.NewReader(flag.Arg(0)))
	for {
		tok := scan.Scan()
		fmt.Println(tok)
		if tok.Done() || tok.Invalid() {
			break
		}
	}
}
