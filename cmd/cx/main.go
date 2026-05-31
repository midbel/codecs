package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/midbel/codecs/json"
	"github.com/midbel/codecs/probe"
	"github.com/midbel/codecs/xml"
)

func main() {
	var (
		decode func(io.Reader) (any, error) = json.Decode
		encode func(any)                    = writeJSON
		opts   probe.Options
	)
	flag.Func("z", "zip mode", func(str string) error {
		switch str {
		case "", "short", "default":
			opts.Zip = probe.ZipShort
		case "longest":
			opts.Zip = probe.ZipLongest
		case "strict":
			opts.Zip = probe.ZipStrict
		default:
			return fmt.Errorf("unsupported zip mode given: %s", str)
		}
		return nil
	})
	flag.Func("e", "expand mode", func(str string) error {
		switch str {
		case "", "default":
			opts.Expand = probe.ExpandDefault
		case "ignore":
			opts.Expand = probe.ExpandIgnore
		case "strict":
			opts.Expand = probe.ExpandError
		default:
			return fmt.Errorf("unsupported expand mode given: %s", str)
		}
		return nil
	})
	flag.Func("i", "input format", func(str string) error {
		switch str {
		case "json", "":
		case "xml":
			decode = xml.Decode
		default:
			return fmt.Errorf("%s: unsupported input format", str)
		}
		return nil
	})
	flag.Func("o", "output format", func(str string) error {
		switch str {
		case "json", "":
		case "xml":
			encode = writeXML
		default:
			return fmt.Errorf("%s: unsupported output format", str)
		}
		return nil
	})
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "open: %s\n", err)
		os.Exit(2)
	}
	in, err := decode(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode: %s\n", err)
		os.Exit(1)
	}
	res, err := probe.Traverse(flag.Arg(1), in, &opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "traverse: %s\n", err)
		os.Exit(1)
	}
	encode(res)
}

func writeJSON(in any) {
	ws := json.NewWriter(os.Stdout)
	ws.Write(in)
}

func writeXML(in any) {
	ws := xml.NewWriter(os.Stdout)
	_ = ws
}
