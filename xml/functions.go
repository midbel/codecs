package xml

import (
	"math"
	"strings"
)

var builtins = map[string]builtinFunc{
	"number":            checkArity(1, callNumber),
	"string":            checkArity(1, callString),
	"boolean":           checkArity(1, callBoolean),
	"not":               checkArity(1, callNot),
	"true":              checkArity(0, callTrue),
	"false":             checkArity(0, callFalse),
	"concat":            nil,
	"contains":          checkArity(2, callContains),
	"string-length":     checkArity(1, callStringLen),
	"starts-with":       checkArity(2, callStartsWith),
	"ends-width":        checkArity(2, callEndsWith),
	"substring":         nil,
	"substring-after":   checkArity(2, callCutSuffix),
	"substring-before":  checkArity(2, callCutPrefix),
	"normalize-space":   checkArity(1, callTrimSpace),
	"upper-case":        checkArity(1, callUpper),
	"lower-case":        checkArity(1, callLower),
	"translate":         nil,
	"min":               nil,
	"max":               nil,
	"sum":               nil,
	"ceiling":           checkArity(1, callCeil),
	"floor":             checkArity(1, callFloor),
	"round":             checkArity(1, callRound),
	"abs":               checkArity(1, callAbs),
	"first":             nil,
	"last":              nil,
	"count":             nil,
	"id":                nil,
	"name":              checkArity(0, callName),
	"local-name":        checkArity(0, callLocalName),
	"position":          checkArity(0, callPosition),
	"text":              checkArity(0, callValue),
	"element-available": checkArity(1, callAvailable),
}

type builtinFunc func(Node, []any) (any, error)

func checkArity(minArgs int, fn builtinFunc) builtinFunc {
	do := func(node Node, args []any) (any, error) {
		if len(args) < minArgs {
			return nil, errArgument
		}
		return fn(node, args)
	}
	return do
}

func callRound(_ Node, args []any) (any, error) {
	n, ok := args[0].(float64)
	if !ok {
		return nil, errType
	}
	return math.Round(n), nil
}

func callCeil(_ Node, args []any) (any, error) {
	n, ok := args[0].(float64)
	if !ok {
		return nil, errType
	}
	return math.Ceil(n), nil
}

func callFloor(_ Node, args []any) (any, error) {
	n, ok := args[0].(float64)
	if !ok {
		return nil, errType
	}
	return math.Floor(n), nil
}

func callAbs(_ Node, args []any) (any, error) {
	n, ok := args[0].(float64)
	if !ok {
		return nil, errType
	}
	return math.Abs(n), nil
}

func callCutPrefix(_ Node, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, errType
	}
	check, ok := args[1].(string)
	if !ok {
		return nil, errType
	}
	str, _ = strings.CutPrefix(str, check)
	return str, nil
}

func callCutSuffix(_ Node, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, errType
	}
	check, ok := args[1].(string)
	if !ok {
		return nil, errType
	}
	str, _ = strings.CutSuffix(str, check)
	return str, nil
}

func callContains(_ Node, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, errType
	}
	check, ok := args[1].(string)
	if !ok {
		return nil, errType
	}
	return strings.Contains(str, check), nil
}

func callStartsWith(_ Node, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, errType
	}
	check, ok := args[1].(string)
	if !ok {
		return nil, errType
	}
	return strings.HasPrefix(str, check), nil
}

func callEndsWith(_ Node, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, errType
	}
	check, ok := args[1].(string)
	if !ok {
		return nil, errType
	}
	return strings.HasSuffix(str, check), nil
}

func callTrimSpace(_ Node, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, errType
	}
	return strings.TrimSpace(str), nil
}

func callStringLen(_ Node, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, errType
	}
	n := len(str)
	return float64(n), nil
}

func callUpper(_ Node, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, errType
	}
	return strings.ToUpper(str), nil
}

func callLower(_ Node, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, errType
	}
	return strings.ToLower(str), nil
}

func callLocalName(ctx Node, _ []any) (any, error) {
	return ctx.LocalName(), nil
}

func callName(ctx Node, _ []any) (any, error) {
	return ctx.QualifiedName(), nil
}

func callValue(ctx Node, _ []any) (any, error) {
	return ctx.Value(), nil
}

func callPosition(ctx Node, _ []any) (any, error) {
	return float64(ctx.Position()), nil
}

func callAvailable(ctx Node, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, errType
	}
	el, ok := ctx.(*Element)
	if !ok {
		return nil, errType
	}
	return el.Has(str), nil
}

func callNumber(_ Node, args []any) (any, error) {
	return toFloat(args[0])
}

func callString(_ Node, args []any) (any, error) {
	return toString(args[0])
}

func callBoolean(_ Node, args []any) (any, error) {
	return toBool(args[0]), nil
}

func callNot(_ Node, args []any) (any, error) {
	ok := toBool(args[0])
	return !ok, nil
}

func callTrue(_ Node, _ []any) (any, error) {
	return true, nil
}

func callFalse(_ Node, _ []any) (any, error) {
	return false, nil
}
