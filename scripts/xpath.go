package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/midbel/codecs/xpath"
)

func main() {
	var (
		trace  = flag.Bool("t", false, "trace")
		rooted = flag.Bool("r", false, "from root")
		scan   = flag.Bool("s", false, "scan")
	)
	flag.Parse()
	if *scan {
		scanQuery(strings.NewReader(flag.Arg(0)))
	} else {
		compileQuery(strings.NewReader(flag.Arg(0)), *rooted, *trace)
	}
}

func scanQuery(str io.Reader) {
	scanner := xpath.Scan(str)
	for {
		tok := scanner.Scan()
		fmt.Println(tok)
		if tok.Type == xpath.EOF {
			break
		}
	}
}

func compileQuery(str io.Reader, rooted, trace bool) {
	cp := xpath.NewCompiler(str)
	if trace {
		cp.Tracer = xpath.TraceStdout()
	}
	expr, err := cp.Compile()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if rooted {
		expr = xpath.FromRoot(expr)
	}
	res := xpath.Debug(expr)
	fmt.Println(res)
}
