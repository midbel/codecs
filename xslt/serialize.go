package xslt

import (
	"fmt"
	"io"

	"github.com/midbel/codecs/xml"
)

type Serializer interface {
	Serialize(io.Writer, []xml.Node) error
}

type textSerializer struct{}

func (textSerializer) Serialize(w io.Writer, nodes []xml.Node) error {
	return nil
}

type xmlSerializer struct {
	wrapRoot bool
	wrapName string

	options xml.WriterOptions
}

func (xmlSerializer) Serialize(w io.Writer, nodes []xml.Node) error {
	return nil
}

type htmlSerializer struct {
	options xml.WriterOptions
	version string
}

func (htmlSerializer) Serialize(w io.Writer, nodes []xml.Node) error {
	return nil
}

type jsonSerializer struct{}

func (jsonSerializer) Serialize(w io.Writer, nodes []xml.Node) error {
	return fmt.Errorf("not yet implemented")
}
