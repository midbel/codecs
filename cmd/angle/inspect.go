package main

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/midbel/cli"
	"github.com/midbel/codecs/inspect"
	"github.com/midbel/codecs/xml"
)

var compareCmd = cli.Command{
	Name:    "compare",
	Alias:   []string{"cmp"},
	Summary: "compare two xml documents",
	Handler: &CompareCmd{},
	Usage:   "compare [-o] <file1> <file2>",
}

var diffCmd = cli.Command{
	Name:    "diff",
	Summary: "show difference between two xml documents",
	Handler: &DiffCmd{},
	Usage:   "diff <file1> <file2>",
}

var sortCmd = cli.Command{
	Name:    "sort",
	Summary: "sort nodes in xml documents",
	Handler: &SortCmd{},
}

var infosCmd = cli.Command{
	Name:    "infos",
	Summary: "Give infos on node in a document",
	Handler: &InfoCmd{},
}

type CompareCmd struct{}

func (c *CompareCmd) Run(args []string) error {
	var (
		mode    = inspect.CmpUnordered
		set     = cli.NewFlagSet("compare")
		ordered = set.Bool("o", false, "ordered comparison")
	)
	if err := set.Parse(args); err != nil {
		return err
	}
	if *ordered {
		mode = inspect.CmpOrdered
	}
	_, err := inspect.Compare(set.Arg(0), set.Arg(1), mode)
	return err
}

type DiffCmd struct{}

func (c *DiffCmd) Run(args []string) error {
	set := cli.NewFlagSet("diff")
	if err := set.Parse(args); err != nil {
		return err
	}
	return fmt.Errorf("not yet implemented")
}

type SortCmd struct{}

func (c *SortCmd) Run(args []string) error {
	set := cli.NewFlagSet("sort")
	if err := set.Parse(args); err != nil {
		return err
	}
	return fmt.Errorf("not yet implemented")
}

type InfoCmd struct{}

func (c *InfoCmd) Run(args []string) error {
	var (
		set       = cli.NewFlagSet("infos")
		qualified = set.Bool("q", false, "show only qualified name")
		verbose   = set.Bool("v", false, "show all informations")
	)
	if err := set.Parse(args); err != nil {
		return err
	}
	stats, err := inspect.Infos(set.Arg(0))
	if err != nil {
		return err
	}

	rd := cli.NewTableRenderer(os.Stdout)
	if *verbose {
		var (
			elements = stats.Elements
			attributes = stats.Attributes
		)
		if *qualified {
			elements = stats.QualifiedEls
			attributes = stats.QualifiedAttrs
		}
		rd.Render(statsTable(stats.Namespaces))
		fmt.Fprintln(os.Stdout)
		rd.Render(countersTable("Elements", elements, 10))
		fmt.Fprintln(os.Stdout)
		rd.Render(countersTable("Attributes", attributes, 5))
		fmt.Fprintln(os.Stdout)
	}
	rd.Render(countersTable("Types", stats.Types, 0))
	fmt.Fprintln(os.Stdout)
	rd.Render(depthTable(stats.Depth, 50))
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "Max depth: %d", stats.MaxDepth)
	return err
}

func depthTable(stats map[int]int, barWidth int) cli.Table {
	var t cli.Table
	t.Headers = []string{
		"Depth",
		"Count",
		"Ratio",
	}
	var total int
	for _, c := range stats {
		total += c
	}
	depths := slices.Collect(maps.Keys(stats))
	slices.Sort(depths)
	for _, d := range depths {
		w := (float64(stats[d]) / float64(total)) * float64(barWidth)
		if stats[d] > 0 && w < 1 {
			w = 1
		}
		r := []string{
			strconv.Itoa(d),
			strconv.Itoa(stats[d]),
			strings.Repeat("+", int(w)),
		}
		t.Rows = append(t.Rows, r)
	}
	return t
}

func countersTable(title string, stats map[string]int, limit int) cli.Table {
	var t cli.Table
	t.Headers = []string{
		title,
		"Count",
	}
	counters := make(map[string]int)
	for n, c := range stats {
		x := strconv.Itoa(c)
		counters[x] = c

		r := []string{
			n,
			x,
		}
		t.Rows = append(t.Rows, r)
	}
	slices.SortFunc(t.Rows, func(r1, r2 []string) int {
		v1, v2 := r1[1], r2[1]
		return counters[v2] - counters[v1]
	})
	if limit > 0 && limit < len(t.Rows) {
		t.Rows = t.Rows[:limit]
	}
	return t
}

func statsTable(stats map[xml.NS]struct{}) cli.Table {
	var t cli.Table
	t.Headers = []string{
		"prefix",
		"url",
	}
	for ns := range stats {
		r := []string{
			ns.Prefix,
			ns.Uri,
		}
		t.Rows = append(t.Rows, r)
	}
	slices.SortFunc(t.Rows, func(r1, r2 []string) int {
		return strings.Compare(r1[0], r2[0])
	})
	return t
}
