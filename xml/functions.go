package xml

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
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
	"date":             callDate,
	"current-date":     callCurrentDate,
	"current-datetime": callCurrentDatetime,
	"decimal":          callDecimal,
	"exists":           callExists,
	"empty":            callEmpty,
	"tail":             callTail,
	"head":             callHead,
	"reverse":          callReverse,
}

type builtinFunc func(Context, []Expr) ([]Item, error)

func callExists(ctx Context, args []Expr) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	return singleValue(!isEmpty(items)), nil
}

func callEmpty(ctx Context, args []Expr) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	return singleValue(isEmpty(items)), nil
}

func callHead(ctx Context, args []Expr) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if isEmpty(items) {
		return nil, nil
	}
	return createSingle(items[0]), nil
}

func callTail(ctx Context, args []Expr) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if isEmpty(items) {
		return nil, nil
	}
	return items[1:], nil
}

func callReverse(ctx Context, args []Expr) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	slices.Reverse(items)
	return items, nil
}

func callDecimal(ctx Context, args []Expr) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	str, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	if str == "" {
		return singleValue(math.NaN()), nil
	}
	v, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return nil, ErrCast
	}
	return singleValue(v), nil
}

func callSum(ctx Context, args []Expr) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	items, err := expandArgs(ctx, args)
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

func callAvg(ctx Context, args []Expr) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	items, err := expandArgs(ctx, args)
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

func callCount(ctx Context, args []Expr) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	return singleValue(float64(len(items))), nil
}

func callMin(ctx Context, args []Expr) ([]Item, error) {
	return nil, errImplemented
}

func callMax(ctx Context, args []Expr) ([]Item, error) {
	return nil, errImplemented
}

func callZeroOrOne(ctx Context, args []Expr) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if len(items) > 1 {
		return nil, fmt.Errorf("too many elements")
	}
	return items, nil
}

func callOneOrMore(ctx Context, args []Expr) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if len(items) < 1 {
		return nil, fmt.Errorf("not enough elements")
	}
	return items, nil
}

func callExactlyOne(ctx Context, args []Expr) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if len(items) != 1 {
		return nil, fmt.Errorf("only one element expected")
	}
	return items, nil
}

func callPosition(ctx Context, args []Expr) ([]Item, error) {
	return singleValue(float64(ctx.Index)), nil
}

func callLast(ctx Context, args []Expr) ([]Item, error) {
	return singleValue(float64(ctx.Size)), nil
}

func callCurrentDate(ctx Context, args []Expr) ([]Item, error) {
	return callCurrentDatetime(ctx, args)
}

func callCurrentDatetime(ctx Context, args []Expr) ([]Item, error) {
	return singleValue(time.Now()), nil
}

func callDate(ctx Context, args []Expr) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	str, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	v, err := time.Parse("2006-01-02", str)
	if err != nil {
		return nil, ErrCast
	}
	return singleValue(v), nil
}

func callString(ctx Context, args []Expr) ([]Item, error) {
	if len(args) == 0 {
		return singleValue(ctx.Value()), nil
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return singleValue(""), nil
	}
	if !items[0].Atomic() {
		return callString(defaultContext(items[0].Node()), nil)
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

func callCompare(ctx Context, args []Expr) ([]Item, error) {
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

func callConcat(ctx Context, args []Expr) ([]Item, error) {
	return nil, errImplemented
}

func callStringJoin(ctx Context, args []Expr) ([]Item, error) {
	return nil, errImplemented
}

func callSubstring(ctx Context, args []Expr) ([]Item, error) {
	return nil, errImplemented
}

func callStringLength(ctx Context, args []Expr) ([]Item, error) {
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
	if !items[0].Atomic() {
		return callStringLength(defaultContext(items[0].Node()), nil)
	}
	str, ok := items[0].Value().(string)
	if !ok {
		return nil, errType
	}
	return singleValue(float64(len(str))), nil
}

func callNormalizeSpace(ctx Context, args []Expr) ([]Item, error) {
	var (
		str string
		err error
	)
	switch len(args) {
	case 0:
		str = ctx.Value()
	case 1:
		str, err = getStringFromExpr(args[0], ctx)
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

func callUppercase(ctx Context, args []Expr) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	str, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	return singleValue(strings.ToUpper(str)), nil
}

func callLowercase(ctx Context, args []Expr) ([]Item, error) {
	if len(args) != 1 {
		return nil, errArgument
	}
	str, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	return singleValue(strings.ToLower(str)), nil
}

func callTranslate(ctx Context, args []Expr) ([]Item, error) {
	return nil, errImplemented
}

func callContains(ctx Context, args []Expr) ([]Item, error) {
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

func callStartsWith(ctx Context, args []Expr) ([]Item, error) {
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

func callEndsWith(ctx Context, args []Expr) ([]Item, error) {
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

func callSubstringBefore(ctx Context, args []Expr) ([]Item, error) {
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

func callTokenize(ctx Context, args []Expr) ([]Item, error) {
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

func callMatches(ctx Context, args []Expr) ([]Item, error) {
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
	ok, err := regexp.MatchString(snd, fst)
	return singleValue(ok), err
}

func callSubstringAfter(ctx Context, args []Expr) ([]Item, error) {
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

func callName(ctx Context, args []Expr) ([]Item, error) {
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

func callLocalName(ctx Context, args []Expr) ([]Item, error) {
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

func callRoot(ctx Context, args []Expr) ([]Item, error) {
	var get func(Node) Node

	get = func(n Node) Node {
		p := n.Parent()
		if p == nil {
			return n
		}
		return get(p)
	}
	if len(args) == 0 {
		n := get(ctx.Node)
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

func callPath(ctx Context, args []Expr) ([]Item, error) {
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
		list := get(ctx.Node)
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
	return callPath(defaultContext(n.Node()), nil)
}

func callHasChildren(ctx Context, args []Expr) ([]Item, error) {
	if len(args) == 0 {
		nodes := ctx.Nodes()
		return singleValue(len(nodes) > 0), nil
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	n, ok := items[0].(nodeItem)
	if !ok {
		return nil, errType
	}
	return callHasChildren(defaultContext(n.Node()), nil)
}

func callInnermost(ctx Context, args []Expr) ([]Item, error) {
	return nil, errImplemented
}

func callOutermost(ctx Context, args []Expr) ([]Item, error) {
	return nil, errImplemented
}

func callBoolean(ctx Context, args []Expr) ([]Item, error) {
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if isEmpty(items) {
		return callFalse(ctx, args)
	}
	return singleValue(items[0].True()), nil
}

func callNot(ctx Context, args []Expr) ([]Item, error) {
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

func callTrue(_ Context, _ []Expr) ([]Item, error) {
	return singleValue(true), nil
}

func callFalse(_ Context, _ []Expr) ([]Item, error) {
	return singleValue(false), nil
}

func getStringFromExpr(expr Expr, ctx Context) (string, error) {
	items, err := expr.find(ctx)
	if err != nil {
		return "", err
	}
	if len(items) != 1 {
		return "", nil
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
	return item.True(), nil
}

func expandArgs(ctx Context, args []Expr) ([]Item, error) {
	var list []Item
	for _, a := range args {
		i, err := a.find(ctx)
		if err != nil {
			return nil, err
		}
		list = append(list, i...)
	}
	return list, nil
}
