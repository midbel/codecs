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
		"let $arr := array{1, 2, 3} return $arr(0)",
		"let $arr := [1, 2, 3] return $arr(0)",
		"let $arr := map{'name': 'foobar', 'answer': 42} return $arr('answer')",
		"let $arr := map{'name': 'foobar', 'answer': 42}, $key := 'name' return $arr($key)",
		"let $arr := map{'name': 'foobar', 'foobar': map{'answer': 42}} return $arr('foobar')('answer')",
	}
	for _, str := range tests {
		_, err := CompileString(str)
		if err != nil {
			t.Errorf("%s: fail to compile expression: %s", str, err)
		}
	}
}
