package xpath

import (
	"testing"
)

func TestCompile(t *testing.T) {
	tests := []string{
		"/root",
		"/root/item",
		"//item",
		"/root/item[1]",
		"array{1, 2, 3}",
		"array{1, 2, array{1, 2, 3}}",
		"[1, 2, 3]",
		"[1, 2, [1, 2]]",
		"map{key: value}",
		"map{'foo': 10, 'bar' : 20}",
		"map{'foo': 10, 'nest': map{'bar': 20}}",
		"map{'foo': 10, 'nest': array{1, 2}}",
		"map{'foo': 10, 'nest': [1, 2]}",
		"[1, 2, [1, 2]](0)",
		"[1, 2, [1, 2]](2)(0)",
		"let $arr1 := array{1, 2, 3} return $arr1(0)",
		"let $arr2 := [1, 2, 3] return $arr2(0)",
		"let $arr3 := map{'name': 'foobar', 'answer': 42} return $arr3('answer')",
		"let $arr4 := map{'name': 'foobar', 'answer': 42}, $key := 'name' return $arr4($key)",
		"let $arr5 := map{'name': 'foobar', 'foobar': map{'answer': 42}} return $arr5('foobar')('answer')",
	}
	for _, str := range tests {
		_, err := CompileString(str)
		if err != nil {
			t.Errorf("%s: fail to compile expression: %s", str, err)
		}
	}
}
