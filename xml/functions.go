package xml

import (
	"regexp"
	"strconv"
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
	"string":           callString,
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
	"matches":          callMatches,
	"tokenize":         callTokenize,
	"sum":              callSum,
	"count":            callCount,
	"avg":              callAvg,
	"min":              callMin,
	"max":              callMax,
	"zero-or-more":     callZeroOrOne,
	"one-or-more":      callOneOrMore,
	"exactly-one":      callExactlyOne,
	"position":         callPosition,
	"last":             callLast,
	"current-date":     callCurrentDate,
}

type builtinFunc func(Node, []Expr, Environ) ([]Item, error)

func checkArity(argCount int, fn builtinFunc) builtinFunc {
	do := func(node Node, args []Expr, env Environ) ([]Item, error) {
		if len(args) < argCount {
			return nil, errArgument
		}
		return fn(node, args, env)
	}
	return do
}

func callSum(ctx Node, args []Expr, env Environ) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	items, err := expandArgs(ctx, args, env)
	if err != nil {
		return nil, err
	}
	var result float64
	for _, n := range items {
		v, err := strconv.ParseFloat(n.Node().Value(), 64)
		if err != nil {
			return nil, err
		}
		result += v
	}
	return singleValue(result), nil
}

func callAvg(ctx Node, args []Expr, env Environ) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	items, err := expandArgs(ctx, args, env)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errArgument
	}
	var result float64
	for _, n := range items {
		v, err := strconv.ParseFloat(n.Node().Value(), 64)
		if err != nil {
			return nil, err
		}
		result += v
	}
	return singleValue(result / float64(len(items))), nil
}

func callCount(ctx Node, args []Expr, env Environ) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	items, err := expandArgs(ctx, args, env)
	if err != nil {
		return nil, err
	}
	return singleValue(float64(len(items))), nil
}

func callMin(ctx Node, args []Expr, env Environ) ([]Item, error) {
	return nil, errImplemented
}

func callMax(ctx Node, args []Expr, env Environ) ([]Item, error) {
	return nil, errImplemented
}

func callZeroOrOne(ctx Node, args []Expr, env Environ) ([]Item, error) {
	return nil, errImplemented
}

func callOneOrMore(ctx Node, args []Expr, env Environ) ([]Item, error) {
	return nil, errImplemented
}

func callExactlyOne(ctx Node, args []Expr, env Environ) ([]Item, error) {
	return nil, errImplemented
}

func callPosition(ctx Node, args []Expr, env Environ) ([]Item, error) {
	return nil, errImplemented
}

func callLast(ctx Node, args []Expr, env Environ) ([]Item, error) {
	return nil, errImplemented
}

func callCurrentDate(ctx Node, args []Expr, env Environ) ([]Item, error) {
	return nil, errImplemented
}

func callString(ctx Node, args []Expr, env Environ) ([]Item, error) {
	if len(args) == 0 {
		return singleValue(ctx.Value()), nil
	}
	items, err := expandArgs(ctx, args, env)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return singleValue(""), nil
	}
	if !items[0].Atomic() {
		return callString(items[0].Node(), args, env)
	}
	var str string
	switch v := items[0].Value().(type) {
	case bool:
		str = strconv.FormatBool(v)
	case float64:
		str = strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		str = v
	default:
		return nil, errType
	}
	return singleValue(str), nil
}

func callCompare(ctx Node, args []Expr, env Environ) ([]Item, error) {
	if len(args) != 2 {
		return nil, errArgument
	}
	fst, err := getStringFromExpr(args[0], ctx, env)
	if err != nil {
		return nil, err
	}
	snd, err := getStringFromExpr(args[1], ctx, env)
	if err != nil {
		return nil, err
	}
	cmp := strings.Compare(fst, snd)
	return singleValue(float64(cmp)), nil
}
func callConcat(ctx Node, args []Expr, env Environ) ([]Item, error) {
	return nil, errImplemented
}

func callStringJoin(ctx Node, args []Expr, env Environ) ([]Item, error) {
	return nil, errImplemented
}

func callSubstring(ctx Node, args []Expr, env Environ) ([]Item, error) {
	return nil, errImplemented
}

func callStringLength(ctx Node, args []Expr, env Environ) ([]Item, error) {
	if len(args) == 0 {
		str := ctx.Value()
		return singleValue(float64(len(str))), nil
	}
	items, err := expandArgs(ctx, args, env)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return singleValue(0.0), nil
	}
	if !items[0].Atomic() {
		return nil, errType
	}
	str, ok := items[0].Value().(string)
	if !ok {
		return nil, errType
	}
	return singleValue(float64(len(str))), nil
}

func callNormalizeSpace(ctx Node, args []Expr, env Environ) ([]Item, error) {
	var (
		str string
		err error
	)
	switch len(args) {
	case 0:
		str = ctx.Value()
	case 1:
		str, err = getStringFromExpr(args[0], ctx, env)
	default:
		err = errArgument
	}
	if err != nil {
		return nil, err
	}
	var prev rune
	clear := func(r rune) rune {
		if r == ' ' && r == prev {
			return -1
		}
		prev = r
		return r
	}
	str = strings.TrimSpace(str)
	return singleValue(strings.Map(clear, str)), nil
}

func callUppercase(ctx Node, args []Expr, env Environ) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	str, err := getStringFromExpr(args[0], ctx, env)
	if err != nil {
		return nil, err
	}
	return singleValue(strings.ToUpper(str)), nil
}

func callLowercase(ctx Node, args []Expr, env Environ) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	str, err := getStringFromExpr(args[0], ctx, env)
	if err != nil {
		return nil, err
	}
	return singleValue(strings.ToLower(str)), nil
}

func callTranslate(ctx Node, args []Expr, env Environ) ([]Item, error) {
	return nil, errImplemented
}

func callContains(ctx Node, args []Expr, env Environ) ([]Item, error) {
	if len(args) != 2 {
		return nil, errArgument
	}
	fst, err := getStringFromExpr(args[0], ctx, env)
	if err != nil {
		return nil, err
	}
	snd, err := getStringFromExpr(args[1], ctx, env)
	if err != nil {
		return nil, err
	}
	res := strings.Contains(fst, snd)
	return singleValue(res), nil
}

func callStartsWith(ctx Node, args []Expr, env Environ) ([]Item, error) {
	if len(args) != 2 {
		return nil, errArgument
	}
	fst, err := getStringFromExpr(args[0], ctx, env)
	if err != nil {
		return nil, err
	}
	snd, err := getStringFromExpr(args[1], ctx, env)
	if err != nil {
		return nil, err
	}
	if snd == "" {
		return callTrue(ctx, args, env)
	}
	if fst == "" && snd != "" {
		return callFalse(ctx, args, env)
	}
	res := strings.HasPrefix(fst, snd)
	return singleValue(res), nil
}

func callEndsWith(ctx Node, args []Expr, env Environ) ([]Item, error) {
	if len(args) != 2 {
		return nil, errArgument
	}
	fst, err := getStringFromExpr(args[0], ctx, env)
	if err != nil {
		return nil, err
	}
	snd, err := getStringFromExpr(args[1], ctx, env)
	if err != nil {
		return nil, err
	}
	if snd == "" {
		return callTrue(ctx, args, env)
	}
	if fst == "" && snd != "" {
		return callFalse(ctx, args, env)
	}
	res := strings.HasSuffix(fst, snd)
	return singleValue(res), nil
}

func callSubstringBefore(ctx Node, args []Expr, env Environ) ([]Item, error) {
	if len(args) != 2 {
		return nil, errArgument
	}
	fst, err := getStringFromExpr(args[0], ctx, env)
	if err != nil {
		return nil, err
	}
	snd, err := getStringFromExpr(args[1], ctx, env)
	if err != nil {
		return nil, err
	}
	str, _ := strings.CutPrefix(fst, snd)
	return singleValue(str), nil
}

func callTokenize(ctx Node, args []Expr, env Environ) ([]Item, error) {
	if len(args) != 2 {
		return nil, errArgument
	}
	fst, err := getStringFromExpr(args[0], ctx, env)
	if err != nil {
		return nil, err
	}
	snd, err := getStringFromExpr(args[1], ctx, env)
	if err != nil {
		return nil, err
	}
	re, err := regexp.Compile(snd)
	if err != nil {
		return nil, err
	}
	var items []Item
	for _, str := range re.Split(fst, -1) {
		items = append(items, createLiteral(str))
	}
	return items, nil
}

func callMatches(ctx Node, args []Expr, env Environ) ([]Item, error) {
	if len(args) != 2 {
		return nil, errArgument
	}
	fst, err := getStringFromExpr(args[0], ctx, env)
	if err != nil {
		return nil, err
	}
	snd, err := getStringFromExpr(args[1], ctx, env)
	if err != nil {
		return nil, err
	}
	ok, err := regexp.MatchString(snd, fst)
	return singleValue(ok), err
}

func callSubstringAfter(ctx Node, args []Expr, env Environ) ([]Item, error) {
	if len(args) != 2 {
		return nil, errArgument
	}
	fst, err := getStringFromExpr(args[0], ctx, env)
	if err != nil {
		return nil, err
	}
	snd, err := getStringFromExpr(args[1], ctx, env)
	if err != nil {
		return nil, err
	}
	str, _ := strings.CutSuffix(fst, snd)
	return singleValue(str), nil
}

func callName(ctx Node, args []Expr, env Environ) ([]Item, error) {
	if len(args) == 0 {
		n := ctx.QualifiedName()
		return singleValue(n), nil
	}
	items, err := expandArgs(ctx, args, env)
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

func callLocalName(ctx Node, args []Expr, env Environ) ([]Item, error) {
	if len(args) == 0 {
		return singleValue(ctx.LocalName()), nil
	}
	items, err := expandArgs(ctx, args, env)
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

func callRoot(ctx Node, args []Expr, env Environ) ([]Item, error) {
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
	items, err := expandArgs(ctx, args, env)
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

func callPath(ctx Node, args []Expr, env Environ) ([]Item, error) {
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
	items, err := expandArgs(ctx, args, env)
	if err != nil {
		return nil, err
	}
	n, ok := items[0].(nodeItem)
	if !ok {
		return nil, errType
	}
	return callPath(n.Node(), nil, env)
}

func callHasChildren(ctx Node, args []Expr, env Environ) ([]Item, error) {
	if len(args) == 0 {
		el, ok := ctx.(*Element)
		if !ok {
			return nil, errType
		}
		return singleValue(len(el.Nodes) > 0), nil
	}
	items, err := expandArgs(ctx, args, env)
	if err != nil {
		return nil, err
	}
	n, ok := items[0].(nodeItem)
	if !ok {
		return nil, errType
	}
	return callHasChildren(n.Node(), nil, env)
}

func callInnermost(ctx Node, args []Expr, env Environ) ([]Item, error) {
	return nil, errImplemented
}

func callOutermost(ctx Node, args []Expr, env Environ) ([]Item, error) {
	return nil, errImplemented
}

func callBoolean(ctx Node, args []Expr, env Environ) ([]Item, error) {
	items, err := expandArgs(ctx, args, env)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return callFalse(ctx, args, env)
	}
	ok, err := getBooleanFromItem(items[0])
	if err != nil {
		return nil, err
	}
	return singleValue(ok), nil
}

func callNot(ctx Node, args []Expr, env Environ) ([]Item, error) {
	items, err := callBoolean(ctx, args, env)
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

func callTrue(_ Node, _ []Expr, _ Environ) ([]Item, error) {
	return singleValue(true), nil
}

func callFalse(_ Node, _ []Expr, _ Environ) ([]Item, error) {
	return singleValue(false), nil
}

func getStringFromExpr(expr Expr, ctx Node, env Environ) (string, error) {
	items, err := expr.Next(ctx, env)
	if err != nil {
		return "", err
	}
	if len(items) != 1 {
		return "", errType
	}
	switch v := items[0].(type) {
	case literalItem:
		str, ok := v.Value().(string)
		if !ok {
			return "", errType
		}
		return str, nil
	case nodeItem:
		return v.Node().Value(), nil
	default:
		return "", errType
	}
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

func expandArgs(ctx Node, args []Expr, env Environ) ([]Item, error) {
	var list []Item
	for _, a := range args {
		i, err := a.Next(ctx, env)
		if err != nil {
			return nil, err
		}
		list = append(list, i...)
	}
	return list, nil
}
