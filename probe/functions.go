package probe

import (
	"errors"
	"fmt"
)

var errArgument = errors.New("invalid number of argument given")

func invalidArgs(msg string, n int) error {
	return fmt.Errorf("%w: %s - %d given", errArgument, msg, n)
}

var builtins = map[string]func(any, []Expr) (any, error){
	"as":       runAs,
	"len":      runLen,
	"at":       runAt,
	"first":    runFirst,
	"last":     runLast,
	"default":  runDefault,
	"not":      runNot,
	"eq":       runEqual,
	"ne":       runNotEqual,
	"lt":       runLesserThan,
	"le":       runLesserEq,
	"gt":       runGreaterThan,
	"ge":       runGreaterEq,
	"between":  runBetween,
	"in":       runIn,
	"ifeq":     runIfEqual,
	"ifne":     runIfNotEqual,
	"ifexists": runIfExists,
	"exists":   runExists,
	"empty":    runEmpty,
	"null":     runNull,
}

// :as()
func runAs(val any, args []Expr) (any, error) {
	if len(args) != 1 {
		return nil, invalidArgs("as takes only one argument", len(args))
	}
	target, err := getStrFromExpr(args[0])
	if err != nil {
		return nil, err
	}
	switch target {
	case "string":
		val, err = castToString(val)
	case "number":
		val, err = castToNumber(val)
	case "bool":
		val, err = castToBool(val)
	default:
		return nil, fmt.Errorf("%s: unknown target type", target)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: value can not be converted to target type %s", ErrType, target)
	}
	return val, nil
}

// :len, :length
func runLen(val any, args []Expr) (any, error) {
	if len(args) != 0 {
		return nil, invalidArgs("length takes not argument(s)", len(args))
	}
	var x int
	switch arr := val.(type) {
	case []any:
		x = len(arr)
	case map[string]any:
		x = len(arr)
	default:
		return nil, compositeExpected("length")
	}
	return float64(x), nil
}

// :at()
func runAt(val any, args []Expr) (any, error) {
	if len(args) != 1 {
		return nil, invalidArgs("at takes only one argument", len(args))
	}
	arr, ok := val.([]any)
	if !ok {
		return nil, arrayExpected("at")
	}
	ix, err := getIntFromExpr(args[0])
	if err != nil {
		return nil, err
	}
	if ix < 0 || ix >= len(arr) {
		return nil, nil // nil, errIndex
	}
	return arr[ix], nil
}

// :first()
func runFirst(val any, args []Expr) (any, error) {
	if len(args) != 0 {
		return nil, invalidArgs("first takes not argument(s)", len(args))
	}
	arr, ok := val.([]any)
	if !ok {
		return nil, arrayExpected("first")
	}
	if len(arr) == 0 {
		return nil, nil
	}
	return arr[0], nil
}

// :last()
func runLast(val any, args []Expr) (any, error) {
	if len(args) != 0 {
		return nil, invalidArgs("last takes not argument(s)", len(args))
	}
	arr, ok := val.([]any)
	if !ok {
		return nil, arrayExpected("last")
	}
	if len(arr) == 0 {
		return nil, nil
	}
	return arr[len(arr)-1], nil
}

// :default()
func runDefault(val any, args []Expr) (any, error) {
	if len(args) != 1 {
		return nil, invalidArgs("default takes only one argument", len(args))
	}
	if isDefined(val) {
		return val, nil
	}
	return getAnyFromExpr(args[0])
}

// :not()
func runNot(val any, args []Expr) (any, error) {
	if len(args) != 1 {
		return nil, invalidArgs("not takes no argument(s)", len(args))
	}
	return !isDefined(val), nil
}

// :eq()
func runEqual(val any, args []Expr) (any, error) {
	if len(args) != 1 {
		return nil, invalidArgs("eq takes only one argument", len(args))
	}
	ok := isEqual(val, args[0])
	if !ok {
		return ok, errDiscard
	}
	return ok, nil
}

// :ne()
func runNotEqual(val any, args []Expr) (any, error) {
	if len(args) != 1 {
		return nil, invalidArgs("ne takes only one argument", len(args))
	}
	ok := !isEqual(val, args[0])
	if !ok {
		return ok, errDiscard
	}
	return ok, nil
}

// :lt
func runLesserThan(val any, args []Expr) (any, error) {
	if len(args) != 1 {
		return nil, invalidArgs("lt takes only one argument", len(args))
	}
	ok := isLess(val, args[0])
	if !ok {
		return ok, errDiscard
	}
	return ok, nil
}

// :le
func runLesserEq(val any, args []Expr) (any, error) {
	if len(args) != 1 {
		return nil, invalidArgs("le takes only one argument", len(args))
	}
	ok := isLess(val, args[0]) || isEqual(val, args[0])
	if !ok {
		return ok, errDiscard
	}
	return ok, nil
}

// :gt
func runGreaterThan(val any, args []Expr) (any, error) {
	if len(args) != 1 {
		return nil, invalidArgs("gt takes only one argument", len(args))
	}
	ok := !isLess(val, args[0]) && !isEqual(val, args[0])
	if !ok {
		return ok, errDiscard
	}
	return ok, nil
}

// :ge
func runGreaterEq(val any, args []Expr) (any, error) {
	if len(args) != 1 {
		return nil, invalidArgs("ge takes only one argument", len(args))
	}
	ok := !isLess(val, args[0]) || isEqual(val, args[0])
	if !ok {
		return ok, errDiscard
	}
	return ok, nil
}

// :between
func runBetween(val any, args []Expr) (any, error) {
	if len(args) != 2 {
		return nil, invalidArgs("between takes two arguments", len(args))
	}
	if isEqual(val, args[0]) || isEqual(val, args[1]) {
		return true, nil
	}
	if isLess(val, args[1]) || !isLess(val, args[0]) {
		return true, nil
	}
	return false, errDiscard
}

// :in
func runIn(val any, args []Expr) (any, error) {
	if len(args) == 0 {
		return nil, invalidArgs("in takes at least one argument", len(args))
	}
	for i := range args {
		if isEqual(val, args[i]) {
			return true, nil
		}
	}
	return false, errDiscard
}

// :ifeq
func runIfEqual(val any, args []Expr) (any, error) {
	if len(args) != 3 {
		return nil, invalidArgs("ifeq takes exactly three arguments", len(args))
	}
	if isEqual(val, args[0]) {
		return args[1], nil
	}
	return args[2], nil
}

// :ifne
func runIfNotEqual(val any, args []Expr) (any, error) {
	if len(args) != 3 {
		return nil, invalidArgs("ifne takes exactly three arguments", len(args))
	}
	if !isEqual(val, args[0]) {
		return args[1], nil
	}
	return args[2], nil
}

// :ifexists
func runIfExists(val any, args []Expr) (any, error) {
	if len(args) < 2 {
		return nil, invalidArgs("ifexists takes at least two arguments", len(args))
	}
	if len(args) == 2 {
		if isDefined(val) {
			return args[0], nil
		}
		return args[1], nil
	}
	var ok bool
	switch arr := val.(type) {
	case []any:
		ix, err := getIntFromExpr(args[0])
		if err != nil {
			return nil, err
		}
		ok = ix >= 0 && ix <= len(arr)
	case map[string]any:
		key, err := getStrFromExpr(args[0])
		if err != nil {
			return nil, err
		}
		_, ok = arr[key]
	default:
		return nil, compositeExpected("ifexists")
	}
	if ok {
		return args[1], nil
	}
	return args[2], nil
}

// :exists
func runExists(val any, args []Expr) (any, error) {
	if len(args) == 0 {
		return isDefined(val), nil
	}
	if len(args) != 1 {
		return nil, invalidArgs("exists takes zero or one argument(s)", len(args))
	}
	switch arr := val.(type) {
	case []any:
		ix, err := getIntFromExpr(args[0])
		if err != nil {
			return nil, err
		}
		return ix >= 0 && ix <= len(arr), nil
	case map[string]any:
		key, err := getStrFromExpr(args[0])
		if err != nil {
			return nil, err
		}
		_, ok := arr[key]
		return ok, nil
	default:
		return nil, compositeExpected("exists")
	}
}

// :empty
func runEmpty(val any, args []Expr) (any, error) {
	if len(args) != 0 {
		return nil, invalidArgs("empty takes no argument(s)", len(args))
	}
	switch arr := val.(type) {
	case []any:
		return len(arr) == 0, nil
	case map[string]any:
		return len(arr) == 0, nil
	case nil:
		return true, nil
	default:
		return nil, compositeExpected("empty")
	}
}

// :null
func runNull(val any, args []Expr) (any, error) {
	if len(args) != 0 {
		return nil, invalidArgs("null takes not argument(s)", len(args))
	}
	return val == nil, nil
}
