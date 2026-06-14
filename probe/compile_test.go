package probe

import (
	"testing"
)

func TestCompile(t *testing.T) {
	tests := []struct {
		Input   string
		Invalid bool
	}{
		{
			Input: "$.foo",
		},
		{
			Input: ".foo",
		},
		{
			Input: "$.foo, $.bar",
		},
		{
			Input: ".foo, .bar",
		},
		{
			Input: "$.foo.\"eq\"",
		},
		{
			Input: "$.foo.100",
		},
		{
			Input:   "$.foo.",
			Invalid: true,
		},
		{
			Input:   "$.foo,",
			Invalid: true,
		},
		{
			Input: "$.foo | \"value\"",
		},
		{
			Input: "$.foo | \"value\" | 0.1",
		},
		{
			Input: "$.foo | $.bar | 0",
		},
		{
			Input: "$.foo:eq(100) | $.bar:ifexists(\"fst\", \"snd\")",
		},
		{
			Input:   "$.foo:eq",
			Invalid: true,
		},
		{
			Input:   "$.foo:last(",
			Invalid: true,
		},
		{
			Input:   "$.foo:gt(100,)",
			Invalid: true,
		},
		{
			Input:   "$.foo:\"r\"()",
			Invalid: true,
		},
		{
			Input:   "$.foo:100()",
			Invalid: true,
		},
		{
			Input: "($.foo, $.bar)",
		},
		{
			Input: "($.foo, $.bar), ($.foo, $.quz)",
		},
		{
			Input: "$.quz => $.foo, $.bar | 0",
		},
		{
			Input: "($.quz => $.foo, $.bar), ($.foo, $.bar)",
		},
		{
			Input: "$.foo => ($.bar => $.foo, $.bar)",
		},
	}
	for _, c := range tests {
		_, err := CompilePath(c.Input)
		if c.Invalid && err == nil {
			t.Errorf("%s: invalid input compiled successfully!", c.Input)
		} else if !c.Invalid && err != nil {
			t.Errorf("%s: fail to compile valid input: %s", c.Input, err)
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
		{
			Input: "$.repos.name",
			Want: []token{
				{Literal: "", Type: Root},
				{Literal: "", Type: Dot},
				{Literal: "repos", Type: Ident},
				{Literal: "", Type: Dot},
				{Literal: "name", Type: Ident},
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
