package main

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"

	"github.com/midbel/codecs/xml"
)

var ErrDocument = errors.New("bad xml document")

const (
	snakeCaseType = "snake"
	kebabCaseType = "kebab"
	lowerCaseType = "lower"
)

type WriterOptions struct {
	NoNamespace bool
	NoProlog    bool
	NoComment   bool
	Compact     bool
	CaseType    string
}

type Document struct {
	File string
	*xml.Document
}

func iterDocuments(files []string) iter.Seq2[*Document, error] {
	get := func(file string, doc *xml.Document) *Document {
		return &Document{
			File:     file,
			Document: doc,
		}
	}

	parse := func(file string) (*Document, error) {
		var opts ParserOptions
		doc, err := parseDocument(file, opts)
		if err != nil {
			return nil, ErrDocument
		}
		return get(file, doc), nil
	}

	fn := func(yield func(*Document, error) bool) {
		for _, f := range files {
			if s, err := os.Stat(f); err == nil && s.IsDir() {
				es, err := os.ReadDir(f)
				if err != nil {
					yield(nil, err)
					return
				}
				for _, e := range es {
					doc, err := parse(filepath.Join(f, e.Name()))
					if !yield(doc, err) {
						break
					}
				}
			} else {
				doc, err := parse(f)
				if !yield(doc, err) {
					break
				}
			}
		}
	}
	return fn
}

type ParserOptions struct {
	Include    bool
	StrictNS   bool
	KeepEmpty  bool
	OmitProlog bool
	Transform  bool
}

func parseDocument(file string, opts ParserOptions) (*xml.Document, error) {
	if file == "" {
		root := xml.NewElement(xml.LocalName("angle"))
		return xml.NewDocument(root), nil
	}
	r, err := openFile(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	p := xml.NewParser(r)
	p.OmitProlog = opts.OmitProlog
	p.StrictNS = opts.StrictNS
	p.KeepEmpty = opts.KeepEmpty

	if opts.Include {
		p.RegisterPI("angle-include", piInclude)
	}
	return p.Parse()
}

func piInclude(_ string, attrs []xml.Attribute) (xml.Node, error) {
	ix := slices.IndexFunc(attrs, func(a xml.Attribute) bool {
		return a.QualifiedName() == "filename"
	})
	if ix < 0 {
		return nil, fmt.Errorf("filename attribute missing")
	}
	var opts ParserOptions
	doc, err := parseDocument(attrs[ix].Value(), opts)
	if err != nil {
		return nil, err
	}
	ix = slices.IndexFunc(attrs, func(a xml.Attribute) bool {
		return a.QualifiedName() == "query"
	})
	if ix >= 0 {

	}
	return doc.Root(), nil
}

func writeDocument(doc *xml.Document, file string, options WriterOptions) error {
	if doc == nil {
		return fmt.Errorf("no document to be written")
	}
	var w io.Writer = os.Stdout
	if file != "" {
		f, err := os.Create(file)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f

		if filepath.Ext(file) == ".gz" {
			z, _ := gzip.NewWriterLevel(w, gzip.BestCompression)
			defer z.Close()

			w = z
		}
	}

	ws := xml.NewWriter(w)
	if options.NoNamespace {
		ws.WriterOptions |= xml.OptionNoNamespace
	}
	if options.NoComment {
		ws.WriterOptions |= xml.OptionNoComment
	}
	if options.Compact {
		ws.WriterOptions |= xml.OptionCompact
	}
	switch options.CaseType {
	case snakeCaseType:
		ws.WriterOptions |= xml.OptionNamespaceSnakeCase | xml.OptionNameSnakeCase
	case kebabCaseType:
		ws.WriterOptions |= xml.OptionNamespaceKebabCase | xml.OptionNameKebabCase
	case lowerCaseType:
		ws.WriterOptions |= xml.OptionNamespaceLowerCase | xml.OptionNameLowerCase
	default:
	}
	return ws.Write(doc)
}

func openFile(file string) (io.ReadCloser, error) {
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
