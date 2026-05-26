package xml

import (
	"fmt"
	"strings"
)

type QName struct {
	Uri   string
	Space string
	Name  string
}

func ParseName(name string) (QName, error) {
	var (
		qn QName
		ok bool
	)
	qn.Space, qn.Name, ok = strings.Cut(name, ":")
	if !ok {
		qn.Name, qn.Space = qn.Space, ""
	}
	if ok && qn.Space == "" {
		return qn, fmt.Errorf("invalid namespace")
	}
	return qn, nil
}

func ExpandedName(name, space, uri string) QName {
	return QName{
		Name:  name,
		Space: space,
		Uri:   uri,
	}
}

func LocalName(name string) QName {
	return ExpandedName(name, "", "")
}

func QualifiedName(name, space string) QName {
	return ExpandedName(name, space, "")
}

func (q QName) Zero() bool {
	return q.isDocumentNode()
}

func (q QName) Equal(other QName) bool {
	return q.Uri == other.Uri && q.Name == other.Name
}

func (q QName) LocalName() string {
	return q.Name
}

func (q QName) ExpandedName() string {
	if q.Uri == "" {
		return q.LocalName()
	}
	return fmt.Sprintf("{%s}%s", q.Uri, q.Name)
}

func (q QName) QualifiedName() string {
	if q.Space == "" {
		return q.LocalName()
	}
	return fmt.Sprintf("%s:%s", q.Space, q.Name)
}

func (q QName) isDocumentNode() bool {
	return q.Space == "" && q.Name == ""
}
