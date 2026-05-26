package xml

import (
	"io"
)

func Decode(r io.Reader) (any, error) {
	doc, err := ParseReader(r)
	if err != nil {
		return nil, err
	}
	return doc.Map()
}
