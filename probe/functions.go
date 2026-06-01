package probe

import (
	"errors"
	"fmt"
)

var errArgument = errors.New("invalid number of argument given")

func invalidArgs(msg string, n int) error {
	return fmt.Errorf("%w: %s - %d given", errArgument, msg, n)
}

// :as()
func cast(val any, args []Expr) (any, error) {
	return val, nil
}

// :len, :length
func length(val any, args []Expr) (any, error) {
	if len(args) != 0 {
		return nil, invalidArgs("length takes not argument(s)", len(args))
	}
	switch arr := val.(type) {
	case []any:
		return len(arr), nil
	case map[string]any:
		return len(arr), nil
	default:
		return nil, compositeExpected("length")
	}
}

// :at()
func at(val any, args []Expr) (any, error) {
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
func first(val any, args []Expr) (any, error) {
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
func last(val any, args []Expr) (any, error) {
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
func fallback(val any, args []Expr) (any, error) {
	if len(args) != 1 {
		return nil, invalidArgs("default takes only one argument", len(args))
	}
	if isDefined(val) {
		return val, nil
	}
	return getAnyFromExpr(args[0])
}

// :not()
func not(val any, args []Expr) (any, error) {
	if len(args) != 1 {
		return nil, invalidArgs("not takes no argument(s)", len(args))
	}
	return !isDefined(val), nil
}

// :eq()
func equal(val any, args []Expr) (any, error) {
	if len(args) != 1 {
		return nil, invalidArgs("eq takes only one argument", len(args))
	}
	return isEqual(val, args[0]), nil
}

// :ne()
func notEqual(val any, args []Expr) (any, error) {
	if len(args) != 1 {
		return nil, invalidArgs("ne takes only one argument", len(args))
	}
	return !isEqual(val, args[0]), nil
}

// :lt
func lesserThan(val any, args []Expr) (any, error) {
	if len(args) != 1 {
		return nil, invalidArgs("lt takes only one argument", len(args))
	}
	return isLess(val, args[0]), nil
}

// :le
func lesserEq(val any, args []Expr) (any, error) {
	if len(args) != 1 {
		return nil, invalidArgs("le takes only one argument", len(args))
	}
	return isLess(val, args[0]) || isEqual(val, args[0]), nil
}

// :gt
func greaterThan(val any, args []Expr) (any, error) {
	if len(args) != 1 {
		return nil, invalidArgs("gt takes only one argument", len(args))
	}
	return !isLess(val, args[0]) && !isEqual(val, args[0]), nil
}

// :ge
func greaterEq(val any, args []Expr) (any, error) {
	if len(args) != 1 {
		return nil, invalidArgs("ge takes only one argument", len(args))
	}
	return !isLess(val, args[0]) || isEqual(val, args[0]), nil
}

// :between
func between(val any, args []Expr) (any, error) {
	if len(args) != 2 {
		return nil, invalidArgs("between takes two arguments", len(args))
	}
	if isEqual(val, args[0]) || isEqual(val, args[1]) {
		return true, nil
	}
	if isLess(val, args[1]) || !isLess(val, args[0]) {
		return true, nil
	}
	return false, nil
}

// :in
func in(val any, args []Expr) (any, error) {
	if len(args) == 0 {
		return nil, invalidArgs("in takes at least one argument", len(args))
	}
	for i := range args {
		if isEqual(val, args[i]) {
			return true, nil
		}
	}
	return false, nil
}

// :ifeq
func ifEqual(val any, args []Expr) (any, error) {
	if len(args) != 3 {
		return nil, invalidArgs("ifeq takes exactly three arguments", len(args))
	}
	if isEqual(val, args[0]) {
		return args[1], nil
	}
	return args[2], nil
}

// :ifne
func ifNotEqual(val any, args []Expr) (any, error) {
	if len(args) != 3 {
		return nil, invalidArgs("ifne takes exactly three arguments", len(args))
	}
	if !isEqual(val, args[0]) {
		return args[1], nil
	}
	return args[2], nil
}

// :ifexists
func ifExists(val any, args []Expr) (any, error) {
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
		return nil, compositeExpected("exists")
	}
	if ok {
		return args[1], nil
	}
	return args[2], nil
}

// :exists
func exists(val any, args []Expr) (any, error) {
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
func empty(val any, args []Expr) (any, error) {
	if len(args) != 0 {
		return nil, invalidArgs("empty takes no argument(s)", len(args))
	}
	switch arr := val.(type) {
	case []any:
		return len(arr) == 0, nil
	case map[string]any:
		return len(arr) == 0, nil
	default:
		return nil, compositeExpected("empty")
	}
}

// :null
func null(val any, args []Expr) (any, error) {
	if len(args) != 0 {
		return nil, invalidArgs("null takes not argument(s)", len(args))
	}
	return val == nil, nil
}
