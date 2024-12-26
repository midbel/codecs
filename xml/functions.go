package xml

import (
	"fmt"
)

var builtins = map[string]builtinFunc{
	"true":    callTrue,
	"false":   callFalse,
	"boolean": callBoolean,
	"not":     callNot,
}

type builtinFunc func(Node, []Expr) ([]Item, error)

func checkArity(argCount int, fn builtinFunc) builtinFunc {
	do := func(node Node, args []Expr) ([]Item, error) {
		if len(args) < argCount {
			return nil, errArgument
		}
		return fn(node, args)
	}
	return do
}

func callBoolean(ctx Node, args []Expr) ([]Item, error) {
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		value := createLiteral(false)
		return createSingle(value), nil
	}
	if len(items) != 1 {
		return nil, fmt.Errorf("%w: too many values in sequence", errType)
	}
	ok, err := getBooleanFromItem(items[0])
	if err != nil {
		return nil, err
	}
	value := createLiteral(ok)
	return createSingle(value), nil
}

func callNot(ctx Node, args []Expr) ([]Item, error) {
	items, err := callBoolean(ctx, args)
	if err != nil {
		return nil, err
	}
	value, ok := items[0].Value().(bool)
	if !ok {
		return nil, errType
	}
	items[0] = createLiteral(!value)
	return items, nil
}

func callTrue(_ Node, _ []Expr) ([]Item, error) {
	value := createLiteral(true)
	return createSingle(value), nil
}

func callFalse(_ Node, _ []Expr) ([]Item, error) {
	value := createLiteral(false)
	return createSingle(value), nil
}

func getBooleanFromItem(item Item) (bool, error) {
	if _, ok := items.(nodeItem); ok {
		return ok, nil
	}
	var res bool
	switch value := items[0].Value().(type) {
	case string:
		res = value != ""
	case float64:
		res = value != 0
	case bool:
		res = value
	default:
		return false, errType
	}
	return res, nil
}

func expandArgs(ctx Node, args []Expr) ([]Item, error) {
	var list []Item
	for _, a := range args {
		i, err := a.Next(ctx)
		if err != nil {
			return nil, err
		}
		list = append(list, i...)
	}
	return list, nil
}
