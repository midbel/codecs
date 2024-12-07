package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/midbel/codecs/xml"
)

func main() {
	options := struct {
		Root         string
		Query        string
		NoTrimSpace  bool
		NoOmitProlog bool
		Compact      bool
		Schema       string
	}{}
	flag.StringVar(&options.Root, "r", "document", "root element name to use when using a query")
	flag.StringVar(&options.Query, "q", "", "search for element in document")
	flag.StringVar(&options.Schema, "s", "", "relax schema to validate XML document")
	flag.BoolVar(&options.NoTrimSpace, "t", false, "trim space")
	flag.BoolVar(&options.NoOmitProlog, "p", false, "omit prolog")
	flag.BoolVar(&options.Compact, "c", false, "write compact output")
	flag.Parse()

	r, err := open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer r.Close()

	p := xml.NewParser(r)
	p.TrimSpace = !options.NoTrimSpace
	p.OmitProlog = !options.NoOmitProlog

	doc, err := p.Parse()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if doc, err = search(doc, options.Query, options.Root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(122)
	}
	if doc == nil {
		return
	}
	ws := xml.NewWriter(os.Stdout)
	ws.Compact = options.Compact
	if err := ws.Write(doc); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(121)
	}
}

func open(file string) (io.ReadCloser, error) {
	u, err := url.Parse(file)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "", "file":
		return os.Open(file)
	case "http", "https":
		req, err := http.NewRequest(http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("accept", "text/xml")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		if res.StatusCode != 200 {
			return nil, fmt.Errorf("fail to retrieve remote file")
		}
		return res.Body, nil
	default:
		return nil, fmt.Errorf("file can not be retrieve with %s protocol", u.Scheme)
	}
}

func search(doc *xml.Document, query, root string) (*xml.Document, error) {
	if query == "" {
		return doc, nil
	}
	expr, err := xml.Compile(strings.NewReader(query))
	if err != nil {
		return nil, err
	}

	list, err := expr.Next(doc.Root())
	if err != nil {
		return nil, err
	}
	if list.Empty() {
		return nil, nil
	}
	var node xml.Node
	if ns := list.Nodes(); list.Len() == 1 {
		node = ns[0]
	} else {
		el := xml.NewElement(xml.LocalName(root))
		el.Nodes = ns
		node = el
	}
	return xml.NewDocument(node), nil
}
