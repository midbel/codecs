package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/codecs/json"
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

	doc, err := json.Parse(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *query != "" {
		q, err := json.Compile(*query)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		doc, err = q.Get(doc)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}

	ws := json.NewWriter(os.Stdout)
	ws.Write(doc)
}
