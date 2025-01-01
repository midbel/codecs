package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/midbel/codecs/xml"
)

func main() {
	compile := flag.Bool("c", false, "compile expression")
	flag.Parse()

	var (
		rs  = strings.NewReader(flag.Arg(0))
		err error
	)
	if *compile {
		err = compileExpr(rs)
	} else {
		err = scanExpr(rs)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

}

func scanExpr(rs io.Reader) error {
	scan := xml.ScanQuery(rs)
	for {
		tok := scan.Scan()
		fmt.Println(tok)
		if tok.Type == xml.EOF {
			break
		}
	}
	return nil
}

func compileExpr(rs io.Reader) error {
	expr, err := xml.Compile(rs)
	if err == nil {
		fmt.Printf("%#v\n", expr)
	}
	return err
}
