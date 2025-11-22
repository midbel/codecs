package xslt

import (
	"strings"
	"testing"

	"github.com/midbel/codecs/xml"
)

const sample = `<?xml version="1.0" encoding="UTF-8"?>

<root>
	<item id="node" lang="en">foobar</item>
</root>
`

func TestMatch(t *testing.T) {
	doc, err := xml.ParseString(sample)
	if err != nil {
		t.Errorf("fail to parse sample xml document: %s", err)
		return
	}
	var (
		attr = xml.NewAttribute(xml.LocalName("id"), "node")
		root = xml.NewElement(xml.LocalName("root"))
		foo  = xml.NewElement(xml.LocalName("foo"))
		bar  = xml.NewElement(xml.LocalName("bar"))
		txt  = xml.NewText("foobar")
	)
	foo.Append(bar)
	root.Append(foo)
	tests := []struct {
		Pattern string
		Want    bool
		xml.Node
	}{
		{
			Pattern: "/",
			Want:    true,
			Node:    doc,
		},
		{
			Pattern: "root",
			Want:    true,
			Node:    doc.Root(),
		},
		{
			Pattern: "foo/bar",
			Want:    true,
			Node:    bar,
		},
		{
			Pattern: "root",
			Want:    false,
			Node:    foo,
		},
		{
			Pattern: "@id",
			Want:    true,
			Node:    &attr,
		},
		{
			Pattern: "@*",
			Want:    true,
			Node:    &attr,
		},
		{
			Pattern: "@lang",
			Want:    false,
			Node:    &attr,
		},
		{
			Pattern: "attribute()",
			Want:    true,
			Node:    &attr,
		},
		{
			Pattern: "text()",
			Want:    true,
			Node:    txt,
		},
		{
			Pattern: "text()",
			Want:    false,
			Node:    &attr,
		},
		{
			Pattern: "text()",
			Want:    false,
			Node:    doc.Root(),
		},
		{
			Pattern: "foo | bar",
			Want:    true,
			Node:    foo,
		},
		{
			Pattern: "foo | bar",
			Want:    true,
			Node:    bar,
		},
		{
			Pattern: "foo | bar",
			Want:    false,
			Node:    doc,
		},
		{
			Pattern: "*",
			Want:    true,
			Node:    doc,
		},
		{
			Pattern: "*",
			Want:    true,
			Node:    doc.Root(),
		},
		{
			Pattern: "node()",
			Want:    true,
			Node:    doc.Root(),
		},
	}
	cp := NewCompiler()
	for _, c := range tests {
		m, err := cp.Compile(strings.NewReader(c.Pattern))
		if err != nil {
			t.Errorf("%s: fail to compile pattern: %s", c.Pattern, err)
			continue
		}
		got := m.Match(c.Node)
		if c.Want != got {
			t.Errorf("%s: result mismatched!!! want %t, got %t", c.Pattern, c.Want, got)
			t.Logf("%s: matcher type: %T", c.Pattern, m)
		}
	}
}

func TestCompile(t *testing.T) {
	tests := []string{
		"*",
		".",
		"item",
		"ns:item",
		"/ns:item",
		"//ns:item",
		"root/item",
		"root//item",
		"/",
		"/root",
		"//item",
		"foo | bar",
		"/foo except /bar",
		"//foo intersect /bar",
		"/foo | //bar",
		"self::foobar",
		"self::ns:foobar",
		"@class",
		"/foo/@id",
		"@*",
		"node()",
		"text()",
		"attribute()",
		"item[1]",
		"item[\"foo\"]",
		"item[\"foo\" = 1]",
	}

	cp := NewCompiler()
	for _, str := range tests {
		_, err := cp.Compile(strings.NewReader(str))
		if err != nil {
			t.Errorf("%s: fail to compile: %s", str, err)
		}
	}
}
