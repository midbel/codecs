package xml_test

import (
	"strings"
	"testing"

	"github.com/midbel/codecs/xml"
)

func TestWriterWrite(t *testing.T) {
	const str = `<?xml version="1.0" encoding="UTF-8"?><test:root id="1"><test:a attr="text">text</test:a><test:a attr="self"/></test:root>`

	doc, err := parseDocument(str)
	if err != nil {
		t.Errorf("fail to parse input document: %s", err)
		return
	}

	data := []struct {
		Want        string
		Compact     bool
		NoProlog    bool
		NoNamespace bool
	}{
		{
			Want:     `<test:root id="1"><test:a attr="text">text</test:a><test:a attr="self"/></test:root>`,
			Compact:  true,
			NoProlog: true,
		},
		{
			Want:    `<?xml version="1.0" encoding="UTF-8"?><test:root id="1"><test:a attr="text">text</test:a><test:a attr="self"/></test:root>`,
			Compact: true,
		},
		{
			Want: strings.Join([]string{
				`<?xml version="1.0" encoding="UTF-8"?>`,
				``,
				`<test:root id="1">`,
				`    <test:a attr="text">text</test:a>`,
				`    <test:a attr="self"/>`,
				`</test:root>`,
			}, "\n"),
		},
		{
			Want: strings.Join([]string{
				`<?xml version="1.0" encoding="UTF-8"?>`,
				``,
				`<root id="1">`,
				`    <a attr="text">text</a>`,
				`    <a attr="self"/>`,
				`</root>`,
			}, "\n"),
			NoNamespace: true,
		},
	}

	for _, d := range data {
		var (
			buf strings.Builder
			ws  = xml.NewWriter(&buf)
		)
		ws.Compact = d.Compact
		ws.NoProlog = d.NoProlog
		ws.NoNamespace = d.NoNamespace
		if err := ws.Write(doc); err != nil {
			t.Errorf("error writing document: %s", err)
			return
		}
		got := buf.String()
		if got != d.Want {
			t.Errorf("result mismatched")
			t.Logf("want: %s", d.Want)
			t.Logf("got : %s", got)
		}
	}
}

func parseDocument(doc string) (*xml.Document, error) {
	return xml.NewParser(strings.NewReader(doc)).Parse()
}
