package xpath

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/midbel/codecs/xml"
)

func Debug(expr Expr) string {
	var str strings.Builder
	debugExpr(&str, expr)
	return str.String()
}

func debugExpr(w io.Writer, expr Expr) {
	switch v := expr.(type) {
	case *Query:
		debugExpr(w, v.expr)
	case query:
		debugExpr(w, v.expr)
	case root:
		io.WriteString(w, "root")
	case current:
		io.WriteString(w, "current")
	case wildcard:
		io.WriteString(w, "wildcard")
	case name:
		io.WriteString(w, "name")
		io.WriteString(w, "(")
		if v.Space != "" {
			io.WriteString(w, v.Space)
			io.WriteString(w, "(")
			io.WriteString(w, v.Uri)
			io.WriteString(w, ")")
			io.WriteString(w, ":")
		}
		io.WriteString(w, v.Name)
		io.WriteString(w, ")")
	case kind:
		io.WriteString(w, "kind")
		io.WriteString(w, "(")
		io.WriteString(w, v.kind.String())
		io.WriteString(w, "()")
		io.WriteString(w, ")")
	case axis:
		io.WriteString(w, "axis")
		io.WriteString(w, "(")
		io.WriteString(w, v.kind)
		io.WriteString(w, ", ")
		debugExpr(w, v.next)
		io.WriteString(w, ")")
	case step:
		io.WriteString(w, "step")
		io.WriteString(w, "(")
		debugExpr(w, v.curr)
		io.WriteString(w, ", ")
		debugExpr(w, v.next)
		io.WriteString(w, ")")
	case stepmap:
		io.WriteString(w, "map")
		io.WriteString(w, "(")
		debugExpr(w, v.step)
		io.WriteString(w, ", ")
		debugExpr(w, v.expr)
		io.WriteString(w, ")")
	case union:
		io.WriteString(w, "union")
		io.WriteString(w, "(")
		for i := range v.all {
			if i > 0 {
				io.WriteString(w, ", ")
			}
			debugExpr(w, v.all[i])
		}
		io.WriteString(w, ")")
	case except:
		io.WriteString(w, "except")
		io.WriteString(w, "(")
		for i := range v.all {
			if i > 0 {
				io.WriteString(w, ", ")
			}
			debugExpr(w, v.all[i])
		}
		io.WriteString(w, ")")
	case intersect:
		io.WriteString(w, "intersect")
		io.WriteString(w, "(")
		for i := range v.all {
			if i > 0 {
				io.WriteString(w, ", ")
			}
			debugExpr(w, v.all[i])
		}
		io.WriteString(w, ")")
	case quantified:
		if v.every {
			io.WriteString(w, "every")
		} else {
			io.WriteString(w, "some")
		}
		io.WriteString(w, "(")
		for i, b := range v.binds {
			if i > 0 {
				io.WriteString(w, ", ")
			}
			io.WriteString(w, "(")
			io.WriteString(w, b.ident)
			io.WriteString(w, ", ")
			debugExpr(w, b.expr)
			io.WriteString(w, ")")
		}
		io.WriteString(w, ", ")
		io.WriteString(w, "satisfies")
		io.WriteString(w, "(")
		debugExpr(w, v.test)
		io.WriteString(w, ")")
		io.WriteString(w, ")")
	case attr:
		io.WriteString(w, "attribute")
		io.WriteString(w, "(")
		io.WriteString(w, v.ident)
		io.WriteString(w, ")")
	case filter:
		io.WriteString(w, "filter")
		io.WriteString(w, "(")
		debugExpr(w, v.expr)
		io.WriteString(w, ", ")
		debugExpr(w, v.check)
		io.WriteString(w, ")")
	case identity:
		io.WriteString(w, "identity")
		io.WriteString(w, "(")
		debugExpr(w, v.left)
		io.WriteString(w, ", ")
		debugExpr(w, v.right)
		io.WriteString(w, ")")
	case binary:
		io.WriteString(w, "binary")
		io.WriteString(w, "(")
		debugExpr(w, v.left)
		io.WriteString(w, ", ")
		debugExpr(w, v.right)
		io.WriteString(w, ", ")
		io.WriteString(w, debugOp(v.op))
		io.WriteString(w, ")")
	case rng:
		io.WriteString(w, "binary")
		io.WriteString(w, "(")
		debugExpr(w, v.left)
		io.WriteString(w, ", ")
		debugExpr(w, v.right)
		io.WriteString(w, ")")
	case reverse:
		io.WriteString(w, "reverse")
		io.WriteString(w, "(")
		debugExpr(w, v.expr)
		io.WriteString(w, ")")
	case sequence:
		io.WriteString(w, "sequence")
		io.WriteString(w, "(")
		for i := range v.all {
			if i > 0 {
				io.WriteString(w, ", ")
			}
			debugExpr(w, v.all[i])
		}
		io.WriteString(w, ")")
	case value:
		io.WriteString(w, "value")
		io.WriteString(w, "(")
		io.WriteString(w, v.seq.CanonicalizeString())
		io.WriteString(w, ")")
	case identifier:
		io.WriteString(w, "identifier")
		io.WriteString(w, "(")
		io.WriteString(w, v.ident)
		io.WriteString(w, ")")
	case literal:
		io.WriteString(w, "literal")
		io.WriteString(w, "(")
		io.WriteString(w, v.expr)
		io.WriteString(w, ")")
	case number:
		io.WriteString(w, "number")
		io.WriteString(w, "(")
		io.WriteString(w, strconv.FormatFloat(v.expr, 'f', -1, 64))
		io.WriteString(w, ")")
	case call:
		io.WriteString(w, "call")
		io.WriteString(w, "(")
		debugName(w, v.QName)
		for i := range v.args {
			io.WriteString(w, ", ")
			debugExpr(w, v.args[i])
		}
		io.WriteString(w, ")")
	default:
		io.WriteString(w, "unknown")
		io.WriteString(w, "(")
		io.WriteString(w, fmt.Sprintf("%T", v))
		io.WriteString(w, ")")
	}
}

func debugName(w io.Writer, qn xml.QName) {
	if qn.Space != "" {
		io.WriteString(w, qn.Space)
		io.WriteString(w, ":")
	}
	io.WriteString(w, qn.Name)
}

func debugOp(op rune) string {
	switch op {
	case opAssign:
		return "assign"
	case opArrow:
		return "arrow"
	case opRange:
		return "range"
	case opConcat:
		return "concat"
	case opBefore:
		return "before"
	case opAfter:
		return "after"
	case opAdd:
		return "add"
	case opSub:
		return "subtract"
	case opMul:
		return "multiply"
	case opDiv:
		return "divide"
	case opMod:
		return "modulo"
	case opEq:
		return "eq"
	case opNe:
		return "ne"
	case opGt:
		return "gt"
	case opGe:
		return "ge"
	case opLt:
		return "lt"
	case opLe:
		return "le"
	case opAnd:
		return "and"
	case opOr:
		return "or"
	default:
		return ""
	}
}
