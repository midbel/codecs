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
	return nil, errImplemented
}

func callLast(n Node, _ []Expr) (any, error) {
	return nil, errImplemented
}

func callCount(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callMin(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callMax(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callSum(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callAvg(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callRound(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callCeil(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callFloor(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callAbs(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callCutPrefix(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callCutSuffix(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callContains(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callStartsWith(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callEndsWith(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callTrimSpace(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callStringLen(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callUpper(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callLower(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callConcat(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callSubstring(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callLocalName(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callName(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callValue(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callPosition(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callAvailable(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
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

func callTrue(ctx Node, args []Expr) (any, error) {
	return true, nil
}

func callFalse(ctx Node, args []Expr) (any, error) {
	return false, nil
}
