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
		Root      string
		Query     string
		Schema    string
		Compact   bool
		Check     bool
		List      bool
		File      string
		NoSpace   bool
		NoComment bool
		Mode      string
	}{}
	flag.StringVar(&options.File, "f", "", "output file")
	flag.StringVar(&options.Root, "r", "document", "root element name to use when using a query")
	flag.StringVar(&options.Query, "q", "", "search for element in document")
	flag.StringVar(&options.Mode, "m", "", "compile mode for xpath query (xsl, xpath)")
	flag.StringVar(&options.Schema, "s", "", "relax schema to validate XML document")
	flag.BoolVar(&options.Compact, "c", false, "write compact output")
	flag.BoolVar(&options.Check, "k", false, "validate only the document")
	flag.BoolVar(&options.List, "l", false, "print results as list")
	flag.BoolVar(&options.NoComment, "no-comment", false, "do not write comment in output")
	flag.BoolVar(&options.NoSpace, "no-namespace", false, "do not write xmlns attributes and namespace in output")
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
	if doc, err = search(doc, options.Query, options.Mode, options.Root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	if doc != nil {
		if err := schema.Validate(doc.Root()); err != nil {
			fmt.Fprintln(os.Stderr, "document does not conform to specify schema")
			if err, ok := err.(relax.NodeError); ok {
				fmt.Fprintln(os.Stderr, xml.WriteNode(err.Node))
				fmt.Fprintln(os.Stderr)
			}
			fmt.Fprintf(os.Stderr, "cause: %s", err)
			fmt.Fprintln(os.Stderr)
			os.Exit(2)
		}
		if options.Check {
			fmt.Println("document is valid")
			return
		}
	}
	if options.List {
		root := doc.Root()
		elem, ok := root.(*xml.Element)
		if !ok {
			return
		}
		for i, n := range elem.Nodes {
			fmt.Println(i+1, n.Identity(), n.Value())
		}
	} else {
		if err := writeDocument(doc, options.File, options.Compact, options.NoComment, options.NoSpace); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(5)
		}
	}
}

func piInclude(_ string, attrs []xml.Attribute) (xml.Node, error) {
	ix := slices.IndexFunc(attrs, func(a xml.Attribute) bool {
		return a.QualifiedName() == "filename"
	})
	if ix < 0 {
		return nil, fmt.Errorf("filename: attribute is missing")
	}
	doc, err := parseDocument(attrs[ix].Value())
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

func writeDocument(doc *xml.Document, file string, compact, noComment, noNamespace bool) error {
	if doc == nil {
		return nil
	}
	var w io.Writer = os.Stdout
	if file != "" {
		f, err := os.Create(file)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	ws := xml.NewWriter(w)
	ws.NoNamespace = noNamespace
	ws.NoComment = noComment
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
		return os.Open(file)
	}
}

func search(doc *xml.Document, query, mode, root string) (*xml.Document, error) {
	if query == "" {
		return doc, nil
	}
	var cpMode xml.StepMode
	switch mode := strings.ToLower(mode); mode {
	case "", "xpath":
		cpMode = xml.ModeDefault
	case "xsl", "xsl2", "xsl3":
		cpMode = xml.ModeXsl
	default:
		return nil, fmt.Errorf("%s: mode not supported", mode)
	}

	expr, err := xml.CompileMode(strings.NewReader(query), cpMode)
	if err != nil {
		return nil, err
	}
	list, err := expr.Find(doc)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("query gives no result")
	}
	node := xml.NewElement(xml.LocalName(root))
	if len(list) == 1 {
		node.Append(list[0].Node())
	} else {
		node.Nodes = getNodesFromList(list)
	}
	return xml.NewDocument(node), nil
}

func getNodesFromList(list []xml.Item) []xml.Node {
	var all []xml.Node
	for _, i := range list {
		all = append(all, i.Node())
	}
	return all
}
