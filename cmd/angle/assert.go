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
	"time"

	"github.com/midbel/codecs/sch"
)

type AssertCmd struct {
	phase string
	quiet bool
	ParserOptions
}

func (a *AssertCmd) Run(args []string) error {
	set := flag.NewFlagSet("assert", flag.ExitOnError)
	set.StringVar(&a.phase, "p", "", "phase")
	set.BoolVar(&a.quiet, "q", false, "quiet")
	if err := set.Parse(args); err != nil {
		return err
	}
	schema, err := parseSchemaFile(set.Arg(0))
	if err != nil {
		return err
	}
	var w io.Writer = os.Stdout
	if a.quiet {
		w = io.Discard
	}
	for i := 1; i < set.NArg(); i++ {
		doc, err := parseDocument(set.Arg(i), a.ParserOptions)
		if err != nil {
			return err
		}
		var (
			now     = time.Now()
			results []sch.Result
		)
		results, err = schema.RunPhase(a.phase, doc)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		var (
			elapsed  = time.Since(now)
			failures = printResults(w, results)
		)
		fmt.Printf("%s: %d failure(s) on %d assertion(s) (elapsed time: %s)", set.Arg(i), failures, len(results), elapsed)
		fmt.Println()
	}
	return nil
}

func printResults(w io.Writer, results []sch.Result) int {
	var failures int
	for _, r := range results {
		if r.Fail > 0 {
			failures++
		}
		fmt.Fprintf(w, "%-16s | %8d | %8d | %8d | %-s", r.Ident, r.Total, r.Pass, r.Fail, r.Message)
		fmt.Fprintln(w)
	}
	return failures
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
