package main

import (
	"flag"
	"fmt"
	"os"
	"net/http"
	"net/url"
	"io"
	"strings"

	"github.com/midbel/codecs/sch"
	"github.com/midbel/codecs/xml"
)

func main() {
	var (
		level = flag.String("l", "", "severity level")
		group = flag.String("g", "", "group")
		list  = flag.Bool("p", false, "print assertions defined in schema")
		failFast = flag.Bool("fail-fast", false, "stop processing on first error")
		quiet = flag.Bool("q", false, "produce small output")
	)
	flag.Parse()
	schema, err := parseSchema(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *list {
		print(schema, *group, *level)
		return
	}
	err = execute(schema, flag.Arg(1), *group, *level, *quiet, *failFast)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
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

const pattern = "%-4d | %8s | %-32s | %3d/%-3d | %s"

func execute(schema *sch.Schema, file, group, level string, quiet, failFast bool) error {
	doc, err := parseDocument(file)
	if err != nil {
		return err
	}
	var (
		count int
		pass int
	)
	for res := range schema.Exec(doc, keepAssert(group, level)) {
		count++
		if !res.Failed() {
			pass++
		}
		if !quiet {
			printResult(res, count)
		}
		if failFast && res.Failed() {
			break
		}
	}
	fmt.Printf("%d assertions", count)
	fmt.Println()
	fmt.Printf("%d pass", pass)
	fmt.Println()
	fmt.Printf("%d failed", count-pass)
	fmt.Println()
	if count != pass {
		return fmt.Errorf("document is not valid")
	}
	return nil
}

func printResult(res sch.Result, ix int) {
	var msg string
	if res.Failed() {
		msg = res.Error.Error()
		msg = shorten(msg, 48)
	} else {
		msg = "ok"
	}
	fmt.Print(getColor(res))
	fmt.Printf(pattern, ix, res.Level, res.Ident, res.Pass, res.Total, msg)
	fmt.Println("\033[0m")
}

func print(schema *sch.Schema, group, level string) {
	var (
		keep  = keepAssert(group, level)
		count int
	)
	for a := range schema.Asserts() {
		if !keep(a) {
			continue
		}
		var total int
		fmt.Printf("%7s | %-20s | %3d | %-s", a.Flag, a.Ident, total, a.Message)
		fmt.Println()
		count++
	}
	fmt.Printf("%d assertions defined", count)
	fmt.Println()
}

func getColor(res sch.Result) string {
	if !res.Failed() {
		return ""
	}
	switch res.Level {
	case sch.LevelWarn:
		return "\033[33m"
	case sch.LevelFatal:
		return "\033[31m"
	default:
		return ""
	}
}

func shorten(str string, maxLength int) string {
	z := len(str)
	if z <= maxLength {
		return str
	}
	x := strings.IndexRune(str[maxLength:], ' ')
	if x < 0 {
		return str
	}
	return str[:maxLength+x] + "..."
}

func keepAssert(group, level string) sch.FilterFunc {
	var groups []string
	if len(group) > 0 {
		groups = strings.Split(group, "-")
	}

	keep := func(a *sch.Assert) bool {
		if len(groups) == 0 {
			return true
		}
		parts := strings.Split(a.Ident, "-")
		if len(parts) < len(groups) {
			return false
		}
		for i := range groups {
			if parts[i] != groups[i] {
				return false
			}
		}
		if level != "" && level != a.Flag {
			return false
		}
		return true
	}
	return keep
}

func parseDocument(file string) (*xml.Document, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	doc, err := xml.NewParser(r).Parse()
	return doc, err
}
