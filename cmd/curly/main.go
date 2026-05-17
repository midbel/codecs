package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/codecs/json"
	"github.com/midbel/codecs/jsonata"
)

func main() {
	var (
		query   = flag.String("q", "", "query")
		compact = flag.Bool("c", false, "compact")
	)
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
	ws.Compact = *compact
	ws.Write(doc)
}
