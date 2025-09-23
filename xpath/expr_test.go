package xpath

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/midbel/codecs/xml"
)

const docBase = `<?xml version="1.0" encoding="utf-8"?>

<root>
	<item id="fst" lang="en">foo</item>
	<item id="snd" lang="en">bar</item>
</root>
`

const docSpace = `<?xml version="1.0" encoding="utf-8"?>
<root xmlns="http://midbel.org/ns"
	xmlns:ang="http://midbel.org/angle">
	<ang:item id="fst" lang="en">foo</ang:item>
	<ang:item id="snd" lang="en">bar</ang:item>
</root>
`

type FindTestCase struct {
	Query   string
	Want    []string
	Options []Option
}

func TestExprFind(t *testing.T) {
	tests := []FindTestCase{
		{
			Query: "/root",
			Want:  []string{"foobar"},
		},
		{
			Query: "/foobar",
			Want:  []string{},
		},
		{
			Query: "/root/item",
			Want:  []string{"foo", "bar"},
		},
		{
			Query: "//item",
			Want:  []string{"foo", "bar"},
		},
		{
			Query: "/root/item[1]",
			Want:  []string{"foo"},
		},
		{
			Query: "/root/item[last()]",
			Want:  []string{"bar"},
		},
		{
			Query: "/root/item[position()-1]",
			Want:  []string{"bar"},
		},
	}
	runTests(t, docBase, tests)
}

func runTests(t *testing.T, doc string, tests []FindTestCase) {
	t.Helper()

	doc, err := xml.ParseString(doc)
	if err != nil {
		t.Errorf("fail to parse xml document: %s", err)
		return
	}
	for _, c := range tests {
		q, err := BuildWith(c.Query, c.Options...)
		if err != nil {
			t.Errorf("fail to build xpath query: %s", err)
			continue
		}
		res, err := q.Find(doc)
		if err != nil {
			t.Errorf("error finding node in document: %s", err)
			continue
		}
		if res.Len() != len(c.Want) {
			t.Errorf("%s: want %d results, got %d", c.Query, len(c.Want), res.Len())
			continue
		}
		got := getValuesFromSequence(res)
		if !slices.Equal(got, c.Want) {
			t.Errorf("%s: nodes mismatched! want %s, got %s", c.Query, c.Want, got)
		}
	}
}

func getValuesFromSequence(seq Sequence) []string {
	var list []string
	for _, i := range seq {
		var (
			val = i.Value()
			str string
		)
		switch v := val.(type) {
		default:
		case time.Time:
			str = v.Format("2006-01-02")
		case float64:
			str = strconv.FormatFloat(v, 'f', -1, 64)
		case bool:
			str = strconv.FormatBool(v)
		case string:
			str = v
		}
		list = append(list, str)
	}
	return list
}
