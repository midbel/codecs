package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/midbel/codecs/sch"
)

type AssertCmd struct {
	reportType string
	options    sch.ReportOptions
}

func (a AssertCmd) Run(args []string) error {
	set := flag.NewFlagSet("assert", flag.ExitOnError)
	if err := set.Parse(args); err != nil {
		return err
	}
	schema, err := parseSchemaFile(flag.Arg(0))
	if err != nil {
		return err
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
	_, _ = re, schema
	return nil
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
