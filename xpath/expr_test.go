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
	<group>
		<item id="nest" lang="ung">qux</item>
	</group>
</root>
`

const docSpace = `<?xml version="1.0" encoding="utf-8"?>
<root xmlns="http://midbel.org/ns"
	xmlns:ang="http://midbel.org/angle">
	<ang:item id="fst" lang="en">foo</ang:item>
	<ang:item id="snd" lang="en">bar</ang:item>
</root>
`

type TestCase struct {
	Query   string
	Want    []string
	Options []Option
}

func TestArray(t *testing.T) {
	tests := []TestCase{
		{
			Query: "[1, 2, 3, 'test']",
			Want:  []string{"1", "2", "3", "test"},
		},
		{
			Query: "array{1, 2, 3}",
			Want:  []string{"1", "2", "3"},
		},
		{
			Query: "let $arr := array{1, 2, 3} return $arr(1)",
			Want:  []string{"1"},
		},
		{
			Query: "[[1, 2, 3], [4, 5, 6]](1)",
			Want:  []string{"1", "2", "3"},
		},
		{
			Query: "[[1, 2, 3], [4, 5, 6]](1)(2)",
			Want:  []string{"2"},
		},
		{
			Query: "array{1, 2, 3}(79)",
			Want:  []string{},
		},
	}
	runArrayTests(t, docBase, tests)
}

func TestSequence(t *testing.T) {
	tests := []TestCase{
		{
			Query: "1 to 3",
			Want:  []string{"1", "2", "3"},
		},
		{
			Query: "('item1', 'item2', (), ((), ()), ('item-4-1', 'item-4-2'))",
			Want:  []string{"item1", "item2", "item-4-1", "item-4-2"},
		},
	}
	runTests(t, docBase, tests)
}

func TestPathWithNS(t *testing.T) {
	options := []Option{
		WithNamespace("", "http://midbel.org/ns"),
		WithNamespace("ang", "http://midbel.org/angle"),
	}
	tests := []TestCase{
		{
			Query: "/root/item",
			Want:  []string{},
		},
		{
			Query: "/root/ang:item",
			Want:  []string{"foo", "bar"},
			Options: []Option{
				WithNoNamespace(),
			},
		},
		{
			Query: "/root/item",
			Want:  []string{},
			Options: []Option{
				WithNoNamespace(),
			},
		},
		{
			Query:   "/root/*:item",
			Want:    []string{"foo", "bar"},
			Options: options,
		},
		{
			Query: "/root/ang:*",
			Want:  []string{"foo", "bar"},
		},
		{
			Query:   "/root/ang:item",
			Want:    []string{"foo", "bar"},
			Options: options,
		},
	}
	runTests(t, docSpace, tests)
}

func TestPath(t *testing.T) {
	tests := []TestCase{
		{
			Query: "/root",
			Want:  []string{"foobarqux"},
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
			Want:  []string{"foo", "bar", "qux"},
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
		{
			Query: "/root/item/@id",
			Want:  []string{"fst", "snd"},
		},
		{
			Query: "/root/item[1] | /root/item[2]",
			Want:  []string{"foo", "bar"},
		},
		{
			Query: "/root/item[@id = \"fst\"]",
			Want:  []string{"foo"},
		},
		{
			Query: "/root/item[@id != \"snd\"]",
			Want:  []string{"foo"},
		},
		{
			Query: "/root/item[text() = \"\"]",
			Want:  []string{},
		},
		{
			Query: "/root/item[text() = \"foo\"]",
			Want:  []string{"foo"},
		},
		{
			Query: "//item[. = \"foo\"]",
			Want:  []string{"foo"},
		},
		{
			Query: "//item[. != \"bar\"]",
			Want:  []string{"foo", "qux"},
		},
		{
			Query: "//@id",
			Want:  []string{"fst", "snd", "nest"},
		},
		{
			Query: "//group//item",
			Want:  []string{"qux"},
		},
		{
			Query: "//group/*",
			Want:  []string{"qux"},
		},
	}
	runTests(t, docBase, tests)
}

func runArrayTests(t *testing.T, doc string, tests []TestCase) {
	t.Helper()

	root, err := xml.ParseString(doc)
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
		res, err := q.Find(root)
		if err != nil {
			t.Errorf("error finding node in document: %s", err)
			continue
		}
		if !res.Singleton() {
			t.Errorf("%s: expected singleton sequence", c.Query)
			continue
		}
		if s, ok := res[0].(interface{ Sequence() Sequence }); ok {
			res = s.Sequence()
		}
		got := getValuesFromSequence(res)
		if !slices.Equal(got, c.Want) {
			t.Errorf("%s: nodes mismatched! want %s, got %s", c.Query, c.Want, got)
		}
	}
}

func runTests(t *testing.T, doc string, tests []TestCase) {
	t.Helper()

	root, err := xml.ParseString(doc)
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
		res, err := q.Find(root)
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
