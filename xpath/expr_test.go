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
	xmlns:ang="http://midbel.org/ang">
	<ang:item id="fst" lang="en">foo</ang:item>
	<ang:item id="snd" lang="en">bar</ang:item>
</root>
`

const docInvoice = `<?xml version="1.0" encoding="utf-8"?>
<in:Invoice xmlns:in="http://invoice.org/invoice" xmlns:cbc="http://invoice.org/commons" xmlns:cac="http://invoice.org/aggregate">
	<cbc:IssueDate>2025-11-05</cbc:IssueDate>
	<cbc:InvoiceType>380</cbc:InvoiceType>
	<cac:Total>
		<cbc:Total currencyID="eur">40</cbc:Total>
	</cac:Total>
	<cac:Line>
		<cbc:Quantity>3</cbc:Quantity>
		<cbc:Total currencyID="eur">15</cbc:Total>
		<cac:Item>
			<cbc:ID>foo</cbc:ID>
		</cac:Item>
	</cac:Line>
	<cac:Line>
		<cbc:Quantity>5</cbc:Quantity>
		<cbc:Total currencyID="eur">25</cbc:Total>
		<cac:Item>
			<cbc:ID>bar</cbc:ID>
		</cac:Item>
	</cac:Line>
</in:Invoice>
`

type TestCase struct {
	Query string
	Want  []string
}

type ContextTestCase struct {
	Context string
	Rooted  bool
	Query   string
	Want    []string
}

func TestWithContext(t *testing.T) {
	tests := []ContextTestCase{
		{
			Context: "cac:Line",
			Rooted:  true,
			Query:   "not(normalize-space(cac:Total))",
			Want:    []string{"true"},
		},
		{
			Context: "/in:Invoice/cac:Line",
			Rooted:  true,
			Query:   "(cbc:Quantity != 0)",
			Want:    []string{"true"},
		},
	}
	xmlSpaces := []xml.NS{
		{
			Prefix: "in",
			Uri:    "http://invoice.org/invoice",
		},
		{
			Prefix: "cbc",
			Uri:    "http://invoice.org/commons",
		},
		{
			Prefix: "cac",
			Uri:    "http://invoice.org/aggregate",
		},
	}
	runTestsWithContext(t, docInvoice, tests, xmlSpaces)
}

func TestGroupingAndLogic(t *testing.T) {
	tests := []TestCase{
		{
			Query: "/in:Invoice/cbc:InvoiceType[self::cbc:InvoiceType and contains(., '380')]",
			Want:  []string{"380"},
		},
		{
			Query: "/in:Invoice/cbc:InvoiceType[not(contains(normalize-space(.), ' '))]",
			Want:  []string{"380"},
		},
		{
			Query: "/in:Invoice/cbc:InvoiceType[not(contains(normalize-space(.), ' ')) and contains(concat('_', ., '_'), '_380_')]",
			Want:  []string{"380"},
		},
		{
			Query: "/in:Invoice/cbc:InvoiceType[(not(contains(normalize-space(.), ' ')) and contains(concat('_', ., '_'), '_380_'))]",
			Want:  []string{"380"},
		},
		{
			Query: "/in:Invoice/cbc:InvoiceType[self::cbc:InvoiceType and (not(contains(normalize-space(.), ' ')) and contains('_380_240_80_', concat('_', ., '_')))]",
			Want:  []string{"380"},
		},
		{
			Query: "/in:Invoice/cac:Total/cbc:Total * 10 * 10 div 100 = sum(/in:Invoice/cac:Line/cbc:Total * 10 * 10 div 100)",
			Want:  []string{"true"},
		},
	}
	xmlSpaces := []xml.NS{
		{
			Prefix: "in",
			Uri:    "http://invoice.org/invoice",
		},
		{
			Prefix: "cbc",
			Uri:    "http://invoice.org/commons",
		},
		{
			Prefix: "cac",
			Uri:    "http://invoice.org/aggregate",
		},
	}
	runTestsNS(t, docInvoice, tests, xmlSpaces)
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
	spaces := []xml.NS{
		{
			Prefix: "",
			Uri:    "http://midbel.org/ns",
		},
		{
			Prefix: "ang",
			Uri:    "http://midbel.org/angle",
		},
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
			Query: "/root/*:item",
			Want:  []string{"foo", "bar"},
		},
		{
			Query: "/root/ang:item",
			Want:  []string{"foo", "bar"},
		},
	}
	runTestsNS(t, docSpace, tests, spaces)
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

func TestVariables(t *testing.T) {
	tests := []TestCase{
		{
			Query: "/root/item[@id=$id]",
			Want:  []string{"foo"},
		},
		{
			Query: "/root/item[@id=$foo and lang=$lang]",
			Want:  []string{"foo"},
		},
		{
			Query: "/root/item[@id=$foo or lang!=$lang]",
			Want:  []string{"foo", "bar"},
		},
	}

	root, err := xml.ParseString(docBase)
	if err != nil {
		t.Errorf("fail to parse xml document: %s", err)
		return
	}

	eval := NewEvaluator()
	eval.Define("id", "fst")
	eval.Define("lang", "en")
	for _, c := range tests {
		q, err := eval.Create(c.Query)
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

func TestPath(t *testing.T) {
	t.Run("basic", testPathBasic)
	t.Run("combine", testPathCombine)
	t.Run("filter", testPathFilter)
	t.Run("axis", testPathAxis)
	t.Run("type", testPathType)
}

func testPathAxis(t *testing.T) {
	tests := []TestCase{
		{
			Query: "/root/child::item",
			Want:  []string{"foo", "bar"},
		},
		{
			Query: "/root/item[2]/self::item",
			Want:  []string{"bar"},
		},
		{
			Query: "/root/descendant::item",
			Want:  []string{"foo", "bar", "qux"},
		},
		{
			Query: "/root/item[1]/descendant-or-self::node()",
			Want:  []string{"foo", "foo"},
		},
		{
			Query: "/root/descendant-or-self::node()",
			Want:  []string{"foobarqux", "foo", "foo", "bar", "bar", "qux", "qux", "qux"},
		},
		{
			Query: "local-name(/root/group/item/parent::*)",
			Want:  []string{"group"},
		},
		{
			Query: "/root/item[@id='fst']/following-sibling::item",
			Want:  []string{"bar"},
		},
		{
			Query: "/root/item[@id='snd']/preceding-sibling::item",
			Want:  []string{"foo"},
		},
		{
			Query: "/root/item[@id='fst']/attribute::lang",
			Want:  []string{"en"},
		},
		{
			Query: "/root/item[@id='fst']/following::item",
			Want:  []string{"bar", "qux"},
		},
		{
			Query: "/root/group/item/preceding::item",
			Want:  []string{"bar", "foo"},
		},
	}
	runTests(t, docBase, tests)
}

func testPathType(t *testing.T) {
	tests := []TestCase{
		{
			Query: "/root/item[1]/attribute(id)",
			Want:  []string{"fst"},
		},
		{
			Query: "/root/item[1]/attribute(*)",
			Want:  []string{"fst", "en"},
		},
		{
			Query: "/root/element()",
			Want:  []string{"foo", "bar", "qux"},
		},
		{
			Query: "/root/element(*)",
			Want:  []string{"foo", "bar", "qux"},
		},
		{
			Query: "/root/element(item)",
			Want:  []string{"foo", "bar"},
		},
		{
			Query: "/root/group/node()",
			Want:  []string{"qux"},
		},
		{
			Query: "//comment()",
			Want:  []string{},
		},
		{
			Query: "//text()",
			Want:  []string{"foo", "bar", "qux"},
		},
	}
	runTests(t, docBase, tests)
}

func testPathBasic(t *testing.T) {
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

func testPathCombine(t *testing.T) {
	tests := []TestCase{
		{
			Query: "/root/item[1], /root/group/item",
			Want:  []string{"foo", "qux"},
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
			Query: "/root/(item|group/item)",
			Want:  []string{"foo", "bar", "qux"},
		},
	}
	runTests(t, docBase, tests)
}

func testPathFilter(t *testing.T) {
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
			Query: "agl:string-reverse('foo')",
			Want:  []string{"oof"},
		},
		{
			Query: "agl:string-reverse(/root/item[2])",
			Want:  []string{"rab"},
		},
		{
			Query: "agl:string-indexof('foo', 'bar')",
			Want:  []string{"0"},
		},
		{
			Query: "agl:string-indexof('foo', 'foo')",
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
			Want:  []string{"FOO"},
		},
		{
			Query: "'foobar' => upper-case() => replace('BAR', /root/group/item[1])",
			Want:  []string{"FOOqux"},
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
	eval := NewEvaluator()
	for _, c := range tests {
		q, err := eval.Create(c.Query)
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

func runTestsWithContext(t *testing.T, doc string, tests []ContextTestCase, spaces []xml.NS) {
	t.Helper()

	root, err := xml.ParseString(doc)
	if err != nil {
		t.Errorf("fail to parse xml document: %s", err)
		return
	}
	for _, c := range tests {
		eval := NewEvaluator()
		for _, n := range spaces {
			eval.RegisterNS(n.Prefix, n.Uri)
		}
		q, err := eval.Create(c.Context)
		if err != nil {
			t.Errorf("fail to build xpath query: %s", err)
			continue
		}
		if c.Rooted {
			q = FromRoot(q)
		}
		seq, err := q.Find(root)
		if err != nil {
			t.Errorf("error finding node in document: %s", err)
			continue
		}
		for i := range seq {
			res, err := eval.Find(c.Query, seq[i].Node())
			if err != nil {
				t.Errorf("error finding node in context: %s", err)
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
}

func runTestsNS(t *testing.T, doc string, tests []TestCase, spaces []xml.NS) {
	t.Helper()

	root, err := xml.ParseString(doc)
	if err != nil {
		t.Errorf("fail to parse xml document: %s", err)
		return
	}
	eval := NewEvaluator()
	for _, n := range spaces {
		eval.RegisterNS(n.Prefix, n.Uri)
	}
	for _, c := range tests {
		q, err := eval.Create(c.Query)
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

func runTests(t *testing.T, doc string, tests []TestCase) {
	t.Helper()

	root, err := xml.ParseString(doc)
	if err != nil {
		t.Errorf("fail to parse xml document: %s", err)
		return
	}
	eval := NewEvaluator()
	for _, c := range tests {
		q, err := eval.Create(c.Query)
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
