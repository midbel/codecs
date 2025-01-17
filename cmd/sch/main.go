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
		opts ReportOptions
		list bool
	)
	flag.BoolVar(&list, "p", false, "print assertions defined in schema")
	flag.StringVar(&opts.Level, "l", "", "severity level")
	flag.StringVar(&opts.Group, "g", "", "group")
	flag.BoolVar(&opts.FailFast, "fail-fast", false, "stop processing on first error")
	flag.BoolVar(&opts.IgnoreZero, "ignore-zero", false, "discard line with zero items")
	flag.BoolVar(&opts.Quiet, "q", false, "produce small output")
	flag.StringVar(&opts.RootSpace, "root-namespace", "", "modify namespace of root element")
	flag.BoolVar(&opts.ErrorOnly, "only-error", false, "print only errorneous assertions")
	flag.DurationVar(&opts.Timeout, "timeout", time.Second*30, "timeout before stopping")
	var (
	// report   = flag.String("o", "", "report format (html, csv, xml)")
	)
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
	report := StdoutReport(opts)
	for i := 1; i < flag.NArg(); i++ {
		err := report.Run(schema, flag.Arg(i))
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
