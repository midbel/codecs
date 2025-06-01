package xpath

import (
	"fmt"

	"github.com/midbel/codecs/xml"
)

func Eval(expr Expr, node xml.Node) (Sequence, error) {
	return nil, nil
}

func EvalWithContext(expr Expr, ctx *Context) (Sequence, error) {
	return eval(expr, ctx)
}

func eval(expr Expr, ctx *Context) (Sequence, error) {
	switch e := expr.(type) {
	case wildcard:
		return evalWildcard(e, ctx)
	case root:
		return evalRoot(e, ctx)
	case current:
		return evalCurrent(e, ctx)
	case step:
		return evalStep(e, ctx)
	case axis:
		return evalAxis(e, ctx)
	case identifier:
		return evalIdentifier(e, ctx)
	case name:
		return evalName(e, ctx)
	case sequence:
		return evalSequence(e, ctx)
	case arrow:
		return evalArrow(e, ctx)
	case binary:
		return evalBinary(e, ctx)
	case identity:
		return evalIdentity(e, ctx)
	case reverse:
		return evalReverse(e, ctx)
	case literal:
		return evalLiteral(e, ctx)
	case number:
		return evalNumber(e, ctx)
	case kind:
		return evalKind(e, ctx)
	case call:
		return evalCall(e, ctx)
	case attr:
		return evalAttr(e, ctx)
	case except:
		return evalExcept(e, ctx)
	case intersect:
		return evalIntersect(e, ctx)
	case union:
		return evalUnion(e, ctx)
	case filter:
		return evalFilter(e, ctx)
	case let:
		return evalLet(e, ctx)
	case rng:
		return evalRange(e, ctx)
	case loop:
		return evalLoop(e, ctx)
	case conditional:
		return evalConditional(e, ctx)
	case quantified:
		return evalQuantified(e, ctx)
	case value:
		return evalValue(e, ctx)
	case cast:
		return evalCast(e, ctx)
	case castable:
		return evalCastable(e, ctx)
	default:
		return nil, fmt.Errorf("unsupported expression type")
	}
}

func evalWildcard(e wildcard, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalRoot(e root, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalCurrent(e current, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalStep(e step, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalAxis(e axis, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalIdentifier(e identifier, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalName(e name, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalSequence(e sequence, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalArrow(e arrow, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalBinary(e binary, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalIdentity(e identity, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalReverse(e reverse, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalLiteral(e literal, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalNumber(e number, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalKind(e kind, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalCall(e call, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalAttr(e attr, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalExcept(e except, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalIntersect(e intersect, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalUnion(e union, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalFilter(e filter, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalLet(e let, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalRange(e rng, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalLoop(e loop, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalConditional(e conditional, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalQuantified(e quantified, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalValue(e value, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalCast(e cast, ctx *Context) (Sequence, error) {
	return nil, nil
}

func evalCastable(e castable, ctx *Context) (Sequence, error) {
	return nil, nil
}
