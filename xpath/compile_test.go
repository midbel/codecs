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
	}
	for _, str := range tests {
		_, err := CompileString(str)
		if err != nil {
			t.Errorf("%s: fail to compile expression: %s", str, err)
		}
	}
}