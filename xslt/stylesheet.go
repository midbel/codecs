package xslt

import (
	"io"

	"github.com/midbel/codecs/xml"
)

type Stylesheet struct {
	vars     xml.Environ[xml.Expr]
	params   xml.Environ[xml.Expr]
	builtins xml.Environ[xml.BuiltinFunc]

	templates []*Template

	context  string
	imported bool
	others   []*Stylesheet
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

func (s *Stylesheet) currentMode() {

}

func (s *Stylesheet) findTemplateByName(name, mode string) (*Template, error) {
	return nil, nil
}

func (s *Stylesheet) matchTemplateByNode(node xml.Node, mode string) (*Template, error) {
	return nil, nil
}

func (s *Stylesheet) executeQuery(query string, node xml.Node) ([]xml.Item, error) {
	return nil, nil
}

func (s *Stylesheet) compileQuery(query string) (xml.Expr, error) {
	return nil, nil
}

type Template struct {
	Name     string
	Mode     string
	Match    string
	Priority int
	Fragment xml.Node
}
