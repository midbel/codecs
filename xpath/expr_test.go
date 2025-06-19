package xpath

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/midbel/codecs/xml"
)

const document = `<?xml version="1.0" encoding="UTF-8"?>

<root>
	<item id="first">element-1</item>
	<item id="second">element-2</item>
	<group>
		<item lang="en">sub-element-1</item>
		<item lang="en">sub-element-2</item>
		<test ignore="true"/>
	</group>
</root>
`

type TestCase struct {
	Expr     string
	Count    int
	Expected []string
}

func TestFilter(t *testing.T) {
	tests := []TestCase{
		{
			Expr:     "/root/item[last()]",
			Expected: []string{"element-2"},
		},
		{
			Expr:     "/root/item[position()>=1]",
			Expected: []string{"element-1", "element-2"},
		},
		{
			Expr:     "/root/item[position()>1]",
			Expected: []string{"element-2"},
		},
		{
			Expr:     "//item[text()=\"element-1\"]",
			Expected: []string{"element-1"},
		},
		{
			Expr:     "//test[@ignore=\"true\"]",
			Expected: []string{""},
		},
		{
			Expr:     "//*[@ignore!=\"false\"]",
			Expected: []string{""},
		},
		{
			Expr:     "//test[@ignore=\"false\"]",
			Expected: []string{},
		},
		{
			Expr:     "//*[@ignore=\"false\"]",
			Expected: []string{},
		},
		{
			Expr:     "//*[not(self::item)]",
			Expected: []string{"root", "group", "test"},
		},
	}
	runTestCase(t, tests)
}

func TestIndex(t *testing.T) {
	tests := []TestCase{
		{
			Expr:     "/root/item[1]",
			Expected: []string{"element-1"},
		},
	}
	runTestCase(t, tests)
}

func TestFunction(t *testing.T) {
	tests := []TestCase{
		{
			Expr:     "count(//item))",
			Expected: []string{"4"},
		},
	}
	runTestCase(t, tests)
}

func TestSequence(t *testing.T) {
	tests := []TestCase{
		{
			Expr:     "1 to 3",
			Expected: []string{"1", "2", "3"},
		},
		{
			Expr:     "('item1', 'item2', (), ((), ()), ('item-4-1', 'item-4-2'))",
			Expected: []string{"item1", "item2", "item-4-1", "item-4-2"},
		},
	}
	runTestCase(t, tests)
}

func TestPath(t *testing.T) {
	tests := []TestCase{
		{
			Expr:     "/root/item",
			Expected: []string{"element-1", "element-2"},
		},

		{
			Expr:     "//item",
			Expected: []string{"element-1", "element-2", "sub-element-1", "sub-element-2"},
		},
		{
			Expr:     "//group/item[1]",
			Expected: []string{"sub-element-1"},
		},
		{
			Expr:     "/root/item[1] | /root/item[2]",
			Expected: []string{"element-1", "element-2"},
		},
		{
			Expr:     "//@ignore",
			Expected: []string{"true"},
		},
	}
	runTestCase(t, tests)
}

func runTestCase(t *testing.T, tests []TestCase) {
	t.Helper()

	doc, err := parseDocument()
	if err != nil {
		t.Errorf("fail to parse document: %s", err)
		return
	}
	for _, c := range tests {
		q, err := CompileString(c.Expr)
		if err != nil {
			t.Errorf("fail to parse xpath expression %s: %s", c.Expr, err)
			continue
		}
		seq, err := q.Find(doc)
		if err != nil {
			t.Errorf("error evaluating expression %s: %s", c.Expr, err)
			continue
		}
		if seq.Len() != len(c.Expected) {
			t.Errorf("%s: number of nodes mismatched! want %d, got %d", c.Expr, len(c.Expected), seq.Len())
			continue
		}
		if got, ok := compareValues(seq, c.Expected); !ok {
			t.Errorf("%s: nodes mismatched! want %s, got %s", c.Expr, c.Expected, got)
		}
	}
}

func compareValues(seq Sequence, values []string) ([]string, bool) {
	var got []string
	for i := range seq {
		var (
			val = seq[i].Value()
			str string
		)
		switch v := val.(type) {
		case time.Time:
			str = v.Format("2006-01-02")
		case float64:
			str = strconv.FormatFloat(v, 'f', -1, 64)
		case bool:
			str = strconv.FormatBool(v)
		case string:
			str = v
		}
		got = append(got, str)
		if str != values[i] {
			return got, false
		}
	}
	return nil, true
}

func parseDocument() (*xml.Document, error) {
	p := xml.NewParser(strings.NewReader(document))
	return p.Parse()
}
