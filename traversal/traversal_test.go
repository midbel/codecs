package traversal

import (
	"testing"
)

func TestCollect(t *testing.T) {
	tests := []struct {
		Paths []string
		Want  []any
	}{
		{
			Paths: []string{"first"},
			Want:  []any{"foo"},
		},
	}
	in := map[string]any{
		"first":  "foo",
		"last":   "bar",
		"answer": 42.0,
	}
	for _, c := range tests {
		got, err := Collect(in, c.Paths)
		if err != nil {
			t.Errorf("error collecting data: %s => %v", c.Paths, err)
			continue
		}
		if len(c.Want) != len(got) {
			t.Errorf("number of results mismatched! want %d, got %d", len(c.Want), len(got))
			continue
		}
		for i := range c.Want {
			if c.Want[i] != got[i] {
				t.Errorf("result mismatched! want %v, got %v", c.Want[i], got[i])
			}
		}
	}
}

func TestScan(t *testing.T) {
	tests := []struct {
		Input string
		Want  []token
	}{
		{
			Input: "$.first",
			Want: []token{
				{Literal: "", Type: Root},
				{Literal: "", Type: Dot},
				{Literal: "first", Type: Ident},
				{Literal: "", Type: Eof},
			},
		},
	}
	for _, c := range tests {
		scan := createScanner(c.Input)
		for i := 0; i < len(c.Want); i++ {
			tok := scan.Scan()
			if tok != c.Want[i] {
				t.Errorf("%s: unexpected token: %+v", c.Input, tok)
				break
			}
		}
		if tok := scan.Scan(); tok.Type != Eof {
			t.Errorf("%s: expected last token to be EOF", c.Input)
		}
	}
}
