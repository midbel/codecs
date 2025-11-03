package main

import (
	"flag"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/midbel/codecs/sch"
)

type AssertCmd struct {
	reportType string
	ParserOptions
}

func (a *AssertCmd) Run(args []string) error {
	set := flag.NewFlagSet("assert", flag.ExitOnError)
	if err := set.Parse(args); err != nil {
		return err
	}
	schema, err := parseSchemaFile(set.Arg(0))
	if err != nil {
		return err
	}
	for i := 1; i < set.NArg(); i++ {
		doc, err := parseDocument(set.Arg(i), a.ParserOptions)
		if err != nil {
			return err
		}
		results, err := schema.Run(doc)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		printResults(results)
	}
	return nil
}

func printResults(results []sch.Result) {
	for _, r := range results {
		fmt.Printf("%+v\n", r)
	}
}

func getFiles(files []string) iter.Seq[string] {
	fn := func(yield func(string) bool) {
		for _, f := range files {
			i, err := os.Stat(f)
			if err != nil {
				continue
			}
			if i.Mode().IsRegular() {
				if !yield(f) {
					return
				}
				continue
			}
			es, err := os.ReadDir(f)
			if err != nil {
				continue
			}
			for _, e := range es {
				if !yield(filepath.Join(f, e.Name())) {
					return
				}
			}
		}
	}
	return fn
}

func parseSchemaFile(file string) (*sch.Schema, error) {
	u, err := url.Parse(file)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return sch.Open(file)
	}
	res, err := http.Get(file)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		var str strings.Builder
		io.Copy(&str, res.Body)
		return nil, fmt.Errorf(str.String())
	}
	return sch.New(res.Body)
}
