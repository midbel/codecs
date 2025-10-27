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

const docNumbers = `<?xml version="1.0" encoding="utf-8"?>

<root>
	<item>
		<label>foo</label>
		<star>10</star>
	</item>
	<item>
		<label>foo</label>
		<star>20</star>
	</item>
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

func TestIf(t *testing.T) {
	tests := []TestCase{
		{
			Query: "if (/root/item[1] = 'foo') then 'ok' else 'nok'",
			Want:  []string{"ok"},
		},
		{
			Query: "if (/root/item[1] = 'bar') then 'ok' else 'nok'",
			Want:  []string{"nok"},
		},
	}
	runTests(t, docBase, tests)
}

func TestFor(t *testing.T) {
	tests := []TestCase{
		{
			Query: "for $i in 1 to 5 return $i",
			Want:  []string{"1", "2", "3", "4", "5"},
		},
	}
	runTests(t, docBase, tests)
}

func TestLet(t *testing.T) {
	tests := []TestCase{
		{
			Query: "let $x := -1 return $x",
			Want:  []string{"-1"},
		},
		{
			Query: "let $x := 1, $y := 1 return $x+$y",
			Want:  []string{"2"},
		},
	}
	runTests(t, docBase, tests)
}

func TestQuantified(t *testing.T) {
	tests := []TestCase{
		{
			Query: "every $x in (1, 2, 3) satisfies $x <= 10",
			Want:  []string{"true"},
		},
		{
			Query: "every $x in (1, 2, 3) satisfies $x > 10",
			Want:  []string{"false"},
		},
		{
			Query: "some $x in (1, 2, 3) satisfies $x > 10",
			Want:  []string{"false"},
		},
		{
			Query: "some $x in (1, 2, 13) satisfies $x > 10",
			Want:  []string{"true"},
		},
		{
			Query: "some $x in (1, 2, 13), $y in (1, 2) satisfies $x * $y > 10",
			Want:  []string{"true"},
		},
		{
			Query: "some $el in //item satisfies contains(string($el), 'foo')",
			Want:  []string{"true"},
		},
		{
			Query: "some $el in //* satisfies exists($el/@id)",
			Want:  []string{"true"},
		},
		{
			Query: "every $el in /root/items satisfies 1=1",
			Want:  []string{"true"},
		},
		{
			Query: "some $el in /root/items satisfies 1=1",
			Want:  []string{"false"},
		},
		{
			Query: "some $x in (1, 2), $y in () satisfies $x + $y > 0",
			Want:  []string{"false"},
		},
		{
			Query: "every $x in (1, 2), $y in (3, 4) satisfies $x < $y",
			Want:  []string{"true"},
		},
	}
	runTests(t, docBase, tests)
}

func TestOperators(t *testing.T) {
	tests := []TestCase{
		{
			Query: "'foo'||'bar'",
			Want:  []string{"foobar"},
		},
		{
			Query: "/root/item[1] is /root/item[1]",
			Want:  []string{"true"},
		},
		{
			Query: "/root/item[1] is /root/item[2]",
			Want:  []string{"false"},
		},
		{
			Query: "/root/item[2] is /root/item[1]",
			Want:  []string{"false"},
		},
		{
			Query: "/root/item[1] >> /root/item[2]",
			Want:  []string{"false"},
		},
		{
			Query: "/root/item[1] << /root/item[2]",
			Want:  []string{"true"},
		},
		{
			Query: "/root/item[2] >> /root/item[1]",
			Want:  []string{"true"},
		},
		{
			Query: "/root/item[2] << /root/item[1]",
			Want:  []string{"false"},
		},
	}
	runTests(t, docBase, tests)
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
		},
		{
			Query: "/root/*:item",
			Want:  []string{"foo", "bar"},
		},
		{
			Query: "/root/ang:*",
			Want:  []string{"foo", "bar"},
		},
		{
			Query: "/root/item",
			Want:  []string{},
		},
		{
			Query:   "/root/*:item",
			Want:    []string{"foo", "bar"},
			Options: options,
		},
		{
			Query:   "/root/ang:item",
			Want:    []string{"foo", "bar"},
			Options: options,
		},
	}
	runTests(t, docSpace, tests)
}

func TestFilterPath(t *testing.T) {
	tests := []TestCase{
		{
			Query: "/root/item[1]",
			Want:  []string{"foo"},
		},
		{
			Query: "/root/item[last()]",
			Want:  []string{"bar"},
		},
		{
			Query: "/root[starts-with(normalize-space(./item[1]), 'foo')]/item",
			Want:  []string{"foo", "bar"},
		},
	}
	runTests(t, docBase, tests)
}

func TestInstanceOf(t *testing.T) {
	tests := []TestCase{
		{
			Query: "1 instance of xs:integer",
			Want:  []string{"true"},
		},
		{
			Query: "(1, 2) instance of xs:integer",
			Want:  []string{"false"},
		},
		{
			Query: "'test' instance of xs:integer",
			Want:  []string{"false"},
		},
		{
			Query: "'test' instance of xs:integer?",
			Want:  []string{"true"},
		},
		{
			Query: "'test' instance of xs:integer*",
			Want:  []string{"true"},
		},
		{
			Query: "(1, 'test') instance of xs:integer*",
			Want:  []string{"true"},
		},
		{
			Query: "'test' instance of xs:integer+",
			Want:  []string{"false"},
		},
		{
			Query: "(1, 'test') instance of xs:integer+",
			Want:  []string{"true"},
		},
		{
			Query: "(1, 2) instance of (xs:integer | xs:string)*",
			Want:  []string{"true"},
		},
		{
			Query: "(1, 2) instance of (xs:boolean | xs:string)*",
			Want:  []string{"false"},
		},
	}
	runTests(t, docBase, tests)
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
			Query: "/root/item[1], /root/group/item",
			Want:  []string{"foo", "qux"},
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
			Query: "/root/item[1] | /root/item[1]",
			Want:  []string{"foo"},
		},
		{
			Query: "/root/item[1] union /root/item[1]",
			Want:  []string{"foo"},
		},
		{
			Query: "/root/item[@lang='en'] intersect /root/item[@id='fst']",
			Want:  []string{"foo"},
		},
		{
			Query: "/root/item[1] intersect /root/group/item",
			Want:  []string{},
		},
		{
			Query: "/root/item[@lang='en'] except /root/item[@id='fst']",
			Want:  []string{"bar"},
		},
		{
			Query: "/root/group/item except /root/item",
			Want:  []string{"qux"},
		},
		{
			Query: "/root/item[@lang='en'] except /root/item",
			Want:  []string{},
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

func testBooleanFunctions(t *testing.T) {
	tests := []TestCase{
		{
			Query: "true()",
			Want:  []string{"true"},
		},
		{
			Query: "false()",
			Want:  []string{"false"},
		},
		{
			Query: "boolean(/root/item)",
			Want:  []string{"true"},
		},
		{
			Query: "boolean(/test)",
			Want:  []string{"false"},
		},
		{
			Query: "not(boolean(/root/item))",
			Want:  []string{"false"},
		},
		{
			Query: "not(boolean(/test))",
			Want:  []string{"true"},
		},
	}
	runTests(t, docBase, tests)
}

func testNodeFunctions(t *testing.T) {
	tests := []TestCase{
		{
			Query: "name(/root)",
			Want:  []string{"root"},
		},
		{
			Query: "fn:local-name(/root)",
			Want:  []string{"root"},
		},
		{
			Query: "local-name(root())",
			Want:  []string{"root"},
		},
		{
			Query: "local-name(root(/root/item))",
			Want:  []string{"root"},
		},
		{
			Query: "path(/root)",
			Want:  []string{"/"},
		},
		{
			Query: "has-children('/root')",
			Want:  []string{"true"},
		},
		{
			Query: "has-children('/root/group/item')",
			Want:  []string{"false"},
		},
	}
	runTests(t, docBase, tests)
}

func testNumberFunctions(t *testing.T) {
	tests := []TestCase{
		{
			Query: "sum(/root/item/star)",
			Want:  []string{"30"},
		},
		{
			Query: "count(/root/item)",
			Want:  []string{"2"},
		},
		{
			Query: "avg(/root/item/star)",
			Want:  []string{"15"},
		},
		{
			Query: "min(/root/item/star)",
			Want:  []string{"10"},
		},
		{
			Query: "max(/root/item/star)",
			Want:  []string{"20"},
		},
		{
			Query: "round(2.5)",
			Want:  []string{"3"},
		},
		{
			Query: "round(2.4999)",
			Want:  []string{"2"},
		},
		{
			Query: "floor(10.5)",
			Want:  []string{"10"},
		},
		{
			Query: "floor(-10.5)",
			Want:  []string{"-11"},
		},
		{
			Query: "ceiling(10.5)",
			Want:  []string{"11"},
		},
		{
			Query: "ceiling(-10.5)",
			Want:  []string{"-10"},
		},
		{
			Query: "abs(1)",
			Want:  []string{"1"},
		},
		{
			Query: "number(/root/item[1]/star)",
			Want:  []string{"10"},
		},
		{
			Query: "xs:decimal(/root/item[1]/star)",
			Want:  []string{"10"},
		},
		{
			Query: "format-integer(42, '00')",
			Want:  []string{"42"},
		},
		{
			Query: "format-integer(42, '##')",
			Want:  []string{"42"},
		},
		{
			Query: "format-integer(42, '00000')",
			Want:  []string{"00042"},
		},
		{
			Query: "format-integer(42, '#####')",
			Want:  []string{"42"},
		},
		{
			Query: "format-integer(1234, '#,###')",
			Want:  []string{"1,234"},
		},
		{
			Query: "format-integer(1234, '0.000')",
			Want:  []string{"1.234"},
		},
		{
			Query: "format-integer(1479632, '0000')",
			Want:  []string{"1479632"},
		},
		{
			Query: "format-integer(1479632, '0.000')",
			Want:  []string{"1.479.632"},
		},
	}
	runTests(t, docNumbers, tests)
}

func testStringFunctions(t *testing.T) {
	tests := []TestCase{
		{
			Query: "string('test')",
			Want:  []string{"test"},
		},
		{
			Query: "string(/root/item[1])",
			Want:  []string{"foo"},
		},
		{
			Query: "string-length(/root/item[1])",
			Want:  []string{"3"},
		},
		{
			Query: "normalize-space('  foobar  ')",
			Want:  []string{"foobar"},
		},
		{
			Query: "normalize-space(/root/item[1])",
			Want:  []string{"foo"},
		},
		{
			Query: "upper-case(/root/item[1])",
			Want:  []string{"FOO"},
		},
		{
			Query: "lower-case(/root/item[1])",
			Want:  []string{"foo"},
		},
		{
			Query: "starts-with(/root/item[1], 'f')",
			Want:  []string{"true"},
		},
		{
			Query: "starts-with(/root/item[1], 'b')",
			Want:  []string{"false"},
		},
		{
			Query: "ends-with(/root/item[1], 'oo')",
			Want:  []string{"true"},
		},
		{
			Query: "ends-with(/root/item[1], 'ar')",
			Want:  []string{"false"},
		},
		{
			Query: "compare('foo', 'foo')",
			Want:  []string{"0"},
		},
		{
			Query: "compare('foo', 'bar')",
			Want:  []string{"1"},
		},
		{
			Query: "compare('bar', 'foo')",
			Want:  []string{"-1"},
		},
		{
			Query: "concat('foo', 'bar')",
			Want:  []string{"foobar"},
		},
		{
			Query: "substring('foobar', 4)",
			Want:  []string{"bar"},
		},
		{
			Query: "substring('', 4)",
			Want:  []string{""},
		},
		{
			Query: "substring('foobar', 1, 3)",
			Want:  []string{"foo"},
		},
		{
			Query: "substring('foobar', 4, 3)",
			Want:  []string{"bar"},
		},
		{
			Query: "substring-before('foobar', 'bar')",
			Want:  []string{"foo"},
		},
		{
			Query: "substring-after('foobar', 'foo')",
			Want:  []string{"bar"},
		},
		{
			Query: "string-join(('foo', 'bar'), '-')",
			Want:  []string{"foo-bar"},
		},
		{
			Query: "contains('foobar', 'bar')",
			Want:  []string{"true"},
		},
		{
			Query: "contains('foobar', 'test')",
			Want:  []string{"false"},
		},
		{
			Query: "contains(/root/item[1], 'foo')",
			Want:  []string{"true"},
		},
		{
			Query: "contains('foobar', /root/item[1])",
			Want:  []string{"true"},
		},
		{
			Query: "contains(/root/item[1], 'bar')",
			Want:  []string{"false"},
		},
		{
			Query: "replace('foobar', 'bar', '')",
			Want:  []string{"foo"},
		},
		{
			Query: "translate('foobar', 'fobar', 'FOBAR')",
			Want:  []string{"FOOBAR"},
		},
		{
			Query: "translate('foobar', 'fbr', 'FBR')",
			Want:  []string{"FooBaR"},
		},
		{
			Query: "translate('foobar', 'FBR', 'fbr')",
			Want:  []string{"foobar"},
		},
		{
			Query: "matches('foobar', '^f.+$')",
			Want:  []string{"true"},
		},
		{
			Query: "matches('foobar', '^t.+t$')",
			Want:  []string{"false"},
		},
		{
			Query: "tokenize('foo bar')",
			Want:  []string{"foo", "bar"},
		},
		{
			Query: "tokenize('foo-bar', '-')",
			Want:  []string{"foo", "bar"},
		},
	}
	runTests(t, docBase, tests)
}

func testAngleStringFunctions(t *testing.T) {
	if fs, ok := builtinEnv.(*funcset); ok {
		fs.EnableAngle()
	}
	tests := []TestCase{
		{
			Query: "aglstr:string-reverse('foo')",
			Want:  []string{"oof"},
		},
		{
			Query: "aglstr:string-reverse(/root/item[2])",
			Want:  []string{"rab"},
		},
		{
			Query: "aglstr:string-indexof('foo', 'bar')",
			Want:  []string{"0"},
		},
		{
			Query: "aglstr:string-indexof('foo', 'foo')",
			Want:  []string{"1"},
		},
	}
	runTests(t, docBase, tests)
}

func testSequenceFunctions(t *testing.T) {
	tests := []TestCase{
		{
			Query: "reverse(/root//item)",
			Want:  []string{"qux", "bar", "foo"},
		},
		{
			Query: "empty(())",
			Want:  []string{"true"},
		},
		{
			Query: "empty(/root/test)",
			Want:  []string{"true"},
		},
		{
			Query: "empty(/root/item)",
			Want:  []string{"false"},
		},
		{
			Query: "exists(())",
			Want:  []string{"false"},
		},
		{
			Query: "exists(/root/test)",
			Want:  []string{"false"},
		},
		{
			Query: "exists(/root/item)",
			Want:  []string{"true"},
		},
		{
			Query: "head(/root/item)",
			Want:  []string{"foo"},
		},
		{
			Query: "head((1))",
			Want:  []string{"1"},
		},
		{
			Query: "tail(/root/item)",
			Want:  []string{"bar"},
		},
		{
			Query: "tail(1 to 5)",
			Want:  []string{"2", "3", "4", "5"},
		},
		{
			Query: "zero-or-one(())",
			Want:  []string{},
		},
		{
			Query: "zero-or-one(/root/group/item)",
			Want:  []string{"qux"},
		},
		{
			Query: "one-or-more(/root/item)",
			Want:  []string{"foo", "bar"},
		},
		{
			Query: "exactly-one(/root/group/item)",
			Want:  []string{"qux"},
		},
	}
	runTests(t, docBase, tests)
}

func TestFunctions(t *testing.T) {
	t.Run("boolean", testBooleanFunctions)
	t.Run("node", testNodeFunctions)
	t.Run("number", testNumberFunctions)
	t.Run("string", testStringFunctions)
	t.Run("sequence", testSequenceFunctions)
	t.Run("angle-string", testAngleStringFunctions)
	t.Run("arrows", testArrows)
}

func testArrows(t *testing.T) {
	tests := []TestCase{
		{
			Query: "/root/item[1] => upper-case()",
			Want: []string{"FOO"},
		},
		{
			Query: "'foobar' => upper-case() => replace('BAR', /root/group/item[1])",
			Want: []string{"FOOqux"},
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
		if len(c.Want) == 0 && res.Len() == 0 {
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
