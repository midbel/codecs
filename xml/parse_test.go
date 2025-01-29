package xml_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/midbel/codecs/xml"
)

func TestParse(t *testing.T) {
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