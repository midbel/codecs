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
	<lines>
		<line>
			<quantity>1</quantity>
			<total>10</total>
			<unit>
				<price>10</price>
			</unit>
		</line>
		<line>
			<quantity>5</quantity>
			<total>25</total>
			<unit>
				<price>5</price>
			</unit>
		</line>
	</lines>
</root>
`

type TestCase struct {
	Expr      string
	Expected  []string
	Composite bool
	Failed    bool
}

func TestArray(t *testing.T) {
	tests := []TestCase{
		{
			Expr:      "[1, 2, 3, 'test']",
			Expected:  []string{"1", "2", "3", "test"},
			Composite: true,
		},
		{
			Expr:      "array{1, 2, 3}",
			Expected:  []string{"1", "2", "3"},
			Composite: true,
		},
		{
			Expr:     "let $arr := array{1, 2, 3} return $arr(1)",
			Expected: []string{"1"},
		},
		{
			Expr:      "[[1, 2, 3], [4, 5, 6]](1)",
			Expected:  []string{"1", "2", "3"},
			Composite: true,
		},
		{
			Expr:     "[[1, 2, 3], [4, 5, 6]](1)(2)",
			Expected: []string{"2"},
		},
		{
			Expr:     "array{1, 2, 3}(79)",
			Expected: []string{},
			Failed:   true,
		},
	}
	runTests(t, tests)
}

func TestIf(t *testing.T) {
	tests := []TestCase{
		{
			Expr:     "if (/root/item[1] = 'element-1') then 'ok' else 'nok'",
			Expected: []string{"ok"},
		},
		{
			Expr:     "if (/root/item[1] = 'test') then 'ok' else 'nok'",
			Expected: []string{"nok"},
		},
	}
	runTests(t, tests)
}

func TestFor(t *testing.T) {
	tests := []TestCase{
		{
			Expr:     "for $i in 1 to 5 return $i",
			Expected: []string{"1", "2", "3", "4", "5"},
		},
	}
	runTests(t, tests)
}

func TestLet(t *testing.T) {
	tests := []TestCase{
		{
			Expr:     "let $x := -1 return $x",
			Expected: []string{"-1"},
		},
		{
			Expr:     "let $x := 1, $y := 1 return $x+$y",
			Expected: []string{"2"},
		},
	}
	runTests(t, tests)
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
	runTests(t, tests)
}

func TestIndex(t *testing.T) {
	tests := []TestCase{
		{
			Expr:     "/root/item[1]",
			Expected: []string{"element-1"},
		},
	}
	runTests(t, tests)
}

func TestFunctions(t *testing.T) {
	t.Run("boolean", testBooleanFunctions)
	t.Run("string", testStringFunctions)
	t.Run("misc", testMiscFunctions)
}

func testStringFunctions(t *testing.T) {
	tests := []TestCase{
		{
			Expr:     "concat('foo', 'bar')",
			Expected: []string{"foobar"},
		},
		{
			Expr:     "concat(/root/item[1], /root/item[2])",
			Expected: []string{"element-1element-2"},
		},
		{
			Expr:     "contains(/root/item[1], 'element')",
			Expected: []string{"true"},
		},
		{
			Expr:     "contains(/root/item[1], 'test')",
			Expected: []string{"false"},
		},
		{
			Expr:     "string(/root/item)",
			Expected: []string{"element-1"},
		},
		{
			Expr:     "string(10)",
			Expected: []string{"10"},
		},
		{
			Expr:     "string(true())",
			Expected: []string{"true"},
		},
		{
			Expr:     "string('')",
			Expected: []string{""},
		},
		{
			Expr:     "string-join(('foo', 'bar'), ' ')",
			Expected: []string{"foo bar"},
		},
		{
			Expr:     "string-join(1 to 3)",
			Expected: []string{"123"},
		},
		{
			Expr:     "string-join(1 to 3, ':')",
			Expected: []string{"1:2:3"},
		},
		{
			Expr:     "substring('foobar', 4)",
			Expected: []string{"bar"},
		},
		{
			Expr:     "substring('foobar', 1, 3)",
			Expected: []string{"foo"},
		},
		{
			Expr:     "string-length('foobar')",
			Expected: []string{"6"},
		},
		{
			Expr:     "upper-case('foobar')",
			Expected: []string{"FOOBAR"},
		},
		{
			Expr:     "upper-case('FOOBAR')",
			Expected: []string{"FOOBAR"},
		},
		{
			Expr:     "lower-case('foobar')",
			Expected: []string{"foobar"},
		},
		{
			Expr:     "lower-case('FOOBAR')",
			Expected: []string{"foobar"},
		},
	}
	runTests(t, tests)
}

func testBooleanFunctions(t *testing.T) {
	tests := []TestCase{
		{
			Expr:     "false()",
			Expected: []string{"false"},
		},
		{
			Expr:     "true()",
			Expected: []string{"true"},
		},
	}
	runTests(t, tests)
}

func testMiscFunctions(t *testing.T) {
	tests := []TestCase{
		{
			Expr:     "count(//item))",
			Expected: []string{"4"},
		},
	}
	runTests(t, tests)
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
	runTests(t, tests)
}

func testAttributes(t *testing.T) {
	tests := []TestCase{
		{
			Expr:     "//@ignore",
			Expected: []string{"true"},
		},
		{
			Expr:     "//attribute::ignore",
			Expected: []string{"true"},
		},
	}
	runTests(t, tests)
}

func testCombinations(t *testing.T) {
	tests := []TestCase{
		{
			Expr:     "/root/item[1] | /root/item[2]",
			Expected: []string{"element-1", "element-2"},
		},
		{
			Expr:     "/root/item union /root//group/item",
			Expected: []string{"element-1", "element-2", "sub-element-1", "sub-element-2"},
		},
		{
			Expr:     "/root//item[contains(., 'sub')] intersect /root/group/item",
			Expected: []string{"sub-element-1", "sub-element-2"},
		},
		{
			Expr:     "/root//item except /root/group/item",
			Expected: []string{"element-1", "element-2"},
		},
	}
	runTests(t, tests)
}

func testAxis(t *testing.T) {
	tests := []TestCase{
		{
			Expr:     "/root/item[1]/following-sibling::item",
			Expected: []string{"element-2"},
		},
		{
			Expr:     "/root/item[2]/preceding-sibling::item",
			Expected: []string{"element-1"},
		},
	}
	runTests(t, tests)
}

func testPaths(t *testing.T) {
	tests := []TestCase{
		{
			Expr:     "/root/item",
			Expected: []string{"element-1", "element-2"},
		},
		{
			Expr:     "/root/item[2]/../item[1]",
			Expected: []string{"element-1"},
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
			Expr:     "/root/item[1], /root/item[2]",
			Expected: []string{"element-1", "element-2"},
		},
	}
	runTests(t, tests)
}

func TestPath(t *testing.T) {
	t.Run("base", testPaths)
	t.Run("axis", testAxis)
	t.Run("combinations", testCombinations)
	t.Run("attributes", testAttributes)
}

func TestQuantified(t *testing.T) {
	tests := []TestCase{
		{
			Expr:     "every $x in (1, 2, 3) satisfies $x <= 10",
			Expected: []string{"true"},
		},
		{
			Expr:     "every $x in (1, 2, 3) satisfies $x > 10",
			Expected: []string{"false"},
		},
		{
			Expr:     "some $x in (1, 2, 3) satisfies $x > 10",
			Expected: []string{"false"},
		},
		{
			Expr:     "some $x in (1, 2, 13) satisfies $x > 10",
			Expected: []string{"true"},
		},
		{
			Expr:     "some $x in (1, 2, 13), $y in (1, 2) satisfies $x * $y > 10",
			Expected: []string{"true"},
		},
		{
			Expr:     "every $el in //item satisfies contains(string($el), 'element')",
			Expected: []string{"true"},
		},
		{
			Expr:     "some $el in //* satisfies exists($el/@ignore)",
			Expected: []string{"true"},
		},
		{
			Expr:     "every $el in /root/items satisfies 1=1",
			Expected: []string{"true"},
		},
		{
			Expr:     "some $el in /root/items satisfies 1=1",
			Expected: []string{"false"},
		},
		{
			Expr:     "some $x in (1, 2), $y in () satisfies $x + $y > 0",
			Expected: []string{"false"},
		},
		{
			Expr:     "every $x in (1, 2), $y in (3, 4) satisfies $x < $y",
			Expected: []string{"true"},
		},
	}
	runTests(t, tests)
}

func TestOperators(t *testing.T) {
	tests := []TestCase{
		{
			Expr:     "'foo'||'bar'",
			Expected: []string{"foobar"},
		},
		{
			Expr:     "/root/item[1] is /root/item[1]",
			Expected: []string{"true"},
		},
		{
			Expr:     "/root/item[1] is /root/item[2]",
			Expected: []string{"false"},
		},
		{
			Expr:     "/root/item[2] is /root/item[1]",
			Expected: []string{"false"},
		},
		{
			Expr:     "/root/item[1] >> /root/item[2]",
			Expected: []string{"false"},
		},
		{
			Expr:     "/root/item[1] << /root/item[2]",
			Expected: []string{"true"},
		},
		{
			Expr:     "/root/item[2] >> /root/item[1]",
			Expected: []string{"true"},
		},
		{
			Expr:     "/root/item[2] << /root/item[1]",
			Expected: []string{"false"},
		},
	}
	runTests(t, tests)
}

func TestMath(t *testing.T) {
	tests := []TestCase{
		{
			Expr:     "sum(/root/lines/line/total)",
			Expected: []string{"35"},
		},
		{
			Expr:     "/root/lines/line/(quantity * unit/price)",
			Expected: []string{"10", "25"},
		},
		{
			Expr:     "/root/lines/line/quantity * /root/lines/line/unit/price",
			Expected: []string{"10", "5", "50", "25"},
		},
	}
	runTests(t, tests)
}

func runTests(t *testing.T, tests []TestCase) {
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
		if c.Failed {
			if err == nil {
				t.Errorf("test pass but expected to fail")
			}
			continue
		}
		if c.Composite {
			checkArrayValues(t, seq, c)
			continue
		}
		if seq.Len() != len(c.Expected) {
			t.Logf("result: %s (%d vs %d)", seq.CanonicalizeString(), seq.Len(), len(c.Expected))
			t.Errorf("%s: number of nodes mismatched! want %d, got %d", c.Expr, len(c.Expected), seq.Len())
			continue
		}
		if got, ok := compareValues(seq, c.Expected); !ok {
			t.Errorf("%s: nodes mismatched! want %s, got %s", c.Expr, c.Expected, got)
		}
	}
}

func checkArrayValues(t *testing.T, seq Sequence, c TestCase) {
	if seq.Len() != 1 {
		t.Errorf("sequence does not contains number of expected element (1)")
		return
	}
	items, ok := seq[0].Value().([]any)
	if !ok {
		t.Errorf("expected array of item but got something else (%T)", seq[0].Value())
		return
	}
	var res Sequence
	for j := range items {
		res.Append(NewLiteralItem(items[j]))
	}
	if res.Len() != len(c.Expected) {
		t.Logf("result: %s (%d vs %d)", res.CanonicalizeString(), res.Len(), len(c.Expected))
		t.Errorf("%s: number of nodes mismatched! want %d, got %d", c.Expr, len(c.Expected), res.Len())
		return
	}
	if got, ok := compareValues(res, c.Expected); !ok {
		t.Errorf("%s: nodes mismatched! want %s, got %s", c.Expr, c.Expected, got)
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
