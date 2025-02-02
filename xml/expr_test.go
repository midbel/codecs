package xml_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/midbel/codecs/xml"
)

func TestExprFind(t *testing.T) {
	queries := []struct {
		Query string
	}{
		{
			Query: `/projects`,
		},
		{
			Query: `//*`,
		},
		{
			Query: `./projects/project`,
		},
		{
			Query: `//tag[position()=1]`,
		},
		{
			Query: `/projects//owner[upper-case(.)='midbel']`,
		},
		{
			Query: `//project[@id]`,
		},
		{
			Query: `//tag[lower-case(@value) = "api"]/parent::project/@id`,
		},
		{
			Query: `//tag | //owner`,
		},
		{
			Query: `//tag intersect //owner`,
		},
	}

	doc, err := sample()
	if err != nil {
		t.Errorf("fail to load sample document: %s", err)
		return
	}

	for _, q := range queries {
		e, err := xml.CompileString(q.Query)
		if err != nil {
			t.Errorf("fail to compile query %q: %s", q.Query, err)
			continue
		}
		items, err := e.Find(doc)
		if err != nil {
			t.Errorf("error finding node: %s", err)
			continue
		}
		_ = items
	}
}

func sample() (xml.Node, error) {
	r, err := os.Open(filepath.Join("testdata/sample.xml"))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	p := xml.NewParser(r)
	return p.Parse()
}
