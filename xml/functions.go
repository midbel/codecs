package xml

import (
	"strconv"
	"strings"
)

var builtins = map[string]builtinFunc{
	"name":             checkArity(0, callName),
	"local-name":       checkArity(0, callLocalName),
	"namespace-uri":    checkArity(0, callNamespaceUri),
	"lang":             checkArity(1, callLang),
	"is-same-node":     checkArity(2, callSameNode),
	"node-before":      checkArity(2, callNodeBefore),
	"node-after":       checkArity(2, callNodeAfter),
	"root":             checkArity(0, callNodeRoot),
	"number":           checkArity(0, callNumber),
	"count":            checkArity(0, callCount),
	"avg":              checkArity(0, callAverage),
	"min":              checkArity(0, callMin),
	"max":              checkArity(0, callMax),
	"sum":              checkArity(0, callSum),
	"compare":          checkArity(2, callCompare),
	"concat":           checkArity(0, callConcat),
	"string-join":      checkArity(1, callStringJoin),
	"substring":        checkArity(1, callSubstring),
	"string-length":    checkArity(0, callStringLength),
	"normalize-space":  checkArity(0, callNormalizeSpace),
	"upper-case":       checkArity(0, callUpperCase),
	"lower-case":       checkArity(0, callLowerCase),
	"translate":        checkArity(0, callTranslate),
	"contains":         checkArity(1, callContains),
	"starts-with":      checkArity(1, callStartsWith),
	"ends-with":        checkArity(1, callEndsWith),
	"substring-before": checkArity(0, callSubstringBefore),
	"substring-after":  checkArity(0, callSubstringAfter),
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

func callName(ctx Node, args []Expr) (any, error) {
	param := getArgOrContext(ctx, args)
	switch n := param.(type) {
	case Node:
		return n.QualifiedName(), nil
	case sequence:
		if len(n.all) == 0 {
			return "", nil
		}
		return nil, nil
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
	case sequence:
		if len(n.all) == 0 {
			return "", nil
		}
		return nil, nil
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
	return nil, errImplemented
}

func callSameNode(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callNodeBefore(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callNodeAfter(ctx Node, args []Expr) (any, error) {
	return nil, errImplemented
}

func callNodeRoot(ctx Node, args []Expr) (any, error) {
	el, ok := getArgOrContext(ctx, args).(*Element)
	if !ok {
		return nil, errType
	}
	var get func(Node) Node

	get = func(n Node) Node {
		p := n.Parent()
		if p == nil {
			return n
		}
		return get(p)
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
	if len(args) == 0 {
		return 0.0, nil
	}
	switch n := args[0].(type) {
	case sequence:
		return float64(len(n.all)), nil
	default:
		return nil, errType
	}
}

func callAverage(ctx Node, args []Expr) (any, error) {
	if len(args) == 0 {
		return 0.0, nil
	}
	switch n := args[0].(type) {
	case sequence:
		_ = n
		return 0, nil
	default:
		return nil, errType
	}
}

func callMax(ctx Node, args []Expr) (any, error) {
	if len(args) == 0 {
		return 0.0, nil
	}
	switch n := args[0].(type) {
	case sequence:
		_ = n
		return 0, nil
	default:
		return nil, errType
	}
}

func callMin(ctx Node, args []Expr) (any, error) {
	if len(args) == 0 {
		return 0.0, nil
	}
	switch n := args[0].(type) {
	case sequence:
		_ = n
		return 0, nil
	default:
		return nil, errType
	}
}

func callSum(ctx Node, args []Expr) (any, error) {
	if len(args) == 0 {
		return 0.0, nil
	}
	switch n := args[0].(type) {
	case sequence:
		_ = n
		return 0, nil
	default:
		return nil, errType
	}
}

func callCompare(ctx Node, args []Expr) (any, error) {
	fst, err := getStringFromNode(args[0], ctx)
	if err != nil {
		return "", err
	}
	snd, err := getStringFromNode(args[1], ctx)
	if err != nil {
		return "", err
	}
	cmp := strings.Compare(fst, snd)
	return float64(cmp), nil
}

func callConcat(ctx Node, args []Expr) (any, error) {
	var list []string
	for i := range args {
		str, err := getStringFromNode(args[i], ctx)
		if err != nil {
			return "", err
		}
		list = append(list, str)
	}
	return strings.Join(list, ""), nil
}

func callStringJoin(ctx Node, args []Expr) (any, error) {
	var (
		list []string
		sep  string
		err  error
	)
	seq, ok := args[0].(sequence)
	if !ok {
		return "", errType
	}
	for i := range seq.all {
		str, err := getStringFromNode(seq.all[i], ctx)
		if err != nil {
			return "", err
		}
		list = append(list, str)
	}
	if len(args) >= 2 {
		sep, err = getStringFromNode(args[1], ctx)
		if err != nil {
			return "", err
		}
	}
	return strings.Join(list, sep), nil
}

func callSubstring(ctx Node, args []Expr) (any, error) {
	return nil, nil
}

func callStringLength(ctx Node, args []Expr) (any, error) {
	param := getArgOrContext(ctx, args)
	switch n := param.(type) {
	case Node:
		return float64(len(n.QualifiedName())), nil
	case sequence:
		if len(n.all) == 0 {
			return float64(0), nil
		}
		return float64(0), nil
	default:
		return 0, errType
	}
}

func callNormalizeSpace(ctx Node, args []Expr) (any, error) {
	var (
		param = getArgOrContext(ctx, args)
		str   string
	)
	switch n := param.(type) {
	case Node:
		str = n.QualifiedName()
	case sequence:
		if len(n.all) == 0 {
			return "", nil
		}
	default:
		return "", errType
	}
	str = strings.TrimSpace(str)
	return strings.ReplaceAll(str, "", ""), nil
}

func callUpperCase(ctx Node, args []Expr) (any, error) {
	var (
		param = getArgOrContext(ctx, args)
		str   string
	)
	switch n := param.(type) {
	case Node:
		str = n.QualifiedName()
	case sequence:
		if len(n.all) == 0 {
			return "", nil
		}
	default:
		return "", errType
	}
	return strings.ToUpper(str), nil
}

func callLowerCase(ctx Node, args []Expr) (any, error) {
	var (
		param = getArgOrContext(ctx, args)
		str   string
	)
	switch n := param.(type) {
	case Node:
		str = n.QualifiedName()
	case sequence:
		if len(n.all) == 0 {
			return "", nil
		}
	default:
		return "", errType
	}
	return strings.ToLower(str), nil
}

func callTranslate(ctx Node, args []Expr) (any, error) {
	return nil, nil
}

func callContains(ctx Node, args []Expr) (any, error) {
	var (
		str    string
		needle string
		err    error
	)
	str, err = getStringFromNode(args[0], ctx)
	if err != nil {
		return false, err
	}
	needle, err = getStringFromNode(args[1], ctx)
	if err != nil {
		return false, err
	}
	if str == "" {
		return false, nil
	}
	if needle == "" {
		return true, nil
	}
	return strings.Contains(str, needle), nil
}

func callStartsWith(ctx Node, args []Expr) (any, error) {
	var (
		str    string
		needle string
		err    error
	)
	str, err = getStringFromNode(args[0], ctx)
	if err != nil {
		return false, err
	}
	needle, err = getStringFromNode(args[1], ctx)
	if err != nil {
		return false, err
	}
	if str == "" {
		return false, nil
	}
	if needle == "" {
		return true, nil
	}
	return strings.HasPrefix(str, needle), nil
}

func callEndsWith(ctx Node, args []Expr) (any, error) {
	var (
		str    string
		needle string
		err    error
	)
	str, err = getStringFromNode(args[0], ctx)
	if err != nil {
		return false, err
	}
	needle, err = getStringFromNode(args[1], ctx)
	if err != nil {
		return false, err
	}
	if str == "" {
		return false, nil
	}
	if needle == "" {
		return true, nil
	}
	return strings.HasSuffix(str, needle), nil
}

func callSubstringBefore(ctx Node, args []Expr) (any, error) {
	var (
		str    string
		needle string
		err    error
	)
	str, err = getStringFromNode(args[0], ctx)
	if err != nil {
		return false, err
	}
	needle, err = getStringFromNode(args[1], ctx)
	if err != nil {
		return false, err
	}
	if str == "" {
		return false, nil
	}
	if needle == "" {
		return true, nil
	}
	str, _ = strings.CutPrefix(str, needle)
	return str, nil
}

func callSubstringAfter(ctx Node, args []Expr) (any, error) {
	var (
		str    string
		needle string
		err    error
	)
	str, err = getStringFromNode(args[0], ctx)
	if err != nil {
		return false, err
	}
	needle, err = getStringFromNode(args[1], ctx)
	if err != nil {
		return false, err
	}
	if str == "" {
		return false, nil
	}
	if needle == "" {
		return true, nil
	}
	str, _ = strings.CutSuffix(str, needle)
	return str, nil
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

func getStringFromNode(expr Expr, ctx Node) (string, error) {
	v, err := evalExpr(expr, ctx)
	if err != nil {
		return "", err
	}
	str, ok := v.(string)
	if !ok {
		return "", errType
	}
	return str, nil
}
