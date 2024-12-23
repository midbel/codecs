package xml

import (
	"strconv"
)

var builtins = map[string]builtinFunc{
	"name":          checkArity(0, callName),
	"local-name":    checkArity(0, callLocalName),
	"namespace-uri": checkArity(0, callNamespaceUri),
	"lang":          checkArity(1, callLang),
	"is-same-node":  checkArity(2, callSameNode),
	"node-before":   checkArity(2, callNodeBefore),
	"node-after":    checkArity(2, callNodeAfter),
	"root":          checkArity(0, callNodeRoot),
	"number":        checkArity(0, callNumber),
	"count":         checkArity(0, callCount),
	"avg":           checkArity(0, callAverage),
	"min":           checkArity(0, callMin),
	"max":           checkArity(0, callMax),
	"sum":           checkArity(0, callSum),
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

func getArgOrContext(ctx Node, args []Expr) any {
	if len(args) == 0 {
		return ctx
	}
	return args[0]
}

func getName(value any, fn func(Node) string) (string, error) {
	if n, ok := value.(Node); ok {
		return fn(n), nil
	}
	return toString(value)
}

func callName(ctx Node, args []Expr) (any, error) {
	param := getArgOrContext(ctx, args)
	switch n := param.(type) {
	case Node:
		return n.QualifiedName(), nil
	case []any:
		if len(n) == 0 {
			return "", nil
		}
		var list []string
		for i := range n {
			str, err := getName(n[i], func(n Node) string {
				return n.QualifiedName()
			})
			if err != nil {
				return nil, err
			}
			list = append(list, str)
		}
		return list, nil
	default:
		return nil, errType
	}
}

func callLocalName(ctx Node, args []Expr) (any, error) {
	param := getArgOrContext(ctx, args)
	switch n := param.(type) {
	case Node:
		return n.LocalName(), nil
	case []any:
		if len(n) == 0 {
			return "", nil
		}
		var list []string
		for i := range n {
			str, err := getName(n[i], func(n Node) string {
				return n.LocalName()
			})
			if err != nil {
				return nil, err
			}
			list = append(list, str)
		}
		return list, nil
	default:
		return nil, errType
	}
}

func callNamespaceUri(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callLang(ctx Node, args []Expr) (any, error) {
	return nil, nil
}

func callSameNode(ctx Node, args []Expr) (any, error) {
	return nil, nil
}

func callNodeBefore(ctx Node, args []Expr) (any, error) {
	return nil, nil
}

func callNodeAfter(ctx Node, args []Expr) (any, error) {
	return nil, nil
}

func callNodeRoot(ctx Node, args []Expr) (any, error) {
	el, ok := getArgOrContext(ctx, args).(*Element)
	if !ok {
		return nil, errType
	}
	var get func(Node) Node

	get = func(n Node) Node {
		if n.Root() {
			return n
		}
		return get(n.Parent())
	}
	return get(el), nil
}

func callNumber(ctx Node, args []Expr) (any, error) {
	param := getArgOrContext(ctx, args)
	switch n := param.(type) {
	case Node:
		v, err := strconv.ParseFloat(n.Value(), 64)
		if err != nil {
			err = errType
		}
		return v, err
	default:
		return nil, errType
	}
}

func callCount(ctx Node, args []Expr) (any, error) {
	return nil, nil
}

func callAverage(ctx Node, args []Expr) (any, error) {
	return nil, nil
}

func callMax(ctx Node, args []Expr) (any, error) {
	return nil, nil
}

func callMin(ctx Node, args []Expr) (any, error) {
	return nil, nil
}

func callSum(ctx Node, args []Expr) (any, error) {
	return nil, nil
}
