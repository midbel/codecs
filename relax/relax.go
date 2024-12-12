package relax

import (
	"fmt"
	"io"
	"strings"
)

func Print(w io.Writer, schema Pattern) {
	printSchema(w, schema, 0)
}

func printSchema(w io.Writer, schema Pattern, depth int) {
	if schema == nil {
		return
	}
	var prefix string
	if depth > 0 {
		prefix = strings.Repeat(" ", depth*2)
	}
	fmt.Fprint(w, prefix)
	switch p := schema.(type) {
	case Grammar:
		fmt.Fprintf(w, "grammar[")
		fmt.Fprintln(w)

		depth++

		pfx := strings.Repeat(" ", depth*2)
		fmt.Fprint(w, pfx)
		fmt.Fprintln(w, "start[")
		printSchema(w, p.Start, depth+1)
		fmt.Fprint(w, pfx)
		fmt.Fprintln(w, "]")

		if len(p.Links) > 0 {
			fmt.Fprint(w, pfx)
			fmt.Fprintln(w, "links[")
			for _, k := range p.Links {
				printSchema(w, k, depth+1)
			}
			fmt.Fprint(w, pfx)
			fmt.Fprintln(w, "]")
		}

		fmt.Fprint(w, prefix)
		fmt.Fprintln(w, "]")
	case Link:
		fmt.Fprintf(w, "link[%s]", p.Ident)
		fmt.Fprintln(w)
	case Element:
		fmt.Fprintf(w, "element(%s)", p.QualifiedName())
		if len(p.Patterns) > 0 {
			fmt.Fprint(w, "[")
		}
		fmt.Fprintln(w)
		for i := range p.Patterns {
			printSchema(w, p.Patterns[i], depth+1)
		}
		if len(p.Patterns) > 0 {
			fmt.Fprint(w, prefix)
			fmt.Fprintln(w, "]")
		}
	case Attribute:
		fmt.Fprintf(w, "attribute(%s)", p.QualifiedName())
		fmt.Fprintln(w)
	case Choice:
		fmt.Fprintf(w, "choice(%d)[", len(p.List))
		fmt.Fprintln(w)
		depth++
		for i := range p.List {
			pfx := strings.Repeat(" ", depth*2)
			fmt.Fprint(w, pfx)
			fmt.Fprintf(w, "choice#%d[", i+1)
			fmt.Fprintln(w)
			printSchema(w, p.List[i], depth+1)
			fmt.Fprint(w, pfx)
			fmt.Fprintln(w, "]")
		}
		fmt.Fprint(w, prefix)
		fmt.Fprintln(w, "]")
	case Group:
		fmt.Fprintf(w, "group(%d)[", len(p.List))
		fmt.Fprintln(w)
		for i := range p.List {
			printSchema(w, p.List[i], depth+1)
		}
		fmt.Fprint(w, prefix)
		fmt.Fprintln(w, "]")
	default:
		fmt.Fprintln(w, "unknown")
	}
}
