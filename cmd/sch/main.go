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
	flag.StringVar(&opts.ReportDir, "html-report-dir", "", "html report directory")
	flag.StringVar(&opts.ListenAddr, "listen-addr", "", "html file serve")
	flag.StringVar(&opts.Format, "format", "", "line format")
	flag.Parse()

	schema, err := parseSchema(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if list {
		printList(schema)
		return
	}

	if opts.ListenAddr != "" {
		files := flag.Args()

		serv, _ := Serve(schema, files[1:], opts)
		defer serv.Close()
		if err := serv.ListenAndServe(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}

	var re Reporter
	switch report {
	case "html":
		re, err = HtmlReport(opts)
	case "stdout", "":
		re, err = StdoutReport(opts)
	case "csv":
	case "xml":
	default:
		fmt.Fprintln(os.Stderr, "%s: unsupported report type")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	execDefault(re, schema, flag.Args())
}

func printList(schema *sch.Schema) {
	for a := range schema.Asserts() {
		fmt.Println(a.Ident)
	}
}

func execDefault(re Reporter, schema *sch.Schema, files []string) error {
	if ex, ok := re.(interface {
		Exec(*sch.Schema, []string) error
	}); ok {
		return ex.Exec(schema, files[1:])
	}
	for i := 1; i < len(files); i++ {
		err := re.Run(schema, files[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s", flag.Arg(i), err)
			fmt.Fprintln(os.Stderr)
		}
	}
	return nil
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
