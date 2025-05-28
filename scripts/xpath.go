package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/midbel/codecs/xpath"
)

func main() {
	flag.Parse()
	scanner := xpath.Scan(strings.NewReader(flag.Arg(0)))
	for {
		tok := scanner.Scan()
		fmt.Println(tok)
		if tok.Type == xpath.EOF {
			break
		}
	}
}
