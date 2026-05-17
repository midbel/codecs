package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/codecs/json"
	"github.com/midbel/codecs/jsonata"
)

func main() {
	query := flag.String("q", "", "query")
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer r.Close()

	doc, err := jsonata.Find(r, *query)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ws := json.NewWriter(os.Stdout)
	ws.Write(doc)
}
