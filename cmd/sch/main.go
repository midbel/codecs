package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/midbel/codecs/sch"
	"github.com/midbel/codecs/xml"
)

func main() {
	var (
		level = flag.String("l", "", "severity level")
		group = flag.String("g", "", "group")
		list  = flag.Bool("p", false, "print assertions defined in schema")
		// skipZero = flag.Bool("skip-zero", false, "skip tests with no nodes matching rules context")
		// failFast = flag.Bool("fail-fast", false, "stop processing on first error")
	)
	flag.Parse()
	schema, err := sch.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *list {
		print(schema, *group, *level)
		return
	}
	err = execute(schema, flag.Arg(1), *group, *level)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

const pattern = "%-4d | %8s | %-32s | %3d/%-3d | %s"

func execute(schema *sch.Schema, file, group, level string) error {
	doc, err := parseDocument(file)
	if err != nil {
		return err
	}
	var ix int
	for res := range schema.Exec(doc, keepAssert(group, level)) {
		ix++
		var msg string
		if res.Failed() {
			msg = res.Error.Error()
			msg = shorten(msg, 48)
		} else {
			msg = "ok"
		}
		fmt.Print(getColor(res))
		fmt.Printf(pattern, ix, res.Level, res.Ident, res.Pass, res.Total, msg)
		if res.Failed() {
			fmt.Print("\033[0m")
		}
		fmt.Println()
	}
	return nil
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
	case "warning":
		return "\033[33m"
	case "fatal":
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
