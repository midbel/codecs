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
			Input:   "$.foo.",
			Invalid: true,
		},
		{
			Input:   "$.foo,",
			Invalid: true,
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
