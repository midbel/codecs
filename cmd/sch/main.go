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

func execute(schema *sch.Schema, file, group, level string) error {
	doc, err := parseDocument(file)
	if err != nil {
		return err
	}
	_ = doc
	return nil
}

func keepAssert(group, level string) func(*sch.Assert) bool {
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
