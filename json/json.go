package json

type Query interface {
	Get(any) (any, error)
}

type Expr interface {
	Eval(any) (any, error)
}

type query struct {
	expr Expr
}

func (q query) Get(doc any) (any, error) {
	a, err := q.expr.Eval(doc)
	return a, err
}

var builtins = map[string]builtinFunc{
	"string":          checkArity(strString, 1),
	"length":          checkArity(strLength, 1),
	"substring":       checkArity(strSubstring, 2, -1),
	"substringBefore": checkArity(strSubstringBefore, 2),
	"substringAfter":  checkArity(strSubstringAfter, 2),
	"uppercase":       checkArity(strUppercase, 1),
	"lowercase":       checkArity(strLowercase, 1),
	"trim":            checkArity(strTrim, 1),
	"pad":             checkArity(strPad, 2, " "),
	"contains":        checkArity(strContains, 2),
	"split":           checkArity(strSplit, 3, -1),
	"replace":         checkArity(strReplace, 3, -1),
	"base64encode":    checkArity(strBase64Encode, 1),
	"base64decode":    checkArity(strBase64Decode, 1),
	"number":          checkArity(numberNumber, 1),
	"abs":             checkArity(numberAbs, 1),
	"ceil":            checkArity(numberCeil, 1),
	"floor":           checkArity(numberFloor, 1),
	"round":           checkArity(numberRound, 1),
	"power":           checkArity(numberPower, 2),
	"sqrt":            checkArity(numberSqrt, 1),
	"random":          checkArity(numberRandom, 0),
	"formatNumber":    checkArity(numberFormatNumber, 1),
	"formatInteger":   checkArity(numberFormatInteger, 1),
	"formatBase":      checkArity(numberFormatBase, 1),
	"parseInt":        checkArity(numberParseInt, 1),
	"parseFloat":      checkArity(numberParseFloat, 1),
	"sum":             nil,
	"min":             nil,
	"max":             nil,
	"average":         nil,
	"boolean":         checkArity(boolBoolean, 1),
	"not":             checkArity(boolNot, 1),
	"count":           checkArity(arrayCount, 1),
	"append":          checkArity(arrayAppend, 2),
	"sort":            checkArity(arraySort, 1),
	"reverse":         checkArity(arrayReverse, 1),
	"shuffle":         checkArity(arrayShuffle, 1),
	"distinct":        checkArity(arrayDistinct, 1),
	"zip":             checkArity(arrayZip, 2),
	"join":            checkArity(arrayJoin, 1, ""),
	"keys":            checkArity(objectKeys, 1),
	"lookup":          checkArity(objectLookup, 2),
	"merge":           checkArity(objectMerge, 1),
	"type":            checkArity(objectType, 1),
	"values":          checkArity(objectValues, 1),
	"now":             checkArity(timeNow, 1),
	"millis":          checkArity(timeMillis, 0),
	"fromMillis":      checkArity(timeFromMillis, 1),
	"toMillis":        checkArity(timeToMillis, 1),
}

func typeError(arg string) error {
	return fmt.Errorf("%s invalid %w", arg, errType)
}

type builtinFunc func(any, []any) (any, error)

func checkArity(fn builtinFunc, minArgsCount int, defaults ...any) builtinFunc {
	do := func(ctx any, args []any) (any, error) {
		if len(args) < minArgsCount-1 {
			return nil, errArgument
		}
		if minArgsCount > 0 && len(args) < minArgsCount-1 {
			args = append([]any{ctx}, args...)
		}
		totalArgsCount := minArgsCount + len(defaults)
		if len(args) > totalArgsCount {
			return nil, errArgument
		}
		if len(args) < totalArgsCount {
			tmp := make([]any, totalArgsCount)
			copy(tmp[len(tmp)-len(defaults):], defaults)
			copy(tmp, args)
			args = tmp
		}
		return fn(ctx, args)
	}
	return do
}

func strString(ctx any, args []any) (any, error) {
	if args[0] == nil {
		return "null", nil
	}
	switch a := args[0].(type) {
	case string:
		return a, nil
	case bool:
		return strconv.FormatBool(a), nil
	case float64:
		return strconv.FormatFloat(a, 'f', -1, 64), nil
	case []any, map[string]any:
		var (
			buf bytes.Buffer
			ws  = NewWriter(&buf)
		)
		if err := ws.Write(a); err != nil {
			return "", err
		}
		return buf.String(), nil
	default:
		return "", nil
	}
}

func strLength(ctx any, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, typeError("str")
	}
	return float64(len(str)), nil
}

func strSubstring(ctx any, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, typeError("str")
	}
	var (
		end float64 = float64(len(str))
		beg float64
	)
	beg, ok = args[1].(float64)
	if !ok {
		return nil, typeError("begin")
	}
	length, ok := args[2].(float64)
	if !ok {
		return nil, typeError("length")
	}
	if offset := beg + length; int(offset) < len(str) {
		end = offset
	}
	return str[int(beg):int(end)], nil
}

func strSubstringBefore(ctx any, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, typeError("str")
	}
	prefix, ok := args[1].(string)
	if !ok {
		return nil, typeError("prefix")
	}
	res, _ := strings.CutPrefix(str, prefix)
	return res, nil
}

func strSubstringAfter(ctx any, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, typeError("str")
	}
	suffix, ok := args[1].(string)
	if !ok {
		return nil, typeError("suffix")
	}
	res, _ := strings.CutSuffix(str, suffix)
	return res, nil
}

func strUppercase(ctx any, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, typeError("str")
	}
	return strings.ToUpper(str), nil
}

func strLowercase(ctx any, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, typeError("str")
	}
	return strings.ToLower(str), nil
}

func strTrim(ctx any, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, typeError("str")
	}
	return strings.TrimSpace(str), nil
}

func strPad(ctx any, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, typeError("str")
	}
	width, ok := args[1].(float64)
	if !ok {
		return nil, typeError("width")
	}
	size := math.Abs(width)
	if len(str) >= int(size) || size == 0 {
		return str, nil
	}
	chars, ok := args[2].(string)
	if !ok {
		return nil, typeError("chars")
	}
	if chars == "" {
		return nil, typeError("chars")
	}
	var (
		repeat = (int(size) - len(str)) / len(chars)
		filler string
	)
	if repeat == 0 {
		repeat += 1
	}
	filler = strings.Repeat(chars, repeat)
	if width > 0 {
		str += filler
	} else {
		str = filler + str
	}
	return str, nil
}

func strContains(ctx any, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, typeError("str")
	}
	search, ok := args[1].(string)
	if !ok {
		return nil, typeError("pattern")
	}
	return strings.Contains(str, search), nil
}

func strSplit(ctx any, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, typeError("str")
	}
	sep, ok := args[1].(string)
	if !ok {
		return nil, typeError("separator")
	}
	limit, ok := args[2].(float64)
	if !ok {
		return nil, typeError("limit")
	}
	var res []any
	for _, s := range strings.SplitN(str, sep, int(limit)) {
		res = append(res, s)
	}
	return res, nil
}

func strReplace(ctx any, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, typeError("str")
	}
	pattern, ok := args[1].(string)
	if !ok {
		return nil, typeError("pattern")
	}
	replace, ok := args[2].(string)
	if !ok {
		return nil, typeError("replace")
	}
	limit, ok := args[3].(float64)
	if !ok {
		return nil, typeError("limit")
	}
	return strings.Replace(str, pattern, replace, int(limit)), nil
}

func strBase64Encode(ctx any, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, typeError("str")
	}
	return base64.StdEncoding.EncodeToString([]byte(str)), nil
}

func strBase64Decode(ctx any, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, typeError("str")
	}
	buf, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return nil, err
	}
	return string(buf), nil
}

func numberNumber(ctx any, args []any) (any, error) {
	switch a := args[0].(type) {
	case string:
		if f, err := strconv.ParseFloat(a, 64); err == nil {
			return f, nil
		}
		n, err := strconv.ParseInt(a, 0, 64)
		if err == nil {
			return float64(n), nil
		}
		return nil, err
	case bool:
		var f float64
		if a {
			f += 1
		}
		return f, nil
	case float64:
		return a, nil
	default:
		return nil, typeError("number")
	}
}

func numberAbs(ctx any, args []any) (any, error) {
	f, ok := args[0].(float64)
	if !ok {
		return nil, typeError("number")
	}
	return math.Abs(f), nil
}

func numberCeil(ctx any, args []any) (any, error) {
	f, ok := args[0].(float64)
	if !ok {
		return nil, typeError("number")
	}
	return math.Ceil(f), nil
}

func numberFloor(ctx any, args []any) (any, error) {
	f, ok := args[0].(float64)
	if !ok {
		return nil, typeError("number")
	}
	return math.Floor(f), nil
}

func numberRound(ctx any, args []any) (any, error) {
	f, ok := args[0].(float64)
	if !ok {
		return nil, typeError("number")
	}
	return math.Round(f), nil
}

func numberPower(ctx any, args []any) (any, error) {
	f, ok := args[0].(float64)
	if !ok {
		return nil, typeError("number")
	}
	e, ok := args[1].(float64)
	if !ok {
		return nil, typeError("exponent")
	}
	return math.Pow(f, e), nil
}

func numberSqrt(ctx any, args []any) (any, error) {
	f, ok := args[0].(float64)
	if !ok {
		return nil, typeError("number")
	}
	return math.Sqrt(f), nil
}

func numberRandom(ctx any, args []any) (any, error) {
	return rand.Float64(), nil
}

func numberFormatNumber(ctx any, args []any) (any, error) {
	return nil, nil
}

func numberFormatBase(ctx any, args []any) (any, error) {
	return nil, nil
}

func numberFormatInteger(ctx any, args []any) (any, error) {
	return nil, nil
}

func numberParseInt(ctx any, args []any) (any, error) {
	switch a := args[0].(type) {
	case float64:
		return a, nil
	case string:
		n, err := strconv.ParseInt(a, 0, 64)
		if err == nil {
			return float64(n), nil
		}
		return nil, err
	case bool:
		var f float64
		if a {
			f += 1
		}
		return f, nil
	default:
		return nil, typeError("str")
	}
}

func numberParseFloat(ctx any, args []any) (any, error) {
	switch a := args[0].(type) {
	case float64:
		return a, nil
	case string:
		n, err := strconv.ParseFloat(a, 64)
		if err == nil {
			return float64(n), nil
		}
		f, err := strconv.ParseInt(a, 10, 64)
		if err == nil {
			return float64(f), nil
		}
		return nil, err
	case bool:
		var f float64
		if a {
			f += 1
		}
		return f, nil
	default:
		return nil, typeError("str")
	}
}

func arrayCount(ctx any, args []any) (any, error) {
	switch a := args[0].(type) {
	case string, float64, bool, map[string]any:
		return 1.0, nil
	case []any:
		return float64(len(a)), nil
	default:
		return nil, typeError("array")
	}
}

func arrayAppend(ctx any, args []any) (any, error) {
	arr1, ok := args[0].([]any)
	if !ok {
		arr1 = []any{args[0]}
	}
	arr2, ok := args[1].([]any)
	if !ok {
		arr2 = []any{args[1]}
	}
	return slices.Concat(arr1, arr2), nil
}

func arraySort(ctx any, args []any) (any, error) {
	return nil, nil
}

func arrayReverse(ctx any, args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return nil, typeError("array")
	}
	tmp := slices.Clone(arr)
	slices.Reverse(tmp)
	return tmp, nil
}

func arrayShuffle(ctx any, args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return nil, typeError("array")
	}
	tmp := slices.Clone(arr)
	rand.Shuffle(len(tmp), func(i, j int) {
		tmp[i], tmp[j] = tmp[j], tmp[i]
	})
	return tmp, nil
}

func arrayDistinct(ctx any, args []any) (any, error) {
	return nil, errImplemented
}

func arrayZip(ctx any, args []any) (any, error) {
	return nil, errImplemented
}

func arrayJoin(ctx any, args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return nil, typeError("array")
	}
	var str []string
	for i := range arr {
		s, ok := arr[i].(string)
		if !ok {
			return nil, typeError("array")
		}
		str = append(str, s)
	}
	sep, ok := args[1].(string)
	if !ok {
		return nil, typeError("separator")
	}
	return strings.Join(str, sep), nil
}

func boolBoolean(ctx any, args []any) (any, error) {
	if args[0] == nil {
		return false, nil
	}
	switch a := args[0].(type) {
	case bool:
		return a, nil
	case string:
		if len(a) == 0 {
			return false, nil
		}
		return true, nil
	case float64:
		if a == 0 {
			return false, nil
		}
		return true, nil
	case []any:
		if len(a) == 0 {
			return false, nil
		}
		return true, nil
	case map[string]any:
		if len(a) == 0 {
			return false, nil
		}
		return true, nil
	default:
		return nil, typeError("arg")
	}
}

func boolNot(ctx any, args []any) (any, error) {
	res, err := boolBoolean(ctx, args)
	if err != nil {
		return nil, err
	}
	res, ok := res.(bool)
	if !ok {
		return nil, errType
	}
	return !res, nil
}

func objectKeys(ctx any, args []any) (any, error) {
	switch doc := args[0].(type) {
	case []any:
		var (
			arr  []any
			seen = make(map[string]struct{})
		)
		for i := range doc {
			d, ok := doc[i].(map[string]any)
			if !ok {
				continue
			}
			for k := range d {
				if _, ok := seen[k]; ok {
					continue
				}
				seen[k] = struct{}{}
				arr = append(arr, k)
			}
		}
		return arr, nil
	case map[string]any:
		var arr []any
		for k := range doc {
			arr = append(arr, k)
		}
		return arr, nil
	default:
		return nil, typeError("array")
	}
}

func objectValues(ctx any, args []any) (any, error) {
	switch doc := args[0].(type) {
	case []any:
		var arr []any
		for i := range doc {
			d, ok := doc[i].(map[string]any)
			if !ok {
				continue
			}
			for _, v := range d {
				arr = append(arr, v)
			}
		}
		return arr, nil
	case map[string]any:
		var arr []any
		for _, v := range doc {
			arr = append(arr, v)
		}
		return arr, nil
	default:
		return nil, typeError("array")
	}
}

func objectLookup(ctx any, args []any) (any, error) {
	key, ok := args[1].(string)
	if !ok {
		return nil, typeError("key")
	}
	switch doc := args[0].(type) {
	case []any:
		var arr []any
		for i := range doc {
			d, ok := doc[i].(map[string]any)
			if !ok {
				continue
			}
			arr = append(arr, d[key])
		}
		return arr, nil
	case map[string]any:
		return doc[key], nil
	default:
		return nil, typeError("array")
	}
}

func objectMerge(ctx any, args []any) (any, error) {
	arr, ok := args[0].([]any)
	if !ok {
		return nil, typeError("array")
	}
	res := make(map[string]any)
	for i := range arr {
		c, ok := arr[i].(map[string]any)
		if !ok {
			return nil, errType
		}
		for k, v := range c {
			res[k] = v
		}
	}
	return res, nil
}

func objectType(ctx any, args []any) (any, error) {
	if args[0] == nil {
		return "null", nil
	}
	switch args[0].(type) {
	case float64:
		return "number", nil
	case bool:
		return "boolean", nil
	case string:
		return "string", nil
	case []any:
		return "array", nil
	case map[string]any:
		return "object", nil
	default:
	}
}

func timeNow(ctx any, args []any) (any, error) {
	return time.Now().Format(time.RFC3339), nil
}

func timeMillis(ctx any, args []any) (any, error) {
	millis := time.Now().UnixMilli()
	return float64(millis), nil
}

func timeFromMillis(ctx any, args []any) (any, error) {
	millis, ok := args[0].(float64)
	if !ok {
		return nil, typeError("timestamp")
	}
	when := time.UnixMilli(int64(millis))
	return when.Format(time.RFC3339), nil
}

func timeToMillis(ctx any, args []any) (any, error) {
	str, ok := args[0].(string)
	if !ok {
		return nil, typeError("timestamp")
	}
	when, err := time.Parse(time.RFC3339, str)
	if err != nil {
		return nil, err
	}
	return float64(when.UnixMilli()), nil
}

type call struct {
	ident string
	args  []Expr
}

func (c call) Eval(doc any) (any, error) {
	fn, ok := builtins[c.ident]
	if !ok || fn == nil {
		return nil, fmt.Errorf("%s function unknown")
	}
	var arr []any
	for i := range c.args {
		a, err := c.args[i].Eval(doc)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", c.ident, err)
		}
		arr = append(arr, a)
	}
	ret, err := fn(doc, arr)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", c.ident, err)
	}
	return ret, nil
}

type literal[T string | float64 | bool] struct {
	value T
}

func (i literal[T]) Eval(_ any) (any, error) {
	return i.value, nil
}

type identifier struct {
	ident string
}

func (i identifier) Eval(doc any) (any, error) {
	switch doc := doc.(type) {
	case map[string]any:
		a, ok := doc[i.ident]
		if !ok {
			return nil, errUndefined
		}
		return a, nil
	case []any:
		var arr []any
		for j := range doc {
			a, err := i.Eval(doc[j])
			if err != nil {
				continue
			}
			if a == nil {
				continue
			}
			if a, ok := a.([]any); ok {
				arr = append(arr, a...)
			} else {
				arr = append(arr, a)
			}
		}
		if len(arr) == 0 {
			return nil, errUndefined
		}
		return arr, nil
	default:
		return nil, errType
	}
}

type reverse struct {
	expr Expr
}

func (r reverse) Eval(doc any) (any, error) {
	v, err := r.expr.Eval(doc)
	if err != nil {
		return nil, err
	}
	if arr, ok := v.([]any); ok && len(arr) == 1 {
		v = arr[0]
	}
	f, ok := v.(float64)
	if !ok {
		return nil, fmt.Errorf("syntax error: not a valid number")
	}
	return -f, nil
}

type ternary struct {
	cdt Expr
	csq Expr
	alt Expr
}

func (t ternary) Eval(doc any) (any, error) {
	ret, err := t.cdt.Eval(doc)
	if err != nil {
		return nil, err
	}
	if toBool(ret) {
		return t.csq.Eval(doc)
	}
	return t.alt.Eval(doc)
}

type binary struct {
	left  Expr
	right Expr
	op    rune
}

func (i binary) Eval(doc any) (any, error) {
	left, err := i.left.Eval(doc)
	if err != nil {
		return nil, err
	}
	right, err := i.right.Eval(doc)
	if err != nil {
		return nil, err
	}
	switch i.op {
	default:
		return nil, fmt.Errorf("syntax error: unsupported operator")
	case And:
		return toBool(left) && toBool(right), nil
	case Or:
		return toBool(left) || toBool(right), nil
	case Add:
		return apply(left, right, func(left, right float64) float64 {
			return left + right
		})
	case Sub:
		return apply(left, right, func(left, right float64) float64 {
			return left - right
		})
	case Mul:
		return apply(left, right, func(left, right float64) float64 {
			return left * right
		})
	case Div:
		return apply(left, right, func(left, right float64) float64 {
			if right == 0 {
				return 0
			}
			return left / right
		})
	case Mod:
		return apply(left, right, func(left, right float64) float64 {
			if right == 0 {
				return 0
			}
			return math.Mod(left, right)
		})
	case Eq:
		return isEq(left, right)
	case Ne:
		return isNe(left, right)
	case Lt:
		return isLe(left, right)
	case Le:
		ok, err := isLe(left, right)
		if !ok && err == nil {
			ok, err = isEq(left, right)
		}
		return ok, err
	case Gt:
		if ok, err := isEq(left, right); ok && err == nil {
			return !ok, nil
		}
		ok, err := isLe(left, right)
		return !ok, err
	case Ge:
		if ok, err := isEq(left, right); ok && err == nil {
			return ok, nil
		}
		ok, err := isLe(left, right)
		return !ok, err
	case Concat:
		return toStr(left) + toStr(right), nil
	case In:
		return isIn(left, right)
	}
}

type arrayTransform struct {
	expr Expr
}

func (a arrayTransform) Eval(doc any) (any, error) {
	res, err := a.expr.Eval(doc)
	if arr, ok := res.([]any); ok {
		return arr, err
	}
	return []any{res}, nil
}

type arrayBuilder struct {
	expr []Expr
}

func (b arrayBuilder) Eval(doc any) (any, error) {
	return b.eval(doc)
}

func (b arrayBuilder) eval(doc any) (any, error) {
	if arr, ok := doc.([]any); ok {
		return b.evalArray(arr)
	}
	return b.evalObject(doc)
}

func (b arrayBuilder) evalObject(doc any) (any, error) {
	var arr []any
	for i := range b.expr {
		a, err := b.expr[i].Eval(doc)
		if err != nil {
			continue
		}
		if as, ok := a.([]any); ok {
			arr = append(arr, as...)
		} else {
			arr = append(arr, a)
		}
	}
	return arr, nil
}

func (b arrayBuilder) evalArray(doc []any) (any, error) {
	var arr []any
	for i := range doc {
		a, err := b.eval(doc[i])
		if err != nil {
			return nil, err
		}
		arr = append(arr, a)
	}
	return arr, nil
}

type objectBuilder struct {
	expr Expr
	list map[Expr]Expr
}

func (b objectBuilder) Eval(doc any) (any, error) {
	if b.expr == nil {
		return b.evalDefault(doc)
	}
	return b.evalContext(doc)
}

func (b objectBuilder) evalDefault(doc any) (any, error) {
	if doc, ok := doc.([]any); ok {
		var arr []any
		for i := range doc {
			a, err := b.buildFromObject(doc[i])
			if err != nil {
				return nil, err
			}
			arr = append(arr, a)
		}
		return arr, nil
	}
	return b.buildFromObject(doc)
}

func (b objectBuilder) evalContext(doc any) (any, error) {
	doc, err := b.getContext(doc)
	if err != nil {
		return nil, err
	}
	if arr, ok := doc.([]any); ok {
		return b.buildFromArray(arr)
	}
	return b.buildFromObject(doc)
}

func (b objectBuilder) buildFromArray(doc []any) (any, error) {
	obj := make(map[string]any)
	for i := range doc {
		for k, v := range b.list {
			key, err := k.Eval(doc[i])
			if err != nil {
				return nil, err
			}
			str, ok := key.(string)
			if !ok {
				return nil, errType
			}
			val, _ := v.Eval(doc[i])
			if v, ok := obj[str]; ok {
				if arr, ok := v.([]any); ok {
					val = append(arr, val)
				} else {
					val = []any{v, val}
				}
			}
			obj[str] = val
		}
	}
	return obj, nil
}

func (b objectBuilder) buildFromObject(doc any) (any, error) {
	obj := make(map[string]any)
	for k, v := range b.list {
		key, err := k.Eval(doc)
		if err != nil {
			return nil, err
		}
		str, ok := key.(string)
		if !ok {
			return nil, errType
		}
		val, _ := v.Eval(doc)
		if v, ok := obj[str]; ok {
			if arr, ok := v.([]any); ok {
				val = append(arr, val)
			} else {
				val = []any{v, val}
			}
		}
		obj[str] = val
	}
	return obj, nil
}

func (b objectBuilder) getContext(doc any) (any, error) {
	if b.expr == nil {
		return doc, nil
	}
	return b.expr.Eval(doc)
}

type path struct {
	expr Expr
	next Expr
}

func (p path) Eval(doc any) (any, error) {
	return p.eval(doc)
}

func (p path) eval(doc any) (any, error) {
	var err error
	switch v := doc.(type) {
	case map[string]any:
		doc, err = p.getObject(v)
	case []any:
		doc, err = p.getArray(v)
	default:
		return nil, fmt.Errorf("%s: %w can not be queried (%T)", errType, doc)
	}
	return doc, err
}

func (p path) getArray(value []any) (any, error) {
	var arr []any
	for i := range value {
		a, err := p.eval(value[i])
		if err != nil {
			continue
		}
		if a != nil {
			arr = append(arr, a)
		}
	}
	return arr, nil
}

func (p path) getObject(value map[string]any) (any, error) {
	ret, err := p.expr.Eval(value)
	if err != nil {
		return nil, err
	}
	return p.getNext(ret)

}

func (p path) getNext(doc any) (any, error) {
	if p.next == nil {
		return doc, nil
	}
	arr, ok := doc.([]any)
	if !ok {
		return p.next.Eval(doc)
	}
	var ret []any
	for i := range arr {
		a, err := p.next.Eval(arr[i])
		if err != nil {
			return nil, err
		}
		ret = append(ret, a)
	}
	return ret, nil
}

type wildcard struct{}

func (w wildcard) Eval(doc any) (any, error) {
	obj, ok := doc.(map[string]any)
	if !ok {
		return nil, errType
	}
	var arr []any
	for k := range obj {
		arr = append(arr, obj[k])
	}
	return arr, nil
}

type descent struct{}

func (d descent) Eval(doc any) (any, error) {
	return nil, nil
}

type transform struct {
	expr Expr
	next Expr
}

func (t transform) Eval(doc any) (any, error) {
	doc, err := t.expr.Eval(doc)
	if err != nil {
		return nil, err
	}
	return t.next.Eval(doc)
}

type filter struct {
	expr  Expr
	check Expr
}

func (i filter) Eval(doc any) (any, error) {
	if doc, ok := doc.([]any); ok {
		var arr []any
		for j := range doc {
			a, err := i.eval(doc[j])
			if err != nil {
				continue
			}
			arr = append(arr, a)
		}
		return arr, nil
	}
	return i.eval(doc)
}

func (i filter) eval(doc any) (any, error) {
	doc, err := i.expr.Eval(doc)
	if err != nil {
		return nil, err
	}
	switch doc := doc.(type) {
	case map[string]any:
		ok, err := i.check.Eval(doc)
		if err != nil {
			return nil, err
		}
		if !toBool(ok) {
			return nil, errDiscard
		}
		return doc, nil
	case []any:
		var arr []any
		for j := range doc {
			res, err := i.check.Eval(doc[j])
			if err != nil {
				continue
			}
			if n, ok := res.(float64); ok {
				ix := int(n)
				if ix < 0 {
					ix += len(doc)
				}
				if ix == j {
					arr = append(arr, doc[j])
				}
			} else if b, ok := res.(bool); ok && b {
				arr = append(arr, doc[j])
			}
		}
		if len(arr) == 0 {
			return nil, errUndefined
		}
		if len(arr) == 1 {
			return arr[0], nil
		}
		return arr, nil
	case string, float64, bool:
		res, err := i.check.Eval(doc)
		if err != nil {
			return nil, err
		}
		ix, err := toFloat(res)
		if err != nil {
			return nil, err
		}
		if ix == 0 {
			return doc, nil
		}
		return nil, errDiscard
	default:
		return nil, errType
	}
}

type orderby struct {
	list Expr
}

func (o orderby) Eval(doc any) (any, error) {
	return nil, nil
}

func isIn(left, right any) (bool, error) {
	return false, nil
}

func isNe(left, right any) (bool, error) {
	ok, err := isEq(left, right)
	if err == nil {
		ok = !ok
	}
	return ok, err
}

func isEq(left, right any) (bool, error) {
	switch left := left.(type) {
	case string:
		right := toStr(right)
		return left == right, nil
	case float64:
		right, err := toFloat(right)
		if err != nil {
			return false, err
		}
		return left == right, nil
	case bool:
		right := toBool(right)
		return left == right, nil
	default:
		return false, fmt.Errorf("value type not supported")
	}
}

func isLe(left, right any) (bool, error) {
	switch left := left.(type) {
	case string:
		right := toStr(right)
		return cmp.Less(left, right), nil
	case float64:
		right, err := toFloat(right)
		if err != nil {
			return false, err
		}
		return cmp.Less(left, right), nil
	default:
		return false, fmt.Errorf("value type not supported")
	}
}

func toBool(v any) bool {
	switch v := v.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case string:
		return len(v) != 0
	default:
		return false
	}
}

func toStr(v any) string {
	switch v := v.(type) {
	case bool:
		return strconv.FormatBool(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		return v
	default:
		return ""
	}
}

func toFloat(v any) (float64, error) {
	switch v := v.(type) {
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("value not supported")
	}
}

func apply(left, right any, do func(left, right float64) float64) (any, error) {
	get := func(v any) (float64, error) {
		if arr, ok := v.([]any); ok && len(arr) == 1 {
			v = arr[0]
		}
		f, ok := v.(float64)
		if !ok {
			return 0, fmt.Errorf("syntax error: not a valid number")
		}
		return f, nil
	}
	x, err := get(left)
	if err != nil {
		return nil, err
	}
	y, err := get(right)
	if err != nil {
		return nil, err
	}
	return do(x, y), nil
}

const (
	powLowest = iota
	powComma
	powTernary
	powOr
	powAnd
	powCmp
	powEq
	powAdd
	powMul
	powPrefix
	powCall
	powGrp
	powMap
	powFilter
	powTransform
)

var bindings = map[rune]int{
	BegGrp:    powCall,
	BegArr:    powFilter,
	BegObj:    powFilter,
	Ternary:   powTernary,
	And:       powAnd,
	Or:        powOr,
	Add:       powAdd,
	Sub:       powAdd,
	Mul:       powMul,
	Div:       powMul,
	Mod:       powMul,
	Wildcard:  powMul,
	Parent:    powMul,
	Eq:        powEq,
	Ne:        powEq,
	In:        powCmp,
	Lt:        powCmp,
	Le:        powCmp,
	Gt:        powCmp,
	Ge:        powCmp,
	Concat:    powAdd,
	Map:       powMap,
	Transform: powTransform,
}

type compiler struct {
	scan *QueryScanner
	curr Token
	peek Token

	prefix map[rune]func() (Expr, error)
	infix  map[rune]func(Expr) (Expr, error)
}

func Compile(query string) (Query, error) {
	cp := compiler{
		scan: ScanQuery(strings.NewReader(query)),
	}
	cp.prefix = map[rune]func() (Expr, error){
		Ident:    cp.compileIdent,
		Func:     cp.compileIdent,
		Number:   cp.compileNumber,
		String:   cp.compileString,
		Boolean:  cp.compileBool,
		BegGrp:   cp.compileGroup,
		Wildcard: cp.compileWildcard,
		Descent:  cp.compileDescent,
		Sub:      cp.compileReverse,
		BegArr:   cp.compileArray,
		BegObj:   cp.compileObjectPrefix,
	}

	cp.infix = map[rune]func(Expr) (Expr, error){
		BegGrp:    cp.compileCall,
		BegArr:    cp.compileFilter,
		BegObj:    cp.compileObject,
		And:       cp.compileBinary,
		Or:        cp.compileBinary,
		Add:       cp.compileBinary,
		Sub:       cp.compileBinary,
		Mul:       cp.compileBinary,
		Wildcard:  cp.compileBinary,
		Div:       cp.compileBinary,
		Mod:       cp.compileBinary,
		Parent:    cp.compileBinary,
		Eq:        cp.compileBinary,
		Ne:        cp.compileBinary,
		Lt:        cp.compileBinary,
		Le:        cp.compileBinary,
		Gt:        cp.compileBinary,
		Ge:        cp.compileBinary,
		Concat:    cp.compileBinary,
		In:        cp.compileBinary,
		Map:       cp.compileMap,
		Ternary:   cp.compileTernary,
		Transform: cp.compileTransform,
	}

	cp.next()
	cp.next()
	return cp.Compile()
}

func (c *compiler) Compile() (Query, error) {
	return c.compile()
}

func (c *compiler) compile() (Query, error) {
	e, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	q := query{
		expr: e,
	}
	return q, nil
}

func (c *compiler) compileTransform(left Expr) (Expr, error) {
	expr := transform{
		expr: left,
	}
	c.next()
	next, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	expr.next = next
	return expr, nil
}

func (c *compiler) compileMap(left Expr) (Expr, error) {
	c.next()
	q := path{
		expr: left,
	}
	next, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	q.next = next
	return q, nil
}

func (c *compiler) compileFilter(left Expr) (Expr, error) {
	c.next()
	if c.is(EndArr) {
		c.next()
		a := arrayTransform{
			expr: left,
		}
		if c.is(BegArr) {
			left, err := c.compileFilter(left)
			if err != nil {
				return nil, err
			}
			a.expr = left
		}
		return a, nil
	}
	expr, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	if !c.is(EndArr) {
		return nil, fmt.Errorf("syntax error: missing ]")
	}
	c.next()

	f := filter{
		expr:  left,
		check: expr,
	}
	return f, nil
}

func (c *compiler) getString() string {
	defer c.next()
	return c.curr.Literal
}

func (c *compiler) getNumber() float64 {
	defer c.next()
	n, _ := strconv.ParseFloat(c.curr.Literal, 64)
	return n
}

func (c *compiler) getBool() bool {
	defer c.next()
	b, _ := strconv.ParseBool(c.curr.Literal)
	return b
}

func (c *compiler) compileExpr(pow int) (Expr, error) {
	fn, ok := c.prefix[c.curr.Type]
	if !ok {
		return nil, fmt.Errorf("syntax error: invalid prefix expression")
	}
	left, err := fn()
	if err != nil {
		return nil, err
	}
	for !c.is(EndArr) && pow < bindings[c.curr.Type] {
		fn, ok := c.infix[c.curr.Type]
		if !ok {
			return nil, fmt.Errorf("syntax error: invalid infix expression")
		}
		left, err = fn(left)
		if err != nil {
			return nil, err
		}
	}
	return left, nil
}

func (c *compiler) compileArray() (Expr, error) {
	c.next()
	var b arrayBuilder
	for !c.done() && !c.is(EndArr) {
		expr, err := c.compileExpr(powComma)
		if err != nil {
			return nil, err
		}
		b.expr = append(b.expr, expr)
		switch {
		case c.is(Comma):
			c.next()
		case c.is(EndArr):
		default:
			return nil, fmt.Errorf("syntax error: expected ',' or ']")
		}
	}
	if !c.is(EndArr) {
		return nil, fmt.Errorf("syntax error: missing ']")
	}
	c.next()
	return b, nil
}

func (c *compiler) compileObjectPrefix() (Expr, error) {
	return c.compileObject(nil)
}

func (c *compiler) compileObject(left Expr) (Expr, error) {
	c.next()
	b := objectBuilder{
		expr: left,
		list: make(map[Expr]Expr),
	}
	for !c.done() && !c.is(EndObj) {
		key, err := c.compileExpr(powLowest)
		if err != nil {
			return nil, err
		}
		if !c.is(Colon) {
			return nil, fmt.Errorf("syntax error: expected ':'")
		}
		c.next()
		val, err := c.compileExpr(powLowest)
		if err != nil {
			return nil, err
		}
		b.list[key] = val
		switch {
		case c.is(Comma):
			c.next()
		case c.is(EndObj):
		default:
			return nil, fmt.Errorf("syntax error: expected ',' or '}")
		}
	}
	if !c.is(EndObj) {
		return nil, fmt.Errorf("syntax error: expected '}")
	}
	c.next()
	return b, nil
}

func (c *compiler) compileWildcard() (Expr, error) {
	defer c.next()
	return wildcard{}, nil
}

func (c *compiler) compileDescent() (Expr, error) {
	defer c.next()
	return descent{}, nil
}

func (c *compiler) compileIdent() (Expr, error) {
	i := identifier{
		ident: c.getString(),
	}
	return i, nil
}

func (c *compiler) compileNumber() (Expr, error) {
	i := literal[float64]{
		value: c.getNumber(),
	}
	return i, nil
}

func (c *compiler) compileString() (Expr, error) {
	i := literal[string]{
		value: c.getString(),
	}
	return i, nil
}

func (c *compiler) compileBool() (Expr, error) {
	i := literal[bool]{
		value: c.getBool(),
	}
	return i, nil
}

func (c *compiler) compileGroup() (Expr, error) {
	c.next()
	expr, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	if !c.is(EndGrp) {
		return nil, fmt.Errorf("syntax error: missing ')'")
	}
	c.next()
	return expr, nil
}

func (c *compiler) compileReverse() (Expr, error) {
	c.next()
	expr, err := c.compileExpr(powPrefix)
	if err != nil {
		return nil, err
	}
	r := reverse{
		expr: expr,
	}
	return r, nil
}

func (c *compiler) compileTernary(left Expr) (Expr, error) {
	c.next()
	t := ternary{
		cdt: left,
	}
	fmt.Println(left)
	csq, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	if !c.is(Colon) {
		return nil, fmt.Errorf("syntax error: missing ':'")
	}
	c.next()
	alt, err := c.compileExpr(powLowest)
	if err != nil {
		return nil, err
	}
	t.csq = csq
	t.alt = alt
	return t, nil
}

func (c *compiler) compileBinary(left Expr) (Expr, error) {
	if c.is(Wildcard) {
		c.curr.Type = Mul
	} else if c.is(Parent) {
		c.curr.Type = Mod
	}
	var (
		pow = bindings[c.curr.Type]
		err error
	)
	bin := binary{
		left: left,
		op:   c.curr.Type,
	}
	c.next()
	bin.right, err = c.compileExpr(pow)
	return bin, err
}

func (c *compiler) compileCall(left Expr) (Expr, error) {
	ident, ok := left.(identifier)
	if !ok {
		return nil, fmt.Errorf("syntax error: identifier expected")
	}
	expr := call{
		ident: ident.ident,
	}
	c.next()
	for !c.done() && !c.is(EndGrp) {
		a, err := c.compileExpr(powLowest)
		if err != nil {
			return nil, err
		}
		expr.args = append(expr.args, a)
		switch {
		case c.is(Comma):
			c.next()
			if c.is(EndGrp) {
				return nil, fmt.Errorf("syntax error: trailing comma")
			}
		case c.is(EndGrp):
		default:
			return nil, fmt.Errorf("syntax error: unexpected token")
		}
	}
	if !c.is(EndGrp) {
		return nil, fmt.Errorf("syntax error: missing ')'")
	}
	c.next()
	return expr, nil
}

func (c *compiler) done() bool {
	return c.is(EOF)
}

func (c *compiler) is(kind rune) bool {
	return c.curr.Type == kind
}

func (c *compiler) next() {
	c.curr = c.peek
	c.peek = c.scan.Scan()
}

type queryMode int8

const (
	pathMode queryMode = 1 << iota
	filterMode
)

type QueryScanner struct {
	input *bufio.Reader
	char  rune

	mode queryMode

	str bytes.Buffer
}

func ScanQuery(r io.Reader) *QueryScanner {
	scan := QueryScanner{
		input: bufio.NewReader(r),
		mode:  pathMode,
	}
	scan.read()
	return &scan
}

func (s *QueryScanner) Scan() Token {
	defer s.str.Reset()
	s.skipBlank()

	var tok Token
	if s.done() {
		tok.Type = EOF
		return tok
	}
	switch {
	case isLetter(s.char):
		s.scanIdent(&tok)
	case isBackQuote(s.char):
		s.scanQuotedIdent(&tok)
	case isNumber(s.char):
		s.scanNumber(&tok)
	case isQuote(s.char):
		s.scanString(&tok)
	case isDelim(s.char) || s.char == '(' || s.char == ')':
		s.scanDelimiter(&tok)
	case isOperator(s.char):
		s.scanOperator(&tok)
	case isDollar(s.char):
		s.scanDollar(&tok)
	case isTransform(s.char):
		s.scanTransform(&tok)
	default:
		tok.Type = Invalid
	}
	s.setMode(tok)
	return tok
}

func (s *QueryScanner) setMode(tok Token) {
	if tok.Type == BegArr {
		s.mode = filterMode
	} else if tok.Type == EndArr {
		s.mode = pathMode
	}
}

func (s *QueryScanner) scanQuotedIdent(tok *Token) {
	for !s.done() && !isBackQuote(s.char) {
		s.write()
		s.read()
	}
	tok.Type = Ident
	tok.Literal = s.str.String()
	if !isBackQuote(s.char) {
		tok.Type = Invalid
	} else {
		s.read()
	}
}

func (s *QueryScanner) scanTransform(tok *Token) {
	s.read()
	tok.Type = Transform
}

func (s *QueryScanner) scanDollar(tok *Token) {
	s.read()
	if !isLetter(s.char) {
		tok.Type = Invalid
		return
	}
	s.scanIdent(tok)
	if tok.Type == Ident {
		tok.Type = Func
	}
}

func (s *QueryScanner) scanIdent(tok *Token) {
	for !s.done() && isAlpha(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	switch tok.Literal {
	case "true", "false":
		tok.Type = Boolean
	case "null":
		tok.Type = Null
	case "and":
		tok.Type = And
	case "or":
		tok.Type = Or
	case "in":
		tok.Type = In
	default:
		tok.Type = Ident
	}
}

func (s *QueryScanner) scanString(tok *Token) {
	s.read()
	for !s.done() && s.char != '"' {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	tok.Type = String
	if s.char != '"' {
		tok.Type = Invalid
	} else {
		s.read()
	}
}

func (s *QueryScanner) scanNumber(tok *Token) {
	tok.Type = Number
	for !s.done() && isNumber(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	if s.char == '.' {
		s.write()
		s.read()
		if !isNumber(s.char) {
			tok.Type = Invalid
			return
		}
		for !s.done() && isNumber(s.char) {
			s.write()
			s.read()
		}
		tok.Literal = s.str.String()
	}
	if s.char == 'e' || s.char == 'E' {
		s.write()
		s.read()
		if s.char == '-' || s.char == '+' {
			s.write()
			s.read()
		}
		if !isNumber(s.char) {
			tok.Type = Invalid
			return
		}
		for !s.done() && isNumber(s.char) {
			s.write()
			s.read()
		}
		tok.Literal = s.str.String()
	}
}

func (s *QueryScanner) scanOperator(tok *Token) {
	switch s.char {
	case '+':
		tok.Type = Add
	case '-':
		tok.Type = Sub
	case '*':
		if s.mode == pathMode {
			tok.Type = Wildcard
			if k := s.peek(); k == s.char {
				s.read()
				tok.Type = Descent
			}
		} else {
			tok.Type = Mul
		}
	case '/':
		tok.Type = Div
	case '%':
		if s.mode == pathMode {
			tok.Type = Parent
		} else {
			tok.Type = Mod
		}
	case '?':
		tok.Type = Ternary
	case ':':
	case '!':
		tok.Type = Invalid
		if k := s.peek(); k == '=' {
			s.read()
			tok.Type = Ne
		}
	case '=':
		tok.Type = Eq
	case '<':
		tok.Type = Lt
		if k := s.peek(); k == '=' {
			s.read()
			tok.Type = Le
		}
	case '>':
		tok.Type = Gt
		if k := s.peek(); k == '=' {
			s.read()
			tok.Type = Ge
		}
	case '.':
		tok.Type = Map
		if k := s.peek(); k == s.char {
			s.read()
			tok.Type = Range
		}
	case '&':
		tok.Type = Concat
	default:
		tok.Type = Invalid
	}
	if tok.Type != Invalid {
		s.read()
	}
}

func (s *QueryScanner) scanDelimiter(tok *Token) {
	switch s.char {
	case '(':
		tok.Type = BegGrp
	case ')':
		tok.Type = EndGrp
	case '[':
		tok.Type = BegArr
	case ']':
		tok.Type = EndArr
	case '{':
		tok.Type = BegObj
	case '}':
		tok.Type = EndObj
	case ',':
		tok.Type = Comma
	case ':':
		tok.Type = Colon
	default:
		tok.Type = Invalid
	}
	if tok.Type != Invalid {
		s.read()
	}
}

func (s *QueryScanner) write() {
	s.str.WriteRune(s.char)
}

func (s *QueryScanner) read() {
	char, _, err := s.input.ReadRune()
	if errors.Is(err, io.EOF) {
		char = utf8.RuneError
	}
	s.char = char
}

func (s *QueryScanner) peek() rune {
	defer s.input.UnreadRune()
	r, _, _ := s.input.ReadRune()
	return r
}

func (s *QueryScanner) done() bool {
	return s.char == utf8.RuneError
}

func (s *QueryScanner) skipBlank() {
	for !s.done() && unicode.IsSpace(s.char) {
		s.read()
	}
}

type Writer struct {
	ws *bufio.Writer

	Indent  string
	Pretty  bool
	Compact bool

	level int
}

func NewWriter(w io.Writer) *Writer {
	ws := Writer{
		ws:     bufio.NewWriter(w),
		Indent: "  ",
	}
	return &ws
}

func (w *Writer) Write(value any) error {
	defer func() {
		w.reset()
		w.ws.Flush()
	}()
	return w.writeValue(value)
}

func (w *Writer) writeValue(value any) error {
	switch v := value.(type) {
	case map[string]any:
		return w.writeObject(v)
	case []any:
		return w.writeArray(v)
	default:
		return w.writeLiteral(value)
	}
}

func (w *Writer) writeObject(value map[string]any) error {
	w.enter()

	w.ws.WriteRune('{')
	w.writeNL()
	var i int
	for k, v := range value {
		if i > 0 {
			w.ws.WriteRune(',')
			w.writeNL()
		}
		w.writePrefix()
		if err := w.writeKey(k); err != nil {
			return err
		}
		if err := w.writeValue(v); err != nil {
			return err
		}
		i++
	}
	w.leave()
	w.writeNL()
	w.writePrefix()
	w.ws.WriteRune('}')
	return nil
}

func (w *Writer) writeArray(value []any) error {
	w.enter()

	w.ws.WriteRune('[')
	w.writeNL()
	for i := range value {
		if i > 0 {
			w.ws.WriteRune(',')
			w.writeNL()
		}
		w.writePrefix()
		if err := w.writeValue(value[i]); err != nil {
			return err
		}
	}
	w.leave()
	w.writeNL()
	w.writePrefix()
	w.ws.WriteRune(']')
	return nil
}

func (w *Writer) writeLiteral(value any) error {
	if value == nil {
		w.ws.WriteString("null")
		return nil
	}
	switch v := value.(type) {
	case bool:
		if v {
			w.ws.WriteString("true")
		} else {
			w.ws.WriteString("false")
		}
	case float64:
		w.ws.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
	case int64:
		w.ws.WriteString(strconv.FormatFloat(float64(v), 'f', -1, 64))
	case string:
		w.writeString(v)
	default:
		return fmt.Errorf("unsupported json type")
	}
	return nil
}

func (w *Writer) writeKey(key string) error {
	w.writeString(key)
	w.ws.WriteRune(':')
	if !w.Compact {
		w.ws.WriteRune(' ')
	}
	return nil
}

func (w *Writer) writeString(value string) error {
	w.ws.WriteRune('"')
	w.ws.WriteString(value)
	w.ws.WriteRune('"')
	return nil
}

func (w *Writer) writePrefix() {
	if w.Compact || w.level == 0 {
		return
	}
	space := strings.Repeat(w.Indent, w.level)
	w.ws.WriteString(space)
}

func (w *Writer) writeNL() {
	if w.Compact {
		return
	}
	w.ws.WriteRune('\n')
}

func (w *Writer) enter() {
	w.level++
}

func (w *Writer) leave() {
	w.level--
}

func (w *Writer) reset() {
	w.level = 0
}

type mode int8

const (
	stdMode mode = 1 << iota
	json5Mode
)

func (m mode) isStd() bool {
	return m == stdMode
}

func (m mode) isExtended() bool {
	return m == json5Mode
}

type Parser struct {
	scan *Scanner
	curr Token
	peek Token

	mode
}

func Parse(r io.Reader) (any, error) {
	p := &Parser{
		scan: Scan(r, stdMode),
		mode: stdMode,
	}
	p.next()
	p.next()
	return p.Parse()
}

func Parse5(r io.Reader) (any, error) {
	p := &Parser{
		scan: Scan(r, json5Mode),
		mode: json5Mode,
	}
	p.next()
	p.next()
	return p.Parse()
}

func (p *Parser) Parse() (any, error) {
	return p.parse()
}

func (p *Parser) parse() (any, error) {
	switch p.curr.Type {
	case BegArr:
		return p.parseArray()
	case BegObj:
		return p.parseObject()
	case String:
		return p.parseString(), nil
	case Number:
		return p.parseNumber(), nil
	case Boolean:
		return p.parseBool(), nil
	case Null:
		return p.parseNull(), nil
	default:
		return nil, fmt.Errorf("syntax error")
	}
}

func (p *Parser) parseKey() (string, error) {
	switch {
	case p.is(String):
	case p.is(Ident) && p.mode.isExtended():
	default:
		return "", fmt.Errorf("syntax error: object key should be string")
	}
	key := p.curr.Literal
	p.next()
	if !p.is(Colon) {
		return "", fmt.Errorf("syntax error: missing ':'")
	}
	p.next()
	return key, nil
}

func (p *Parser) parseObject() (any, error) {
	p.next()
	obj := make(map[string]any)
	for !p.done() && !p.is(EndObj) {
		k, err := p.parseKey()
		if err != nil {
			return nil, err
		}
		a, err := p.parse()
		if err != nil {
			return nil, err
		}

		obj[k] = a
		switch {
		case p.is(Comma):
			p.next()
			if p.is(EndObj) && !p.mode.isExtended() {
				return nil, fmt.Errorf("syntax error: trailing comma not allowed")
			}
		case p.is(EndObj):
		default:
			return nil, fmt.Errorf("syntax error: expected ',' or '}'")
		}
	}
	if !p.is(EndObj) {
		return nil, fmt.Errorf("array: missing '}'")
	}
	p.next()
	return obj, nil
}

func (p *Parser) parseArray() (any, error) {
	p.next()
	var arr []any
	for !p.done() && !p.is(EndArr) {
		a, err := p.parse()
		if err != nil {
			return nil, err
		}
		arr = append(arr, a)
		switch {
		case p.is(Comma):
			p.next()
			if p.is(EndArr) && !p.mode.isExtended() {
				return nil, fmt.Errorf("syntax error: trailing comma not allowed")
			}
		case p.is(EndArr):
		default:
			return nil, fmt.Errorf("syntax error: expected ',' or ']'")
		}
	}
	if !p.is(EndArr) {
		return nil, fmt.Errorf("array: missing ']'")
	}
	p.next()
	return arr, nil
}

func (p *Parser) parseNumber() any {
	defer p.next()
	n, err := strconv.ParseFloat(p.curr.Literal, 64)
	if err != nil {
		n, _ := strconv.ParseInt(p.curr.Literal, 0, 64)
		return float64(n)
	}
	return n
}

func (p *Parser) parseBool() any {
	defer p.next()
	if p.curr.Literal == "true" {
		return true
	}
	return false
}

func (p *Parser) parseString() any {
	defer p.next()
	return p.curr.Literal
}

func (p *Parser) parseNull() any {
	defer p.next()
	return nil
}

func (p *Parser) done() bool {
	return p.is(EOF)
}

func (p *Parser) is(kind rune) bool {
	return p.curr.Type == kind
}

func (p *Parser) next() {
	p.curr = p.peek
	p.peek = p.scan.Scan()
}

type Token struct {
	Literal string
	Type    rune
}

func (t Token) String() string {
	var prefix string
	switch t.Type {
	case Transform:
		return "<transform>"
	case Doc:
		return "<document>"
	case Ternary:
		return "<ternary>"
	case Colon:
		return "<colon>"
	case BegGrp:
		return "<beg-grp>"
	case EndGrp:
		return "<end-grp>"
	case And:
		return "<and>"
	case Or:
		return "<or>"
	case In:
		return "<in>"
	case Add:
		return "<add>"
	case Sub:
		return "<subtract>"
	case Mul:
		return "<multiply>"
	case Div:
		return "<divide>"
	case Mod:
		return "<modulo>"
	case Eq:
		return "<equal>"
	case Ne:
		return "<not-equal>"
	case Lt:
		return "<lesser-than>"
	case Le:
		return "<lesser-eq>"
	case Gt:
		return "<greater-than>"
	case Ge:
		return "<greater-eq>"
	case Concat:
		return "<concat>"
	case Map:
		return "<map>"
	case Parent:
		return "<parent>"
	case Wildcard:
		return "<wildcard>"
	case Descent:
		return "<descend>"
	case Range:
		return "<range>"
	case EOF:
		return "<eof>"
	case BegArr:
		return "<beg-arr>"
	case EndArr:
		return "<end-arr>"
	case BegObj:
		return "<beg-obj>"
	case EndObj:
		return "<end-obj>"
	case Comma:
		return "<comma>"
	case Boolean:
		prefix = "boolean"
	case Null:
		return "<null>"
	case String:
		prefix = "string"
	case Number:
		prefix = "number"
	case Ident:
		prefix = "identifier"
	case Func:
		prefix = "function"
	case Comment:
		prefix = "comment"
	case Invalid:
		prefix = "invalid"
	}
	return fmt.Sprintf("%s(%s)", prefix, t.Literal)
}

const (
	EOF = -(1 + iota)
	BegArr
	EndArr
	BegObj
	EndObj
	Comma
	Colon
	Boolean
	Null
	String
	Number
	Ident
	Func
	Comment
	// query token
	Doc
	BegGrp
	EndGrp
	In
	And
	Or
	Add
	Sub
	Mul
	Div
	Mod
	Eq
	Ne
	Lt
	Le
	Gt
	Ge
	Concat
	Ternary
	Map
	Parent
	Wildcard
	Descent
	Range
	Transform
	// common
	Invalid
)

type Scanner struct {
	input io.RuneScanner
	char  rune
	mode

	str bytes.Buffer
}

func Scan(r io.Reader, mode mode) *Scanner {
	scan := Scanner{
		input: bufio.NewReader(r),
		mode:  mode,
	}
	scan.read()
	return &scan
}

func (s *Scanner) Scan() Token {
	defer s.str.Reset()
	s.skipBlank()

	var tok Token
	if s.done() {
		tok.Type = EOF
		return tok
	}
	switch {
	case s.mode.isExtended() && isComment(s.char, s.peek()):
		s.scanComment(&tok)
	case s.mode.isExtended() && isLetter(s.char):
		s.scanLiteral(&tok)
	case isLower(s.char):
		s.scanIdent(&tok)
	case isQuote(s.char) || (s.mode.isExtended() && isApos(s.char)):
		s.scanString(&tok)
	case isNumber(s.char) || s.char == '-':
		s.scanNumber(&tok)
	case s.mode.isExtended() && (s.char == '+' || s.char == '.'):
		s.scanNumber(&tok)
	case isDelim(s.char):
		s.scanDelimiter(&tok)
	default:
		tok.Type = Invalid
	}
	return tok
}

func (s *Scanner) scanLiteral(tok *Token) {
	for !s.done() && isAlpha(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	switch tok.Literal {
	case "true", "false":
		tok.Type = Boolean
	case "null":
		tok.Type = Null
	default:
		tok.Type = Ident
	}
}

func (s *Scanner) scanComment(tok *Token) {
	s.read()
	s.read()
	s.skipBlank()
	for !s.done() && !isNL(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	tok.Type = Comment
}

func (s *Scanner) scanIdent(tok *Token) {
	for !s.done() && isAlpha(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	switch tok.Literal {
	case "true", "false":
		tok.Type = Boolean
	case "null":
		tok.Type = Null
	default:
		tok.Type = Invalid
	}
}

func (s *Scanner) scanString(tok *Token) {
	quote := s.char
	s.read()
	for !s.done() && s.char != quote {
		if s.char == '\\' {
			s.read()
			if isNL(s.char) {
				s.write()
				s.read()
				continue
			}
			if ok := s.scanEscape(quote); !ok {
				tok.Type = Invalid
				return
			}
		}
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	tok.Type = String
	if s.char != quote {
		tok.Type = Invalid
	} else {
		s.read()
	}
}

func (s *Scanner) scanEscape(quote rune) bool {
	switch s.char {
	case quote:
		s.char = quote
	case '\\':
		s.char = '\\'
	case '/':
		s.char = '/'
	case 'b':
		s.char = '\b'
	case 'f':
		s.char = '\f'
	case 'n':
		s.char = '\n'
	case 'r':
		s.char = '\r'
	case 't':
		s.char = '\t'
	case 'u':
		s.read()
		buf := make([]rune, 4)
		for i := 1; i <= 4; i++ {
			if !isHex(s.char) {
				return false
			}
			buf[i-1] = s.char
			if i < 4 {
				s.read()
			}
		}
		char, _ := strconv.ParseInt(string(buf), 16, 32)
		s.char = rune(char)
	default:
		return false
	}
	return true
}

func (s *Scanner) scanHexa(tok *Token) {
	s.read()
	s.read()
	s.writeRune('0')
	s.writeRune('x')
	for !s.done() && isHex(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
}

func (s *Scanner) scanNumber(tok *Token) {
	tok.Type = Number
	if s.mode.isExtended() && s.char == '0' && s.peek() == 'x' {
		s.scanHexa(tok)
		return
	}
	if s.mode.isExtended() && s.char == '.' {
		s.writeRune('0')
		s.writeRune('.')
		s.read()
	}
	if s.char == '-' || s.char == '+' {
		s.write()
		s.read()
	}
	for !s.done() && isNumber(s.char) {
		s.write()
		s.read()
	}
	tok.Literal = s.str.String()
	if s.char == '.' {
		s.write()
		s.read()
		if !isNumber(s.char) {
			if !s.mode.isExtended() {
				tok.Type = Invalid
			}
			return
		}
		for !s.done() && isNumber(s.char) {
			s.write()
			s.read()
		}
		tok.Literal = s.str.String()
	}
	if s.char == 'e' || s.char == 'E' {
		s.write()
		s.read()
		if s.char == '-' || s.char == '+' {
			s.write()
			s.read()
		}
		if !isNumber(s.char) {
			tok.Type = Invalid
			return
		}
		for !s.done() && isNumber(s.char) {
			s.write()
			s.read()
		}
		tok.Literal = s.str.String()
	}
}

func (s *Scanner) scanDelimiter(tok *Token) {
	switch s.char {
	case '[':
		tok.Type = BegArr
	case ']':
		tok.Type = EndArr
	case '{':
		tok.Type = BegObj
	case '}':
		tok.Type = EndObj
	case ',':
		tok.Type = Comma
	case ':':
		tok.Type = Colon
	default:
		tok.Type = Invalid
	}
	if tok.Type != Invalid {
		s.read()
	}
}

func (s *Scanner) writeRune(c rune) {
	s.str.WriteRune(c)
}

func (s *Scanner) write() {
	s.writeRune(s.char)
}

func (s *Scanner) read() {
	char, _, err := s.input.ReadRune()
	if errors.Is(err, io.EOF) {
		char = utf8.RuneError
	}
	s.char = char
}

func (s *Scanner) peek() rune {
	defer s.input.UnreadRune()
	r, _, _ := s.input.ReadRune()
	return r
}

func (s *Scanner) done() bool {
	return s.char == utf8.RuneError
}

func (s *Scanner) skipBlank() {
	for !s.done() && unicode.IsSpace(s.char) {
		s.read()
	}
}

func isComment(c, k rune) bool {
	return c == '/' && c == k
}

func isHex(c rune) bool {
	return isNumber(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isNumber(c rune) bool {
	return c >= '0' && c <= '9'
}

func isLower(c rune) bool {
	return c >= 'a' && c <= 'z'
}

func isUpper(c rune) bool {
	return c >= 'A' && c <= 'Z'
}

func isLetter(c rune) bool {
	return isLower(c) || isUpper(c)
}

func isAlpha(c rune) bool {
	return isLetter(c) || isNumber(c) || c == '_'
}

func isApos(c rune) bool {
	return c == '\''
}

func isQuote(c rune) bool {
	return c == '"'
}

func isBackQuote(c rune) bool {
	return c == '`'
}

func isDelim(c rune) bool {
	return c == '{' || c == '}' || c == '[' || c == ']' || c == ',' || c == ':'
}

func isNL(c rune) bool {
	return c == '\n' || c == '\r'
}

func isOperator(c rune) bool {
	return c == '!' || c == '=' || c == '<' || c == '>' ||
		c == '&' || c == '*' || c == '/' || c == '%' || c == '-' ||
		c == '+' || c == '.' || c == '?' || c == ':'
}

func isTransform(c rune) bool {
	return c == '|'
}

func isDollar(c rune) bool {
	return c == '$'
}