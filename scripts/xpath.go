package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/midbel/codecs/xpath"
)

func main() {
	trace := flag.Bool("t", false, "trace")
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
	str := xpath.Debug(expr)
	fmt.Println(str)
}
