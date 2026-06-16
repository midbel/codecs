package probe

import (
	"testing"
)

func TestExecute(t *testing.T) {
	body := map[string]any{
		"protected": false,
		"owner": map[string]any{
			"name":    "midbel",
			"repo":    "https://github.com/midbel",
			"sponsor": nil,
		},
		"tags":       []any{},
		"repository": map[string]any{},
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
			Want:  "42",
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
			Query: "$.languages => .name, .star | 0",
			Want: []any{
				createArray("go", 10.0),
				createArray("rust", 0.0),
				createArray("js", 8.0),
				createArray("ts", 8.0),
				createArray("java", 6.0),
			},
		},
		{
			Query: "($.languages => .name, .star | 0), ($.owner => .name)",
			Want: &Result{
				Sets: []any{
					[]any{
						createArray("go", 10.0),
						createArray("rust", 0.0),
						createArray("js", 8.0),
						createArray("ts", 8.0),
						createArray("java", 6.0),
					},
					"midbel",
				},
			},
		},
		{
			Query: "$.languages.star:eq(10)",
			Want: []any{
				createArray(10.0),
			},
			Opts: &Options{
				Missing: MissingIgnore,
			},
		},
		{
			Query: "$.languages.meta.year:between(2000, 2020)",
			Want: []any{
				createArray(2009.0, 2012.0, 2012.0),
			},
			Opts: &Options{
				Missing: MissingIgnore,
			},
		},
		{
			Query: "$.languages.name:in(\"java\", \"go\")",
			Want: []any{
				createArray("go", "java"),
			},
		},
		{
			Query: "$.languages.star:ge(7), $.languages.star:gt(7)",
			Want: []any{
				createArray(10.0, 8.0, 8.0),
				createArray(10.0, 8.0, 8.0),
			},
			Opts: &Options{
				Missing: MissingIgnore,
				Zip:     NoZip,
			},
		},
		{
			Query: "$.languages.name, $.languages.meta.version",
			Want: []any{
				createArray("go", "1.26"),
				createArray("rust", "1.26"),
				createArray("js", "6"),
			},
			Opts: &Options{
				Missing: MissingIgnore,
				Zip:     ZipShort,
			},
		},
		{
			Query: "$.languages.name, $.languages.meta.version | \"?\"",
			Want: []any{
				createArray("go", "1.26"),
				createArray("rust", "1.26"),
				createArray("js", "?"),
				createArray("ts", "6"),
				createArray("java", "?"),
			},
			Opts: &Options{
				Missing: MissingIgnore,
				Zip:     ZipStrict,
			},
		},
		{
			Query: "$.owner:len()",
			Want:  3.0,
		},
		{
			Query: "$.owner.name:ifeq(\"midbel\", 100, 0)",
			Want:  100.0,
		},
		{
			Query: "$.owner.name:ifne(\"midbel\", 100, 0)",
			Want:  0.0,
		},
		{
			Query: "$.languages.name, $.languages.usage",
			Want: []any{
				createArray("go", "cli"),
				createArray("go", "daemon"),
				createArray("rust", "?"),
				createArray("js", "cli"),
				createArray("js", "browser"),
				createArray("ts", "cli"),
				createArray("ts", "browser"),
				createArray("java", "cli"),
				createArray("java", "daemon"),
				createArray("java", "data"),
			},
			Opts: &Options{
				Missing:      MissingReplace,
				MissingValue: "?",
			},
		},
		{
			Query: "$.languages:at(1).name",
			Want:  "rust",
		},
		{
			Query: "$.languages:at(1000).name",
			Want:  nil,
		},
		{
			Query: "$.tags:empty()",
			Want:  []any{createArray()},
		},
		{
			Query: "$.owner.sponsor:null()",
			Want:  nil,
		},
		{
			Query: "$.owner.name:null()",
			Want:  nil,
		},
	}
	for _, c := range tests {
		got, err := Execute(c.Query, body, c.Opts)
		if err != nil {
			t.Errorf("%s: unexpected error: %s", c.Query, err)
			continue
		}
		var want *Result
		if w, ok := c.Want.(*Result); !ok {
			want = &Result{
				Sets: []any{c.Want},
			}
		} else {
			want = w
		}
		if !compareResults(got, want) {
			t.Errorf("%s: results mismatched! want %v, got %v", c.Query, want, got)
		}
	}
}

func compareResults(got, want *Result) bool {
	if len(got.Sets) != len(want.Sets) {
		return false
	}
	for i := range got.Sets {
		if !testEqual(got.Sets[i], want.Sets[i]) {
			return false
		}
	}
	return true
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
