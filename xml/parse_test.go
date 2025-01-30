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

func TestCompileValidExpression(t *testing.T) {
	data := []string{
		`/`,
		`//*`,
		`/projects`,
		`/child::projects`,
		`/child::dev:projects`,
		`./dev:projects`,
		`/projects/descendant::project`,
		`/projects//owner`,
		`/projects//owner | //maintainer`,
		`//owner union //maintainer intersect /projects/project except /projects/project[@deprecated=false()]`,
		`//owner[../repository/@active[true()]]`,
		`//owner[string-length(normalize-space(./@alias)) != 0]`,
		`//url[substring(., 1, 4) = 'https']`,
		`some $x in (1, 2, 3) satisfies $x mod 10 < 10`,
		`some $x in (1, 2, 3), $y in ("foo", "bar") satisfies $x || $y = 'test'`,
		`if ($x != 'foobar' and $y = -3.4e-101) then ./@id else ./owner`,
		`for $x in (1, 2, 3), $y in ('foo', 'bar') return $x || $y`,
		`if ($x castable as xs:string) then $x cast as xs:string else $x cast as xs:decimal`,
	}
	for _, str := range data {
		r := strings.NewReader(str)
		_, err := xml.Compile(r)
		if err != nil {
			t.Errorf("compile expression failed: %s: %s", str, err)
		}
	}
}
