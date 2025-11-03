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
	"slices"
	"strings"

	"github.com/midbel/codecs/sch"
)

type AssertCmd struct {
	reportType string
	options    sch.ReportOptions
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
	if set.NArg() <= 1 {
		return fmt.Errorf("no enough files given")
	}

	var re sch.Reporter
	switch a.reportType {
	case "html":
		re, err = sch.HtmlReport(a.options)
	case "stdout", "":
		re, err = sch.StdoutReport(a.options)
	case "csv":
	case "xml":
	default:
		fmt.Fprintln(os.Stderr, "%s: unsupported report type")
	}
	if err != nil {
		return err
	}
	args = set.Args()
	it := getFiles(args[1:])
	return re.Run(schema, slices.Collect(it))
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
	return sch.Parse(res.Body)
}
