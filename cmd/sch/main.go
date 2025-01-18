package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/midbel/codecs/sch"
)

func main() {
	var (
		opts   ReportOptions
		list   bool
		report string
	)
	flag.BoolVar(&list, "p", false, "print assertions defined in schema")
	flag.StringVar(&report, "r", "", "print result in given format (stdout, html, csv, xml)")
	flag.StringVar(&opts.Level, "l", "", "severity level")
	flag.StringVar(&opts.Group, "g", "", "group")
	flag.BoolVar(&opts.FailFast, "fail-fast", false, "stop processing on first error")
	flag.BoolVar(&opts.IgnoreZero, "ignore-zero", false, "discard line with zero items")
	flag.BoolVar(&opts.Quiet, "q", false, "produce small output")
	flag.StringVar(&opts.RootSpace, "root-namespace", "", "modify namespace of root element")
	flag.BoolVar(&opts.ErrorOnly, "only-error", false, "print only errorneous assertions")
	flag.DurationVar(&opts.Timeout, "timeout", time.Second*30, "timeout before stopping")
	flag.Parse()

	schema, err := parseSchema(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if list {
		print(schema, opts.Keep())
		return
	}
	var re Reporter
	switch report {
	case "html":
		re = HtmlReport(opts)
	case "stdout", "":
		re = StdoutReport(opts)
	case "csv":
	case "xml":
	default:
		fmt.Fprintln(os.Stderr, "%s: unsupported report type")
	}
	for i := 1; i < flag.NArg(); i++ {
		err := re.Run(schema, flag.Arg(i))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s", flag.Arg(i), err)
			fmt.Fprintln(os.Stderr)
		}
	}
}

func parseSchema(file string) (*sch.Schema, error) {
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
