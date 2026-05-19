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
