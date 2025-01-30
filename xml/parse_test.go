package xml_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/midbel/codecs/xml"
)

func TestParseValidDocument(t *testing.T) {
	r, err := os.Open(filepath.Join("testdata/sample.xml"))
	if err != nil {
		t.Errorf("fail to open sample file: %s", err)
		return
	}
	defer r.Close()

	_, err = xml.NewParser(r).Parse()
	if err != nil {
		t.Errorf("fail to parse sample file: %s", err)
	}
}

const prolog = `<?xml version="1.0" encoding="UTF-8"?>`

func TestParseInvalidDocument(t *testing.T) {
	data := []struct {
		Xml        string
		Cause      string
		OmitProlog bool
	}{
		{
			Xml:   ``,
			Cause: "document without root element",
		},
		{
			Xml:        `<root></root>`,
			Cause:      "document without prolog",
			OmitProlog: true,
		},
		{
			Xml:   `<root empty-attr></root>`,
			Cause: "attribute without value",
		},
		{
			Xml:   `<root id="id-1" id="id-2"></root>`,
			Cause: "duplicate attribute",
		},
	}
	for _, d := range data {
		if !d.OmitProlog {
			d.Xml = prolog + d.Xml
		}
		str := strings.NewReader(d.Xml)
		_, err := xml.NewParser(str).Parse()
		if err == nil {
			t.Errorf("%s: invalid document parsed properly!", d.Cause)
		}
	}
}
