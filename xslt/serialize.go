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
	return x, err
}

func (s xmlSerializer) Serialize(w io.Writer, nodes []xml.Node) error {
	if !s.WrapRoot && len(nodes) != 1 {
		return fmt.Errorf("result tree has more than one root node")
	}
	root := s.rootNode(nodes)
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
	return writer.Write(doc)
}

func (s xmlSerializer) rootNode(nodes []xml.Node) xml.Node {
	if len(nodes) == 1 {
		return nodes[0]
	}
	if len(nodes) > 0 && s.WrapRoot {
		el := xml.NewElement(xml.LocalName(s.WrapName))
		for i := range nodes {
			el.Append(nodes[i])
		}
		return el
	}
	return xml.NewElement(xml.LocalName(s.WrapName))
}

type htmlSerializer struct {
	options xml.WriterOptions
	version string
}

func newHtmlSerializer(s *Stylesheet, n xml.Node) (Serializer, error) {
	return htmlSerializer{}, nil
}

func (htmlSerializer) Serialize(w io.Writer, nodes []xml.Node) error {
	return nil
}

type jsonSerializer struct{}

func newJsonSerializer(s *Stylesheet, n xml.Node) (Serializer, error) {
	return jsonSerializer{}, nil
}

func (jsonSerializer) Serialize(w io.Writer, nodes []xml.Node) error {
	return fmt.Errorf("not yet implemented")
}
