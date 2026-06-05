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
				createArray("midbel", "https://github.com/midbel"),
			},
		},
		{
			Query: "$.languages.name",
			Want: []any{
				createArray("go", "rust", "js", "ts", "java"),
			},
		},
		{
			Query: "$.languages.star",
			Want: []any{
				createArray(10.0, nil, 8.0, 8.0, 6.0),
			},
		},
		{
			Query: "$.languages.usage:first()",
			Want: []any{
				createArray("cli", nil, "cli", "cli", "cli"),
			},
		},
		{
			Query: "$.languages.usage:first()",
			Want: []any{
				createArray("cli", "cli", "cli", "cli"),
			},
			Opts: &Options{
				Missing: MissingIgnore,
			},
		},
		{
			Query: "$.languages.usage:last()",
			Want: []any{
				createArray("daemon", nil, "browser", "browser", "data"),
			},
		},
		{
			Query: "$.owner.age:default(\"42\")",
			Want:  42.0,
		},
		{
			Query: "$.languages.usage:first()",
			Want: []any{
				createArray("cli", "*", "cli", "cli", "cli"),
			},
			Opts: &Options{
				Missing:      MissingReplace,
				MissingValue: "*",
			},
		},
		{
			Query: "$.languages.name, $.languages.star | 0",
			Want: []any{
				createArray("go", 10.0),
				createArray("rust", 0.0),
				createArray("js", 8.0),
				createArray("ts", 8.0),
				createArray("java", 6.0),
			},
		},
		{
			Query: "$.language.star:eq(10)",
			Want: []any{
				createArray(10.0),
			},
		},
		{
			Query: "$.language.star:ge(7)",
			Want: []any{
				createArray(10.0, 8.0, 8.0),
			},
		},
		{
			Query: "$.owner:len()",
			Want:  createArray(2.0),
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

func createArray(vals ...any) []any {
	return vals
}
