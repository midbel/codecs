package xslt_test

import (
	"testing"
	"path/filepath"
	"bytes"
	"fmt"
	"os"

	"github.com/midbel/codecs/xml"
	"github.com/midbel/codecs/xslt"

)

func TestTransform(t *testing.T) {
	tests := []struct{
		Name string
		Dir string
	}{
		{
			Name: "value-of",
			Dir: "testdata/valueof",
		},
	}
	for _, tt := range tests {
		fn := executeTest(tt.Name, tt.Dir)
		t.Run(tt.Name, fn)
	}
}

func executeTest(name, dir string) func(*testing.T) {
	return func(t *testing.T) {
		doc, err := parseDocument(filepath.Join(dir, "doc.xml"))
		if err != nil {
			t.Errorf("error loading document: %s", err)
			return
		}
		sheet, err := xslt.Load(filepath.Join(dir, "transform.xslt"), "testdata")
		if err != nil {
			t.Errorf("error loading stylesheet: %s", err)
			return
		}
		var str bytes.Buffer
		if err := sheet.Generate(&str, doc); err != nil {
			t.Errorf("error executing transform: %s", err)
			return
		}
		err = compareBytes(t, filepath.Join(dir, "result.xml"), str.Bytes())
		if err != nil {
			t.Errorf("comparing results mismatched")
		}
	}
}

func compareBytes(t *testing.T, file string, got []byte) error {
	want, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if !bytes.Equal(want, got) {
		return fmt.Errorf("bytes mismatched")
	}
	return nil
}

func parseDocument(file string) (*xml.Document, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	p := xml.NewParser(r)
	return p.Parse()
}