package xpath

import (
	"testing"
)

func TestCompileError(t *testing.T) {
	t.Run("expressions", testErrorExpr)
	t.Run("array", testErrorArray)
	t.Run("if", testErrorIf)
	t.Run("let", testErrorLet)
}

func testErrorExpr(t *testing.T) {
	tests := []string{
		"$x instance xs:integer",
		"$x castable xs:integer",
		"10 20",
	}
	compileExpr(t, tests)
}

func testErrorLet(t *testing.T) {
	tests := []string{
		"let $x = 10; return $x",
		"let x = 10; return $x",
		"let $x = 10 return $x",
	}
	compileExpr(t, tests)
}

func testErrorIf(t *testing.T) {
	tests := []string{
		"if (10 < 20) (true()) else (false())",
		"if 10 > 20 then true() else false()",
	}
	compileExpr(t, tests)
}

func testErrorArray(t *testing.T) {
	tests := []string{
		"[1, 2, 3,]",
		"array{1, 2, 3,}",
		"array[1]",
	}
	compileExpr(t, tests)
}

func compileExpr(t *testing.T, tests []string) {
	t.Helper()
	for _, expr := range tests {
		_, err := CompileString(expr)
		if err == nil {
			t.Errorf("%q: expected error compiling but succeed", expr)
			continue
		}
		syntaxErr, ok := err.(SyntaxError)
		if !ok {
			t.Errorf("expected error of type SyntaxError but got %T", err)
			continue
		}
		if syntaxErr.Code != CodeInvalidSyntax {
			t.Errorf("want error code to be %s, but got %s", CodeInvalidSyntax, syntaxErr.Code)
		}
	}
}
