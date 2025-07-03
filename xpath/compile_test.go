package xpath

import (
	"testing"
)

func TestCompileError(t *testing.T) {
	t.Run("expressions", testErrorExpr)
	t.Run("array", testErrorArray)
	t.Run("if", testErrorIf)
	t.Run("let", testErrorLet)
	t.Run("some_every", testErrorQuantified)
	t.Run("instanceof_castas_castableas", testErrorCastInstanceOf)
}

func testErrorQuantified(t *testing.T) {
	tests := []string{
		"some x in (1, 2, 3)",
		"every x in (1, 2, 3)",
		"some $x in (1, 2), $x in (1, 2) satisfies",
	}
	compileExpr(t, tests)
}

func testErrorExpr(t *testing.T) {
	tests := []string{
		"10 20",
	}
	compileExpr(t, tests)
}

func testErrorCastInstanceOf(t *testing.T) {
	tests := []string{
		"$x instance xs:integer",
		"$x castable xs:integer",
		"$x instance of /proc",
		"$x castable as $var",
		"$x cast as 10",
		"/proc instance of x:",
		"/proc instance of x:$x",
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
		"if (10 > 20) then true()",
	}
	compileExpr(t, tests)
}

func testErrorArray(t *testing.T) {
	tests := []string{
		"[1, 2, 3,]",
		"array{1, 2, 3,}",
		"array[1]",
		"array[if $x < 10 then true() else false()]",
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
