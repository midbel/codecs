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
	var (
		compile = flag.Bool("c", false, "compile expression")
		mode    = flag.String("m", "", "compile mode")
	)
	flag.Parse()

	var (
		rs  = strings.NewReader(flag.Arg(0))
		err error
	)
	if *compile {
		err = compileExpr(rs, *mode)
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
		if tok.Type == xml.EOF || tok.Type == xml.Invalid {
			break
		}
	}
	return nil
}

func compileExpr(rs io.Reader, mode string) error {
	var cpMode xml.StepMode
	switch mode {
	case "xsl", "xsl2", "xsl3":
		cpMode = xml.ModeXsl
	case "", "xpath", "xpath3":
		cpMode = xml.ModeDefault
	default:
		return fmt.Errorf("%s: mode not supported", mode)
	}
	expr, err := xml.CompileMode(rs, cpMode)
	if err == nil {
		fmt.Printf("%#v\n", expr)
	}
	return err
}
