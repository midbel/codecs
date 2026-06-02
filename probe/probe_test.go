package probe

import (
	"testing"
)

func TestTraverse(t *testing.T) {
	body := map[string]any{
		"protected": false,
		"owner": map[string]any{
			"name": "midbel",
			"repo": "https://github.com/midbel",
		},
		"languages": []any{
			map[string]any{
				"name":  "go",
				"star":  10.0,
				"usage": []any{"cli", "daemon"},
				"meta": map[string]any{
					"version": "1.26",
					"year":    2009.0,
					"typed":   true,
				},
			},
			map[string]any{
				"name": "rust",
				"meta": map[string]any{
					"version": "1.26",
					"year":    2012.0,
					"typed":   true,
				},
			},
			map[string]any{
				"name":  "js",
				"star":  8.0,
				"usage": []any{"cli", "browser"},
				"meta": map[string]any{
					"year":  1995.0,
					"typed": false,
				},
			},
			map[string]any{
				"name":  "ts",
				"star":  8.0,
				"usage": []any{"cli", "browser"},
				"meta": map[string]any{
					"version": "6",
					"year":    2012.0,
					"typed":   true,
				},
			},
			map[string]any{
				"name":  "java",
				"star":  6.0,
				"usage": []any{"cli", "daemon", "data"},
				"meta": map[string]any{
					"year":  1996.0,
					"typed": true,
				},
			},
		},
	}
	tests := []struct {
		Query string
		Want  any
		Opts  *Options
	}{
		{
			Query: "$.owner.name",
			Want:  "midbel",
		},
		{
			Query: "$.protected | $.owner.name",
			Want:  "midbel",
		},
		{
			Query: "$.owner.name, $.owner.repo",
			Want: []any{
				[]any{"midbel", "https://github.com/midbel"},
			},
		},
		{
			Query: "$.languages.name",
			Want: []any{
				[]any{"go", "rust", "js", "ts", "java"},
			},
		},
		{
			Query: "$.languages.star",
			Want: []any{
				[]any{10.0, nil, 8.0, 8.0, 6.0},
			},
		},
	}
	for _, c := range tests {
		got, err := Traverse(c.Query, body, c.Opts)
		if err != nil {
			t.Errorf("%s: unexpected error: %s", c.Query, err)
			continue
		}
		if !testEqual(got, c.Want) {
			t.Errorf("results mismatched! want %v, got %v", c.Want, got)
		}
	}
}

func testEqual(got, want any) bool {
	if isEqual(got, want) {
		return true
	}
	switch gs := got.(type) {
	case []any:
		ws, ok := want.([]any)
		if !ok {
			return false
		}
		if len(gs) != len(ws) {
			return false
		}
		for i := range gs {
			if !testEqual(gs[i], ws[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		return false
	default:
		return false
	}
}

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
