package xslt

import (
	"io"

	"github.com/midbel/codecs/xml"
)

type Stylesheet struct {
	vars   xml.Environ[xml.Expr]
	params xml.Environ[xml.Expr]

	templates []*Template
	others    []*Stylesheet
}

func Load(file string) (*Stylesheet, error) {
	return nil, nil
}

func (s *Stylesheet) Execute(doc xml.Node) (xml.Node, error) {
	return nil, nil
}

func (s *Stylesheet) Write(w io.Writer, doc xml.Node) error {
	return nil
}

type Template struct {
	Name     string
	Mode     string
	Match    string
	Priority int
	Fragment xml.Node
}
