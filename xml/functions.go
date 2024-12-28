package xml

import (
	"strings"
)

var builtins = map[string]builtinFunc{
	"true":             callTrue,
	"false":            callFalse,
	"boolean":          callBoolean,
	"not":              callNot,
	"name":             callName,
	"local-name":       callLocalName,
	"root":             callRoot,
	"path":             callPath,
	"has-children":     callHasChildren,
	"innermost":        callInnermost,
	"outermost":        callOutermost,
	"compare":          callCompare,
	"concat":           callConcat,
	"string-join":      callStringJoin,
	"substring":        callSubstring,
	"string-length":    callStringLength,
	"normalize-space":  callNormalizeSpace,
	"upper-case":       callUppercase,
	"lower-case":       callLowercase,
	"translate":        callTranslate,
	"contains":         callContains,
	"starts-with":      callStartsWith,
	"ends-with":        callEndsWith,
	"substring-before": callSubstringBefore,
	"substring-after":  callSubstringAfter,
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

func callCompare(ctx Node, args []Expr) ([]Item, error) {
	if len(args) != 2 {
		return nil, errArgument
	}
	fst, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	snd, err := getStringFromExpr(args[1], ctx)
	if err != nil {
		return nil, err
	}
	cmp := strings.Compare(fst, snd)
	return singleValue(float64(cmp)), nil
}
func callConcat(ctx Node, args []Expr) ([]Item, error) {
	return nil, errImplemented
}

func callStringJoin(ctx Node, args []Expr) ([]Item, error) {
	return nil, errImplemented
}

func callSubstring(ctx Node, args []Expr) ([]Item, error) {
	return nil, errImplemented
}

func callStringLength(ctx Node, args []Expr) ([]Item, error) {
	if len(args) == 0 {
		str := ctx.Value()
		return singleValue(float64(len(str))), nil
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return singleValue(0.0), nil
	}
	v, ok := items[0].(literalItem)
	if !ok {
		return nil, errType
	}
	str, ok := v.Value().(string)
	if !ok {
		return nil, errType
	}
	return singleValue(float64(len(str))), nil
}

func callNormalizeSpace(ctx Node, args []Expr) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	var prev rune
	clear := func(r rune) rune {
		if r == ' ' && r == prev {
			return -1
		}
		prev = r
		return r
	}

	str, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	str = strings.TrimSpace(str)
	return singleValue(strings.Map(clear, str)), nil
}

func callUppercase(ctx Node, args []Expr) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	str, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	return singleValue(strings.ToUpper(str)), nil
}

func callLowercase(ctx Node, args []Expr) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	str, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	return singleValue(strings.ToLower(str)), nil
}

func callTranslate(ctx Node, args []Expr) ([]Item, error) {
	return nil, errImplemented
}

func callContains(ctx Node, args []Expr) ([]Item, error) {
	if len(args) != 2 {
		return nil, errArgument
	}
	fst, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	snd, err := getStringFromExpr(args[1], ctx)
	if err != nil {
		return nil, err
	}
	res := strings.Contains(fst, snd)
	return singleValue(res), nil
}

func callStartsWith(ctx Node, args []Expr) ([]Item, error) {
	if len(args) != 2 {
		return nil, errArgument
	}
	fst, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	snd, err := getStringFromExpr(args[1], ctx)
	if err != nil {
		return nil, err
	}
	if snd == "" {
		return callTrue(ctx, args)
	}
	if fst == "" && snd != "" {
		return callFalse(ctx, args)
	}
	res := strings.HasPrefix(fst, snd)
	return singleValue(res), nil
}

func callEndsWith(ctx Node, args []Expr) ([]Item, error) {
	if len(args) != 2 {
		return nil, errArgument
	}
	fst, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	snd, err := getStringFromExpr(args[1], ctx)
	if err != nil {
		return nil, err
	}
	if snd == "" {
		return callTrue(ctx, args)
	}
	if fst == "" && snd != "" {
		return callFalse(ctx, args)
	}
	res := strings.HasSuffix(fst, snd)
	return singleValue(res), nil
}

func callSubstringBefore(ctx Node, args []Expr) ([]Item, error) {
	if len(args) != 2 {
		return nil, errArgument
	}
	fst, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	snd, err := getStringFromExpr(args[1], ctx)
	if err != nil {
		return nil, err
	}
	str, _ := strings.CutPrefix(fst, snd)
	return singleValue(str), nil
}

func callSubstringAfter(ctx Node, args []Expr) ([]Item, error) {
	if len(args) != 2 {
		return nil, errArgument
	}
	fst, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	snd, err := getStringFromExpr(args[1], ctx)
	if err != nil {
		return nil, err
	}
	str, _ := strings.CutSuffix(fst, snd)
	return singleValue(str), nil
}

func callName(ctx Node, args []Expr) ([]Item, error) {
	if len(args) == 0 {
		n := ctx.QualifiedName()
		return singleValue(n), nil
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return singleValue(""), nil
	}
	n, ok := items[0].(nodeItem)
	if !ok {
		return nil, errType
	}
	return singleValue(n.Node().QualifiedName()), nil
}

func callLocalName(ctx Node, args []Expr) ([]Item, error) {
	if len(args) == 0 {
		return singleValue(ctx.LocalName()), nil
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return singleValue(""), nil
	}
	n, ok := items[0].(nodeItem)
	if !ok {
		return nil, errType
	}
	return singleValue(n.Node().LocalName()), nil
}

func callRoot(ctx Node, args []Expr) ([]Item, error) {
	var get func(Node) Node

	get = func(n Node) Node {
		p := n.Parent()
		if p == nil {
			return n
		}
		return get(p)
	}
	if len(args) == 0 {
		n := get(ctx)
		return singleNode(n), nil
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	n, ok := items[0].(nodeItem)
	if !ok {
		return nil, errType
	}
	root := get(n.Node())
	return singleNode(root), nil
}

func callPath(ctx Node, args []Expr) ([]Item, error) {
	var get func(n Node) []string

	get = func(n Node) []string {
		p := n.Parent()
		if p == nil {
			return nil
		}
		x := get(p)
		g := []string{n.QualifiedName()}
		return append(g, x...)
	}

	if len(args) == 0 {
		list := get(ctx)
		return singleValue(strings.Join(list, "/")), nil
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	n, ok := items[0].(nodeItem)
	if !ok {
		return nil, errType
	}
	return callPath(n.Node(), nil)
}

func callHasChildren(ctx Node, args []Expr) ([]Item, error) {
	if len(args) == 0 {
		el, ok := ctx.(*Element)
		if !ok {
			return nil, errType
		}
		return singleValue(len(el.Nodes) > 0), nil
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	n, ok := items[0].(nodeItem)
	if !ok {
		return nil, errType
	}
	return callHasChildren(n.Node(), nil)
}

func callInnermost(ctx Node, args []Expr) ([]Item, error) {
	return nil, errImplemented
}

func callOutermost(ctx Node, args []Expr) ([]Item, error) {
	return nil, errImplemented
}

func callBoolean(ctx Node, args []Expr) ([]Item, error) {
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return callFalse(ctx, args)
	}
	ok, err := getBooleanFromItem(items[0])
	if err != nil {
		return nil, err
	}
	return singleValue(ok), nil
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
	return singleValue(true), nil
}

func callFalse(_ Node, _ []Expr) ([]Item, error) {
	return singleValue(false), nil
}

func getStringFromExpr(expr Expr, ctx Node) (string, error) {
	items, err := expr.Next(ctx)
	if err != nil {
		return "", err
	}
	if len(items) != 1 {
		return "", errType
	}
	v, ok := items[0].(literalItem)
	if !ok {
		return "", errType
	}
	str, ok := v.Value().(string)
	if !ok {
		return "", errType
	}
	return str, nil
}

func getBooleanFromItem(item Item) (bool, error) {
	if _, ok := item.(nodeItem); ok {
		return ok, nil
	}
	var res bool
	switch value := item.Value().(type) {
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
