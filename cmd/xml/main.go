package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/midbel/codecs/relax"
	"github.com/midbel/codecs/xml"
)

func main() {
	options := struct {
		Root    string
		Query   string
		Schema  string
		Compact bool
		Check   bool
	}{}
	flag.StringVar(&options.Root, "r", "document", "root element name to use when using a query")
	flag.StringVar(&options.Query, "q", "", "search for element in document")
	flag.StringVar(&options.Schema, "s", "", "relax schema to validate XML document")
	flag.BoolVar(&options.Compact, "c", false, "write compact output")
	flag.BoolVar(&options.Check, "k", false, "validate only the document")
	flag.Parse()

	schema, err := parseSchema(options.Schema)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	doc, err := parseDocument(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if doc, err = search(doc, options.Query, options.Root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	if doc != nil {
		if err := schema.Validate(doc.Root()); err != nil {
			fmt.Fprintln(os.Stderr, "document does not conform to specify schema")
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if options.Check {
			fmt.Println("document is valid")
			return
		}
	}
	if err := writeDocument(doc, options.Compact); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(5)
	}
}

func piInclude(_ string, attrs []xml.Attribute) (xml.Node, error) {
	ix := slices.IndexFunc(attrs, func(a xml.Attribute) bool {
		return a.QualifiedName() == "filename"
	})
	if ix < 0 {
		return nil, fmt.Errorf("filename: attribute is missing")
	}
	doc, err := parseDocument(attrs[ix].Value)
	if err != nil {
		return nil, err
	}
	return doc.Root(), nil
}

func parseDocument(file string) (*xml.Document, error) {
	r, err := open(file)
	if err != nil {
		return nil, err
	}

	p := xml.NewParser(r)
	p.RegisterPI("include", piInclude)
	p.TrimSpace = true
	p.OmitProlog = true

	return p.Parse()
}

func writeDocument(doc *xml.Document, compact bool) error {
	if doc == nil {
		return nil
	}
	ws := xml.NewWriter(os.Stdout)
	ws.Compact = compact
	return ws.Write(doc)
}

func parseSchema(file string) (relax.Pattern, error) {
	if file == "" {
		return relax.Valid(), nil
	}
	r, err := open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	p := relax.Parse(r)
	return p.Parse()
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
