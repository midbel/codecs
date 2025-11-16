package xslt

import (
	"fmt"
	"io"

	"github.com/midbel/codecs/xml"
)

type Serializer interface {
	Serialize(io.Writer, []xml.Node) error
}

func defaultSerializer(s *Stylesheet) Serializer {
	return xmlSerializer{
		Stylesheet: s,
	}
}

type textSerializer struct{}

func newTextSerializer(_ *Stylesheet, _ xml.Node) (Serializer, error) {
	return textSerializer{}, nil
}

func (textSerializer) Serialize(w io.Writer, nodes []xml.Node) error {
	for i := range nodes {
		_, err := io.WriteString(w, nodes[i].Value())
		if err != nil {
			return err
		}
	}
	return nil
}

type xmlSerializer struct {
	options xml.WriterOptions
	doctype *xml.DocType
	*Stylesheet
}

func newXmlSerializer(s *Stylesheet, n xml.Node) (Serializer, error) {
	x := xmlSerializer{
		Stylesheet: s,
	}
	el, err := getElementFromNode(n)
	if err != nil {
		return nil, err
	}
	if i, err := getAttribute(el, "indent"); err != nil || i != "yes" {
		x.options |= xml.OptionCompact
	}
	if i, err := getAttribute(el, "omit-xml-declaration"); err != nil || i != "yes" {
		x.options |= xml.OptionNoProlog
	}
	var doctype xml.DocType
	if i, err := getAttribute(el, "doctype-public"); err == nil && i != "" {
		doctype.PublicID = i
	}
	if i, err := getAttribute(el, "doctype-system"); err == nil && i != "" {
		doctype.SystemID = i
	}
	if doctype.PublicID != "" || doctype.SystemID != "" {
		x.doctype = &doctype
	}
	return x, err
}

func (s xmlSerializer) Serialize(w io.Writer, nodes []xml.Node) error {
	if !s.WrapRoot && len(nodes) != 1 {
		return fmt.Errorf("result tree has more than one root node")
	}
	root := getRootNode(nodes, s.WrapRoot, xml.LocalName(s.WrapName))
	if el, ok := root.(*xml.Element); ok {
		for _, n := range s.aliases.Names() {
			pre, _ := s.aliases.Resolve(n)
			uri, err := s.env.ResolveNS(pre)
			if err != nil {
				continue
			}
			a := xml.NewAttribute(xml.QualifiedName(pre, "xmlns"), uri)
			el.SetAttribute(a)
		}
	}
	writer := xml.NewWriter(w)
	writer.WriterOptions = s.options
	var doc *xml.Document
	if d, ok := root.(*xml.Document); !ok {
		doc = xml.NewDocument(root)
	} else {
		doc = d
	}
	doc.DocType = s.doctype
	return writer.Write(doc)
}

var (
	doctypeHtml5 = xml.NewDocType("html", "", "")
	doctypeHtml4 = xml.NewDocType("html", "-//W3C//DTD HTML 4.01//EN", "http://www.w3.org/TR/html4/strict.dtd")
)

type htmlSerializer struct {
	options xml.WriterOptions
	doctype *xml.DocType
}

func newHtmlSerializer(_ *Stylesheet, n xml.Node) (Serializer, error) {
	h := htmlSerializer{
		doctype: doctypeHtml5,
		options: xml.OptionNoProlog,
	}
	el, err := getElementFromNode(n)
	if err != nil {
		return nil, err
	}
	if i, err := getAttribute(el, "indent"); err != nil || i != "yes" {
		h.options |= xml.OptionCompact
	}
	if i, err := getAttribute(el, "html-version"); err == nil {
		if i == "4" || i == "4.01" {
			h.doctype = doctypeHtml4
		}
	}
	return h, nil
}

func (s htmlSerializer) Serialize(w io.Writer, nodes []xml.Node) error {
	var (
		writer = xml.NewWriter(w)
		root   = getRootNode(nodes, false, xml.LocalName("html"))
	)
	writer.WriterOptions = s.options
	var doc *xml.Document
	if d, ok := root.(*xml.Document); !ok {
		doc = xml.NewDocument(root)
	} else {
		doc = d
	}
	doc.DocType = s.doctype
	return writer.Write(doc)
}

type jsonSerializer struct{}

func newJsonSerializer(s *Stylesheet, n xml.Node) (Serializer, error) {
	return jsonSerializer{}, nil
}

func (jsonSerializer) Serialize(w io.Writer, nodes []xml.Node) error {
	return fmt.Errorf("not implemented")
}

func getRootNode(nodes []xml.Node, wrapped bool, qname xml.QName) xml.Node {
	if len(nodes) == 1 {
		return nodes[0]
	}
	if len(nodes) > 0 && wrapped {
		el := xml.NewElement(qname)
		for i := range nodes {
			el.Append(nodes[i])
		}
		return el
	}
	return xml.NewElement(qname)
}
