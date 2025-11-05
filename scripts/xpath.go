package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/midbel/codecs/xpath"
)

func main() {
	var (
		trace  = flag.Bool("t", false, "trace")
		rooted = flag.Bool("r", false, "from root")
	)
	flag.Parse()
	scanner := xpath.Scan(strings.NewReader(flag.Arg(0)))
	for {
		tok := scanner.Scan()
		fmt.Println(tok)
		if tok.Type == xpath.EOF {
			break
		}
	}
	cp := xpath.NewCompiler(strings.NewReader(flag.Arg(0)))
	if *trace {
		cp.Tracer = xpath.TraceStdout()
	}
	expr, err := cp.Compile()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *rooted {
		expr = xpath.FromRoot(expr)
	}
	str := xpath.Debug(expr)
	fmt.Println(str)
}
