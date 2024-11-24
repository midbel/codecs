package json

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"time"
)

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
	val, ok := res.(bool)
	if !ok {
		return nil, errType
	}
	return !val, nil
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
		return "unknown", nil
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
