package xml

import (
	"math"
	"strconv"
	"strings"
)

var builtins = map[string]builtinFunc{
	"number":            checkArity(1, callNumber),
	"string":            checkArity(1, callString),
	"boolean":           checkArity(1, callBoolean),
	"not":               checkArity(1, callNot),
	"true":              checkArity(0, callTrue),
	"false":             checkArity(0, callFalse),
	"concat":            checkArity(1, callConcat),
	"contains":          checkArity(2, callContains),
	"string-length":     checkArity(1, callStringLen),
	"starts-with":       checkArity(2, callStartsWith),
	"ends-width":        checkArity(2, callEndsWith),
	"substring":         checkArity(2, callSubstring),
	"substring-after":   checkArity(2, callCutSuffix),
	"substring-before":  checkArity(2, callCutPrefix),
	"normalize-space":   checkArity(1, callTrimSpace),
	"upper-case":        checkArity(1, callUpper),
	"lower-case":        checkArity(1, callLower),
	"translate":         nil,
	"min":               checkArity(1, callMin),
	"max":               checkArity(1, callMax),
	"sum":               checkArity(1, callSum),
	"avg":               checkArity(1, callAvg),
	"ceiling":           checkArity(1, callCeil),
	"floor":             checkArity(1, callFloor),
	"round":             checkArity(1, callRound),
	"abs":               checkArity(1, callAbs),
	"first":             checkArity(0, callFirst),
	"last":              checkArity(0, callLast),
	"count":             checkArity(0, callCount),
	"id":                nil,
	"name":              checkArity(0, callName),
	"local-name":        checkArity(0, callLocalName),
	"position":          checkArity(0, callPosition),
	"text":              checkArity(0, callValue),
	"element-available": checkArity(1, callAvailable),
}

type builtinFunc func(Node, []Expr) (any, error)

func checkArity(minArgs int, fn builtinFunc) builtinFunc {
	do := func(node Node, args []Expr) (any, error) {
		if len(args) < minArgs {
			return nil, errArgument
		}
		return fn(node, args)
	}
	return do
}

func callFirst(n Node, _ []Expr) (any, error) {
	return float64(0), nil
}

func callLast(n Node, _ []Expr) (any, error) {
	p := n.Parent()
	if p == nil {
		return 1.0, nil
	}
	x, ok := p.(*Element)
	if !ok {
		return 1.0, nil
	}
	return float64(len(x.Nodes)), nil
}

func callCount(ctx Node, args []Expr) (any, error) {
	return 0.0, nil
}

func callMin(ctx Node, args []Expr) (any, error) {
	return nil, nil
}

func callMax(ctx Node, args []Expr) (any, error) {
	return nil, nil
}

func callSum(ctx Node, args []Expr) (any, error) {
	return nil, nil
}

func callAvg(ctx Node, args []Expr) (any, error) {
	return nil, nil
}

func callRound(ctx Node, args []Expr) (any, error) {
	n, ok := args[0].(float64)
	if !ok {
		return nil, errType
	}
	return math.Round(n), nil
}

func callCeil(ctx Node, args []Expr) (any, error) {
	n, ok := args[0].(float64)
	if !ok {
		return nil, errType
	}
	return math.Ceil(n), nil
}

func callFloor(ctx Node, args []Expr) (any, error) {
	n, ok := args[0].(float64)
	if !ok {
		return nil, errType
	}
	return math.Floor(n), nil
}

func callAbs(ctx Node, args []Expr) (any, error) {
	n, ok := args[0].(float64)
	if !ok {
		return nil, errType
	}
	return math.Abs(n), nil
}

func callCutPrefix(ctx Node, args []Expr) (any, error) {
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

func callCutSuffix(ctx Node, args []Expr) (any, error) {
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

func callContains(ctx Node, args []Expr) (any, error) {
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

func callStartsWith(ctx Node, args []Expr) (any, error) {
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

func callEndsWith(ctx Node, args []Expr) (any, error) {
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

func callTrimSpace(ctx Node, args []Expr) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, errType
	}
	return strings.TrimSpace(str), nil
}

func callStringLen(ctx Node, args []Expr) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, errType
	}
	n := len(str)
	return float64(n), nil
}

func callUpper(ctx Node, args []Expr) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, errType
	}
	return strings.ToUpper(str), nil
}

func callLower(ctx Node, args []Expr) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, errType
	}
	return strings.ToLower(str), nil
}

func callConcat(ctx Node, args []Expr) (any, error) {
	var str []string
	for i := range args {
		s, err := toString(args[i])
		if err != nil {
			return nil, err
		}
		str = append(str, s)
	}
	return strings.Join(str, ""), nil
}

func callSubstring(ctx Node, args []Expr) (any, error) {
	str, err := toString(args[0])
	if err != nil {
		return nil, err
	}
	pos, err := toFloat(args[1])
	if err != nil {
		return nil, err
	}
	var size float64
	if len(args) >= 2 {
		size, err = toFloat(args[2])
		if err != nil {
			return nil, err
		}
	}
	if size == 0 {
		size = float64(len(str)) - pos
	}
	return str[int(pos):int(pos+size)], nil
}

func callLocalName(ctx Node, _ []Expr) (any, error) {
	return ctx.LocalName(), nil
}

func callName(ctx Node, _ []Expr) (any, error) {
	return ctx.QualifiedName(), nil
}

func callValue(ctx Node, _ []Expr) (any, error) {
	return ctx.Value(), nil
}

func callPosition(ctx Node, _ []Expr) (any, error) {
	return float64(ctx.Position()), nil
}

func callAvailable(ctx Node, args []Expr) (any, error) {
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

func callNumber(ctx Node, args []Expr) (any, error) {
	return toFloat(args[0])
}

func callString(ctx Node, args []Expr) (any, error) {
	return toString(args[0])
}

func callBoolean(ctx Node, args []Expr) (any, error) {
	return toBool(args[0]), nil
}

func callNot(ctx Node, args []Expr) (any, error) {
	ok := toBool(args[0])
	return !ok, nil
}

func callTrue(ctx Node, _ []Expr) (any, error) {
	return true, nil
}

func callFalse(ctx Node, _ []Expr) (any, error) {
	return false, nil
}
