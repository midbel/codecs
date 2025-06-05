package xslt_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/midbel/codecs/xml"
	"github.com/midbel/codecs/xslt"
)

func TestTransform(t *testing.T) {
	tests := []struct {
		Name   string
		Dir    string
		Failed bool
	}{
		{
			Name: "value-of",
			Dir:  "testdata/valueof-basic",
		},
		{
			Name: "value-of/empty",
			Dir:  "testdata/valueof-empty",
		},
		{
			Name: "value-of/separator",
			Dir:  "testdata/valueof-separator",
		},
		{
			Name: "value-of/body",
			Dir:  "testdata/valueof-body",
		},
		{
			Name: "value-of/body-sep",
			Dir:  "testdata/valueof-body-sep",
		},
		{
			Name:   "value-of/select-error",
			Dir:    "testdata/valueof-errselect",
			Failed: true,
		},
		{
			Name:   "if/test-true",
			Dir:    "testdata/if-basic-true",
		},
		{
			Name:   "if/test-false",
			Dir:    "testdata/if-basic-false",
		},
	}
	for _, tt := range tests {
		fn := executeTest(tt.Name, tt.Dir, tt.Failed)
		t.Run(tt.Name, fn)
	}
}

func executeTest(name, dir string, failure bool) func(*testing.T) {
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
			if failure {
				return
			}
			t.Errorf("error executing transform: %s", err)
			return
		}
		if failure {
			t.Errorf("expected error but transformation pass!")
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
		t.Log("want:", string(want))
		t.Log("got :", string(got))
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
