package inspect

import (
	"fmt"
	"os"

	"github.com/midbel/codecs/xml"
)

type Counter struct {
}

func Infos(file string) (*Counter, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	rs := xml.NewReader(r)
	rs.OnOpenAny(func(rs *xml.Reader, e xml.E) error {
		fmt.Println("new element", e.QualifiedName())
		return nil
	})
	return nil, rs.Start()
}
