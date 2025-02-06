package casing_test

import (
	"testing"

	"github.com/midbel/codecs/casing"
)

func TestCasing(t *testing.T) {
	data := []struct{
		Input string
		Want string
		Case casing.CaseType
	}{
		{
			Input: "foobar",
			Want: "foobar",
			Case: casing.SnakeCase,
		},
		{
			Input: "foobar",
			Want: "FOOBAR",
			Case: casing.UpperSnakeCase,
		},
		{
			Input: "fooBar",
			Want: "foo-bar",
			Case: casing.KebabCase,
		},
		{
			Input: "fooBar",
			Want: "FOO-BAR",
			Case: casing.UpperKebabCase,
		},
		{
			Input: "fooBar",
			Want: "foo_bar",
			Case: casing.SnakeCase,
		},
		{
			Input: "fooBar",
			Want: "FOO_BAR",
			Case: casing.UpperSnakeCase,
		},
		{
			Input: "fooBAR",
			Want: "foo_bar",
			Case: casing.SnakeCase,
		},
		{
			Input: "fooBAR",
			Want: "foo-bar",
			Case: casing.KebabCase,
		},
		{
			Input: "foo___-___BAR",
			Want: "foo-bar",
			Case: casing.KebabCase,
		},
		{
			Input: "  ---foo_bar---  ",
			Want: "foo-bar",
			Case: casing.KebabCase,
		},
	}
	for _, d := range data {
		got := casing.To(d.Case, d.Input)
		if got != d.Want {
			t.Errorf("result mismatched! want %s, got %s", d.Want, got)
		}
	}
}