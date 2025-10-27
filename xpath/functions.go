package xpath

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/midbel/codecs/environ"
	"github.com/midbel/codecs/xml"
)

func init() {
	builtinEnv = defaultFuncset()
}

type BuiltinFunc func(Context, []Expr) (Sequence, error)

func (fn BuiltinFunc) Call(ctx Context, args []Expr) (Sequence, error) {
	return fn(ctx, args)
}

var builtinEnv environ.Environ[BuiltinFunc]

func DefaultBuiltin() environ.Environ[BuiltinFunc] {
	c, ok := builtinEnv.(interface {
		Clone() environ.Environ[BuiltinFunc]
	})
	if ok {
		return c.Clone()
	}
	return builtinEnv
}

type registeredBuiltin struct {
	xml.QName
	MinArg int
	MaxArg int
	Func   BuiltinFunc
}

func registerFunc(name, space string, fn BuiltinFunc) registeredBuiltin {
	qn := xml.QualifiedName(name, space)
	if uri, ok := defaultNS[space]; ok {
		qn.Uri = uri
	} else {
		qn.Uri = angleNS[space]
	}

	return registeredBuiltin{
		QName: qn,
		Func:  fn,
	}
}

type funcset struct {
	environ.Environ[BuiltinFunc]
}

func defaultFuncset() environ.Environ[BuiltinFunc] {
	set := funcset{
		Environ: environ.Empty[BuiltinFunc](),
	}
	set.enableFuncSet(builtins)
	return &set
}

func (f *funcset) Clone() environ.Environ[BuiltinFunc] {
	c, ok := f.Environ.(interface {
		Clone() environ.Environ[BuiltinFunc]
	})
	if !ok {
		return f
	}
	var x funcset
	x.Environ = c.Clone()
	return &x
}

func (f *funcset) EnableProcess() {
	f.enableFuncSet(processFuncs)
}

func (f *funcset) EnableHTTP() {
	f.enableFuncSet(httpFuncs)
}

func (f *funcset) EnableFile() {
	f.enableFuncSet(fileFuncs)
}

func (f *funcset) EnableBinary() {
	f.enableFuncSet(binaryFuncs)
}

func (f *funcset) EnableCrypto() {
	f.enableFuncSet(cryptoFuncs)
}

func (f *funcset) EnableAngle() {
	f.enableFuncSet(angleFuncs)
	f.enableFuncSet(angleStringFuncs)
}

func (f *funcset) enableFuncSet(set []registeredBuiltin) {
	for _, b := range set {
		f.Define(b.ExpandedName(), b.Func)
	}
}

var builtins = []registeredBuiltin{
	registerFunc("namespace-uri", "fn", callNamespaceUri),
	registerFunc("uri-collection", "fn", callUriCollection),
	// node and document functiosn
	registerFunc("name", "fn", callName),
	registerFunc("local-name", "fn", callLocalName),
	registerFunc("root", "fn", callRoot),
	registerFunc("path", "fn", callPath),
	registerFunc("has-children", "fn", callHasChildren),
	registerFunc("innermost", "fn", callInnermost),
	registerFunc("outermost", "fn", callOutermost),
	registerFunc("doc", "fn", callDoc),
	registerFunc("collection", "fn", callCollection),
	registerFunc("position", "fn", callPosition),
	registerFunc("last", "fn", callLast),
	// string functions
	registerFunc("string", "fn", callString),
	registerFunc("compare", "fn", callCompare),
	registerFunc("concat", "fn", callConcat),
	registerFunc("string-join", "fn", callStringJoin),
	registerFunc("substring", "fn", callSubstring),
	registerFunc("string-length", "fn", callStringLength),
	registerFunc("normalize-space", "fn", callNormalizeSpace),
	registerFunc("upper-case", "fn", callUppercase),
	registerFunc("lower-case", "fn", callLowercase),
	registerFunc("translate", "fn", callTranslate),
	registerFunc("contains", "fn", callContains),
	registerFunc("starts-with", "fn", callStartsWith),
	registerFunc("ends-with", "fn", callEndsWith),
	registerFunc("substring-before", "fn", callSubstringBefore),
	registerFunc("substring-after", "fn", callSubstringAfter),
	registerFunc("replace", "fn", callReplace),
	registerFunc("matches", "fn", callMatches),
	registerFunc("tokenize", "fn", callTokenize),
	// sequence function
	registerFunc("empty", "fn", callEmpty),
	registerFunc("tail", "fn", callTail),
	registerFunc("head", "fn", callHead),
	registerFunc("exists", "fn", callExists),
	registerFunc("insert-before", "fn", callXYZ),
	registerFunc("remove", "fn", callXYZ),
	registerFunc("reverse", "fn", callReverse),
	registerFunc("subsequence", "fn", callXYZ),
	registerFunc("unordered", "fn", callXYZ),
	registerFunc("zero-or-one", "fn", callZeroOrOne),
	registerFunc("one-or-more", "fn", callOneOrMore),
	registerFunc("exactly-one", "fn", callExactlyOne),
	registerFunc("distinct-values", "fn", callDistinctValues),
	// boolean functions
	registerFunc("true", "fn", callTrue),
	registerFunc("false", "fn", callFalse),
	registerFunc("boolean", "fn", callBoolean),
	registerFunc("not", "fn", callNot),
	// number + aggregate functions
	registerFunc("number", "fn", callNumber),
	registerFunc("round", "fn", callRound),
	registerFunc("floor", "fn", callFloor),
	registerFunc("ceiling", "fn", callCeil),
	registerFunc("abs", "fn", callAbs),
	registerFunc("sum", "fn", callSum),
	registerFunc("count", "fn", callCount),
	registerFunc("avg", "fn", callAvg),
	registerFunc("min", "fn", callMin),
	registerFunc("max", "fn", callMax),
	registerFunc("format-number", "fn", callFormatNumber),
	registerFunc("format-integer", "fn", callFormatInteger),
	// date functions
	registerFunc("dateTime", "fn", callDateTime),
	registerFunc("year-from-dateTime", "fn", callYearFromDateTime),
	registerFunc("year-from-date", "fn", callYearFromDate),
	registerFunc("month-from-dateTime", "fn", callMonthFromDateTime),
	registerFunc("month-from-date", "fn", callMonthFromDate),
	registerFunc("day-from-dateTime", "fn", callDayFromDateTime),
	registerFunc("day-from-date", "fn", callDayFromDate),
	registerFunc("hours-from-dateTime", "fn", callHoursFromDateTime),
	registerFunc("minutes-from-dateTime", "fn", callMinutesFromDateTime),
	registerFunc("seconds-from-dateTime", "fn", callSecondsFromDateTime),
	registerFunc("format-date", "fn", callFormatDate),
	registerFunc("format-dateTime", "fn", callFormatDateTime),
	registerFunc("current-date", "fn", callCurrentDate),
	registerFunc("current-dateTime", "fn", callCurrentDatetime),
	// function related functions
	registerFunc("function-arity", "fn", callXYZ),
	registerFunc("function-name", "fn", callXYZ),
	registerFunc("function-lookup", "fn", callXYZ),
	// array functions
	registerFunc("append", "array", callXYZ),
	registerFunc("filter", "array", callXYZ),
	registerFunc("flatten", "array", callXYZ),
	registerFunc("fold-left", "array", callXYZ),
	registerFunc("fold-right", "array", callXYZ),
	registerFunc("for-each", "array", callXYZ),
	registerFunc("for-each-pair", "array", callXYZ),
	registerFunc("get", "array", callXYZ),
	registerFunc("head", "array", callXYZ),
	registerFunc("insert-before", "array", callXYZ),
	registerFunc("insert-after", "array", callXYZ),
	registerFunc("join", "array", callXYZ),
	registerFunc("put", "array", callXYZ),
	registerFunc("remove", "array", callXYZ),
	registerFunc("reverse", "array", callXYZ),
	registerFunc("size", "array", callXYZ),
	registerFunc("sort", "array", callXYZ),
	registerFunc("subarray", "array", callXYZ),
	registerFunc("tail", "array", callXYZ),
	// map functions
	registerFunc("contains", "map", callXYZ),
	registerFunc("entry", "map", callXYZ),
	registerFunc("find", "map", callXYZ),
	registerFunc("for-each", "map", callXYZ),
	registerFunc("get", "map", callXYZ),
	registerFunc("keys", "map", callXYZ),
	registerFunc("merge", "map", callXYZ),
	registerFunc("put", "map", callXYZ),
	registerFunc("remove", "map", callXYZ),
	registerFunc("size", "map", callXYZ),
	// constructor functions
	registerFunc("string", "xs", callConstructor(xsString)),
	registerFunc("decimal", "xs", callConstructor(xsDecimal)),
	registerFunc("integer", "xs", callConstructor(xsInteger)),
	registerFunc("boolean", "xs", callConstructor(xsBool)),
	registerFunc("dateTime", "xs", callConstructor(xsDateTime)),
	registerFunc("date", "xs", callConstructor(xsDate)),
}

var fileFuncs = []registeredBuiltin{
	registerFunc("read-file", "file", callReadFile),
	registerFunc("write-file", "file", callWriteFile),
	registerFunc("exists", "file", callFileExists),
	registerFunc("delete", "file", callDeleteFile),
	registerFunc("list", "file", callListDir),
}

var angleFuncs = []registeredBuiltin{
	registerFunc("coalesce", "agl", callXYZ),
}

var angleStringFuncs = []registeredBuiltin{
	registerFunc("string-indexof", "aglstr", callStringIndexOf),
	registerFunc("string-reverse", "aglstr", callStringReverse),
}

var envFuncs = []registeredBuiltin{
	registerFunc("get", "env", callXYZ),
}

var imgFuncs = []registeredBuiltin{
	registerFunc("resize-png", "image", callXYZ),
	registerFunc("resize-jpg", "image", callXYZ),
}

var binaryFuncs = []registeredBuiltin{
	registerFunc("encode", "binary", callXYZ),
	registerFunc("decode", "binary", callXYZ),
	registerFunc("concat", "binary", callXYZ),
	registerFunc("substring", "binary", callXYZ),
}

var cryptoFuncs = []registeredBuiltin{
	registerFunc("hash", "crypto", callHash),
	registerFunc("hmac", "crypto", callHmac),
	registerFunc("encrypt", "crypto", callXYZ),
	registerFunc("decrypt", "crypto", callXYZ),
	registerFunc("sign", "crypto", callXYZ),
	registerFunc("verify", "crypto", callXYZ),
}

var httpFuncs = []registeredBuiltin{
	registerFunc("send", "http", callXYZ),
	registerFunc("get", "http", callHttpGet),
	registerFunc("post", "http", callHttpPost),
}

var archiveFuncs = []registeredBuiltin{
	registerFunc("entries", "archive", callXYZ),
	registerFunc("extract", "archive", callXYZ),
	registerFunc("create", "archive", callXYZ),
}

var processFuncs = []registeredBuiltin{
	registerFunc("execute", "process", callXYZ),
	registerFunc("wait", "process", callXYZ),
	registerFunc("stdout", "process", callXYZ),
	registerFunc("stderr", "process", callXYZ),
	registerFunc("exit-code", "process", callXYZ),
}

func callXYZ(ctx Context, args []Expr) (Sequence, error) {
	return nil, ErrImplemented
}

func callHash(ctx Context, args []Expr) (Sequence, error) {
	if len(args) < 2 {
		return nil, ErrArgument
	}
	input, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	alg, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	var res []byte
	switch in := []byte(input); alg {
	case "MD5":
		tmp := md5.Sum(in)
		res = tmp[:]
	case "SHA-1":
		tmp := sha1.Sum(in)
		res = tmp[:]
	case "SHA-256":
		tmp := sha256.Sum256(in)
		res = tmp[:]
	case "SHA-512":
		tmp := sha512.Sum512(in)
		res = tmp[:]
	default:
		return nil, fmt.Errorf("%s unsupported algorithm", alg)
	}
	var str string
	if len(args) >= 3 {
		encoding, err := getStringFromExpr(args[2], ctx)
		if err != nil {
			return nil, err
		}
		switch encoding {
		case "base64":
			str = base64.StdEncoding.EncodeToString(res)
		case "hex":
			str = hex.EncodeToString(res)
		default:
			return nil, fmt.Errorf("%s: unsupported output encoding", encoding)
		}
	}
	return Singleton(str), nil
}

func callHmac(ctx Context, args []Expr) (Sequence, error) {
	if len(args) < 3 {
		return nil, ErrArgument
	}
	msg, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	key, err := getStringFromExpr(args[1], ctx)
	if err != nil {
		return nil, err
	}
	alg, err := getStringFromExpr(args[2], ctx)
	if err != nil {
		return nil, err
	}
	var mac hash.Hash
	switch key := []byte(key); alg {
	case "MD5":
		mac = hmac.New(md5.New, key)
	case "SHA-1":
		mac = hmac.New(sha1.New, key)
	case "SHA-256":
		mac = hmac.New(sha256.New, key)
	case "SHA-384":
		mac = hmac.New(sha512.New384, key)
	case "SHA-512":
		mac = hmac.New(sha512.New, key)
	default:
		return nil, fmt.Errorf("%s: unsupported algorithm", alg)
	}
	mac.Write([]byte(msg))
	var (
		res = mac.Sum(nil)
		str string
	)
	if len(args) >= 4 {
		encoding, err := getStringFromExpr(args[3], ctx)
		if err != nil {
			return nil, err
		}
		switch encoding {
		case "base64":
			str = base64.StdEncoding.EncodeToString(res)
		case "hex":
			str = hex.EncodeToString(res)
		default:
			return nil, fmt.Errorf("%s: unsupported output encoding", encoding)
		}
	}
	return Singleton(str), nil
}

func callHttpGet(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	url, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var body bytes.Buffer
	if _, err := io.Copy(&body, res.Body); err != nil {
		return nil, err
	}
	return Singleton(body.String()), nil
}

func callHttpPost(ctx Context, args []Expr) (Sequence, error) {
	if len(args) < 2 {
		return nil, ErrArgument
	}
	url, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	content, err := getStringFromExpr(args[1], ctx)
	if err != nil {
		return nil, err
	}
	mime := "text/xml"
	if len(args) >= 3 {
		mime, _ = getStringFromExpr(args[2], ctx)
	}

	res, err := http.Post(url, mime, bytes.NewBufferString(content))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var body bytes.Buffer
	if _, err := io.Copy(&body, res.Body); err != nil {
		return nil, err
	}
	return Singleton(body.String()), nil
}

func callReadFile(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	file, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	buf, err := os.ReadFile(file)
	if err == nil {
		return Singleton(string(buf)), nil
	}
	return nil, err
}

func callWriteFile(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 2 {
		return nil, ErrArgument
	}
	file, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	content, err := getStringFromExpr(args[1], ctx)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(file, []byte(content), 0o644)
	return nil, err
}

func callFileExists(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	file, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	_, err = os.Stat(file)
	return Singleton(err == nil), err
}

func callDeleteFile(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	file, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	err = os.Remove(file)
	return nil, err
}

func callListDir(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	file, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	es, err := os.ReadDir(file)
	if err != nil {
		return nil, err
	}
	var list []Item
	for i := range es {
		list = append(list, createLiteral(es[i].Name()))
	}
	return list, nil
}

func callNamespaceUri(ctx Context, args []Expr) (Sequence, error) {
	if len(args) == 0 {
		a := NewValueFromNode(ctx.Node)
		return callNamespaceUri(ctx, []Expr{a})
	}
	if len(args) != 1 {
		return nil, ErrArgument
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if items.Len() == 0 {
		return Singleton(""), nil
	}
	n, ok := items[0].Node().(*xml.Element)
	if !ok {
		return nil, nil
	}
	return Singleton(n.QName.Uri), nil
}

func callUriCollection(ctx Context, args []Expr) (Sequence, error) {
	if len(args) == 0 {
		return ctx.DefaultUriCollection(), nil
	}
	uri, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	return ctx.UriCollection(uri)
}

func callCollection(ctx Context, args []Expr) (Sequence, error) {
	return nil, ErrImplemented
}

func callRound(ctx Context, args []Expr) (Sequence, error) {
	if len(args) < 1 && len(args) > 2 {
		return nil, ErrArgument
	}
	val, err := getFloatFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	return Singleton(math.Round(val)), nil
}

func callFloor(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	val, err := getFloatFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	return Singleton(math.Floor(val)), nil
}

func callCeil(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	val, err := getFloatFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	return Singleton(math.Ceil(val)), nil
}

func callAbs(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	val, err := getFloatFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	return Singleton(math.Abs(val)), nil
}

func callFormatNumber(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 2 {
		return nil, ErrArgument
	}
	val, err := getFloatFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	// picture, err := getStringFromExpr(args[1], ctx)
	// if err != nil {
	// 	return nil, err
	// }
	// res, err := formatNumber(val, picture)
	return Singleton(val), nil
}

func callFormatInteger(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 2 {
		return nil, ErrArgument
	}
	val, err := getIntFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	picture, err := getStringFromExpr(args[1], ctx)
	if err != nil {
		return nil, err
	}
	res, err := formatInteger(val, picture)
	return Singleton(res), err
}

func callNumber(ctx Context, args []Expr) (Sequence, error) {
	var (
		str string
		err error
	)
	if len(args) >= 1 {
		str, err = getStringFromExpr(args[0], ctx)
		if err != nil {
			return nil, err
		}
	} else {
		str = ctx.Value()
	}
	val, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return nil, err
	}
	return Singleton(val), nil
}

func callExists(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	return Singleton(!items.Empty()), nil
}

func callDistinctValues(ctx Context, args []Expr) (Sequence, error) {
	return nil, nil
}

func callEmpty(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	return Singleton(items.Empty()), nil
}

func callHead(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if items.Empty() {
		return nil, nil
	}
	return createSingle(items[0]), nil
}

func callTail(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if items.Empty() {
		return nil, nil
	}
	return items[1:], nil
}

func callReverse(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	slices.Reverse(items)
	return items, nil
}

func callSum(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
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
	return Singleton(result), nil
}

func callAvg(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, ErrArgument
	}
	var result float64
	for _, n := range items {
		v, err := strconv.ParseFloat(n.Node().Value(), 64)
		if err != nil {
			return nil, err
		}
		result += v
	}
	return Singleton(result / float64(len(items))), nil
}

func callCount(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	return Singleton(float64(len(items))), nil
}

func callMin(ctx Context, args []Expr) (Sequence, error) {
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	var res any
	if items.Every(isFloat) {
		list, _ := convert[float64](items, toFloat)
		res = lowestValue(list)
	} else {
		list, _ := convert[string](items, toString)
		res = lowestValue(list)
	}
	return Singleton(res), nil
}

func callMax(ctx Context, args []Expr) (Sequence, error) {
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	var res any
	if items.Every(isFloat) {
		list, _ := convert[float64](items, toFloat)
		res = greatestValue(list)
	} else {
		list, _ := convert[string](items, toString)
		res = greatestValue(list)
	}
	return Singleton(res), nil
}

func callZeroOrOne(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
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

func callOneOrMore(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
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

func callExactlyOne(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
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

func callPosition(ctx Context, args []Expr) (Sequence, error) {
	return Singleton(float64(ctx.Index)), nil
}

func callLast(ctx Context, args []Expr) (Sequence, error) {
	return Singleton(float64(ctx.Size)), nil
}

func callCurrentDate(ctx Context, args []Expr) (Sequence, error) {
	return callCurrentDatetime(ctx, args)
}

func callCurrentDatetime(ctx Context, args []Expr) (Sequence, error) {
	return Singleton(time.Now()), nil
}

func callDate(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	str, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	v, err := time.Parse("2006-01-02", str)
	if err != nil {
		return nil, ErrCast
	}
	return Singleton(v), nil
}

func callDateTime(ctx Context, args []Expr) (Sequence, error) {
	return nil, ErrImplemented
}

func callYearFromDateTime(ctx Context, args []Expr) (Sequence, error) {
	return nil, ErrImplemented
}

func callYearFromDate(ctx Context, args []Expr) (Sequence, error) {
	return nil, ErrImplemented
}

func callMonthFromDateTime(ctx Context, args []Expr) (Sequence, error) {
	return nil, ErrImplemented
}

func callMonthFromDate(ctx Context, args []Expr) (Sequence, error) {
	return nil, ErrImplemented
}

func callDayFromDateTime(ctx Context, args []Expr) (Sequence, error) {
	return nil, ErrImplemented
}

func callDayFromDate(ctx Context, args []Expr) (Sequence, error) {
	return nil, ErrImplemented
}

func callHoursFromDateTime(ctx Context, args []Expr) (Sequence, error) {
	return nil, ErrImplemented
}

func callMinutesFromDateTime(ctx Context, args []Expr) (Sequence, error) {
	return nil, ErrImplemented
}

func callSecondsFromDateTime(ctx Context, args []Expr) (Sequence, error) {
	return nil, ErrImplemented
}

func callFormatDate(ctx Context, args []Expr) (Sequence, error) {
	return nil, nil
}

func callFormatDateTime(ctx Context, args []Expr) (Sequence, error) {
	return nil, nil
}

func callStringReverse(ctx Context, args []Expr) (Sequence, error) {
	if len(args) == 0 {
		args = append(args, NewValueFromNode(ctx.Node))
		return callStringReverse(ctx, args)
	}
	str, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	chars := []rune(str)
	slices.Reverse(chars)
	return Singleton(string(chars)), nil
}

func callStringIndexOf(ctx Context, args []Expr) (Sequence, error) {
	if len(args) < 2 {
		args = append([]Expr{NewValueFromNode(ctx.Node)}, args...)
		return callStringIndexOf(ctx, args)
	}
	str, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	needle, err := getStringFromExpr(args[1], ctx)
	if err != nil {
		return nil, err
	}
	ix := strings.Index(str, needle) + 1
	return Singleton(float64(ix)), nil
}

func callString(ctx Context, args []Expr) (Sequence, error) {
	if len(args) == 0 {
		return Singleton(ctx.Value()), nil
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if items.Empty() {
		return Singleton(""), nil
	}
	if !items[0].Atomic() {
		ctx.Node = items[0].Node()
		ctx.Size = 1
		ctx.Index = 1
		return callString(ctx, nil)
	}
	str, err := toString(items[0].Value())
	if err != nil {
		return nil, err
	}
	return Singleton(str), nil
}

func callCompare(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 2 {
		return nil, ErrArgument
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
	return Singleton(float64(cmp)), nil
}

func callConcat(ctx Context, args []Expr) (Sequence, error) {
	if len(args) < 2 {
		return nil, ErrArgument
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	var list []string
	for _, i := range items {
		str, err := toString(i.Value())
		if err != nil {
			return nil, err
		}
		list = append(list, str)
	}
	str := strings.Join(list, "")
	return Singleton(str), nil
}

func callStringJoin(ctx Context, args []Expr) (Sequence, error) {
	if len(args) < 1 {
		return nil, ErrArgument
	}
	items, err := expandArgs(ctx, args[:1])
	if err != nil {
		return nil, err
	}
	list, err := convert(items, toString)
	if err != nil {
		return nil, err
	}
	var sep string
	if len(args) >= 2 {
		sep, err = getStringFromExpr(args[1], ctx)
		if err != nil {
			return nil, err
		}
	}
	str := strings.Join(list, sep)
	return Singleton(str), nil
}

func callSubstring(ctx Context, args []Expr) (Sequence, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, ErrArgument
	}
	str, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	if str == "" {
		return Singleton(""), nil
	}
	beg, err := getFloatFromExpr(args[1], ctx)
	if err != nil {
		return nil, err
	}
	beg -= 1

	var size float64
	if len(args) == 3 {
		size, err = getFloatFromExpr(args[2], ctx)
		if err != nil {
			return nil, err
		}
		size = size
	} else {
		size = float64(len(str))
	}
	if beg < 0 {
		size += beg
		beg = 0
	}
	if z := len(str); int(beg+size) >= z {
		size = float64(z) - beg
	}
	str = str[int(beg) : int(beg)+int(size)]
	return Singleton(str), nil
}

func callStringLength(ctx Context, args []Expr) (Sequence, error) {
	if len(args) == 0 {
		str := ctx.Value()
		return Singleton(float64(len(str))), nil
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return Singleton(0.0), nil
	}
	if !items[0].Atomic() {
		ctx.Node = items[0].Node()
		ctx.Size = 1
		ctx.Index = 1
		return callStringLength(ctx, nil)
	}
	str, ok := items[0].Value().(string)
	if !ok {
		return nil, ErrType
	}
	return Singleton(float64(len(str))), nil
}

func callNormalizeSpace(ctx Context, args []Expr) (Sequence, error) {
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
		err = ErrArgument
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
	return Singleton(strings.Map(clear, str)), nil
}

func callUppercase(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	str, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	return Singleton(strings.ToUpper(str)), nil
}

func callLowercase(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	str, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	return Singleton(strings.ToLower(str)), nil
}

func callReplace(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 3 {
		return nil, ErrArgument
	}
	str, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	src, err := getStringFromExpr(args[1], ctx)
	if err != nil {
		return nil, err
	}
	dst, err := getStringFromExpr(args[2], ctx)
	if err != nil {
		return nil, err
	}
	str = strings.ReplaceAll(str, src, dst)
	return Singleton(str), nil
}

func callTranslate(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 3 {
		return nil, ErrArgument
	}
	str, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	src, err := getStringFromExpr(args[1], ctx)
	if err != nil {
		return nil, err
	}
	dst, err := getStringFromExpr(args[2], ctx)
	if err != nil {
		return nil, err
	}
	set := []rune(dst)
	str = strings.Map(func(r rune) rune {
		ix := strings.IndexRune(src, r)
		if ix < 0 {
			return r
		}
		if ix < len(set) {
			return set[ix]
		}
		return -1
	}, str)
	return Singleton(str), nil
}

func callContains(ctx Context, args []Expr) (Sequence, error) {
	if len(args) < 1 {
		return nil, ErrArgument
	}
	if len(args) == 1 {
		list := []Expr{NewValueFromNode(ctx.Node)}
		return callContains(ctx, append(list, args...))
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
	return Singleton(res), nil
}

func callStartsWith(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 2 {
		return nil, ErrArgument
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
	return Singleton(res), nil
}

func callEndsWith(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 2 {
		return nil, ErrArgument
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
	return Singleton(res), nil
}

func callTokenize(ctx Context, args []Expr) (Sequence, error) {
	if len(args) > 2 {
		return nil, ErrArgument
	}
	fst, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	snd := " "
	if len(args) == 2 {
		snd, err = getStringFromExpr(args[1], ctx)
		if err != nil {
			return nil, err
		}
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

func callMatches(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 2 {
		return nil, ErrArgument
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
	return Singleton(ok), err
}

func callSubstringAfter(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 2 {
		return nil, ErrArgument
	}
	fst, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	snd, err := getStringFromExpr(args[1], ctx)
	if err != nil {
		return nil, err
	}
	_, str, ok := strings.Cut(fst, snd)
	if !ok {
		return Singleton(""), nil
	}
	return Singleton(str), nil
}

func callSubstringBefore(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 2 {
		return nil, ErrArgument
	}
	fst, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	snd, err := getStringFromExpr(args[1], ctx)
	if err != nil {
		return nil, err
	}
	str, _, ok := strings.Cut(fst, snd)
	if !ok {
		return Singleton(""), nil
	}
	return Singleton(str), nil
}

func callName(ctx Context, args []Expr) (Sequence, error) {
	if len(args) == 0 {
		n := ctx.QualifiedName()
		return Singleton(n), nil
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return Singleton(""), nil
	}
	n, ok := items[0].(nodeItem)
	if !ok {
		return nil, ErrType
	}
	return Singleton(n.Node().QualifiedName()), nil
}

func callLocalName(ctx Context, args []Expr) (Sequence, error) {
	if len(args) == 0 {
		return Singleton(ctx.LocalName()), nil
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return Singleton(""), nil
	}
	n, ok := items[0].(nodeItem)
	if !ok {
		return nil, ErrType
	}
	return Singleton(n.Node().LocalName()), nil
}

func callRoot(ctx Context, args []Expr) (Sequence, error) {
	var get func(xml.Node) xml.Node

	get = func(n xml.Node) xml.Node {
		p := n.Parent()
		if p == nil {
			return n
		}
		return get(p)
	}
	if len(args) == 0 {
		n := get(ctx.Node)
		return Singleton(n), nil
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	n, ok := items[0].(nodeItem)
	if !ok {
		return nil, ErrType
	}
	root := get(n.Node())
	return Singleton(root), nil
}

func callPath(ctx Context, args []Expr) (Sequence, error) {
	var get func(n xml.Node) []string

	get = func(n xml.Node) []string {
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
		return Singleton(strings.Join(list, "/")), nil
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	n, ok := items[0].(nodeItem)
	if !ok {
		return nil, ErrType
	}
	ctx.Node = n.Node()
	ctx.Size = 1
	ctx.Index = 1
	return callPath(ctx, nil)
}

func callHasChildren(ctx Context, args []Expr) (Sequence, error) {
	if len(args) == 0 {
		nodes := ctx.Nodes()
		return Singleton(len(nodes) > 0), nil
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	n, ok := items[0].(nodeItem)
	if !ok {
		return nil, ErrType
	}
	ctx.Node = n.Node()
	ctx.Size = 1
	ctx.Index = 1
	return callHasChildren(ctx, nil)
}

func callInnermost(ctx Context, args []Expr) (Sequence, error) {
	return nil, ErrImplemented
}

func callOutermost(ctx Context, args []Expr) (Sequence, error) {
	return nil, ErrImplemented
}

func callBoolean(ctx Context, args []Expr) (Sequence, error) {
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if items.Empty() {
		return callFalse(ctx, args)
	}
	return Singleton(items[0].True()), nil
}

func callNot(ctx Context, args []Expr) (Sequence, error) {
	items, err := callBoolean(ctx, args)
	if err != nil {
		return nil, err
	}
	value, ok := items[0].Value().(bool)
	if !ok {
		return nil, ErrType
	}
	return Singleton(!value), nil
}

func callTrue(_ Context, _ []Expr) (Sequence, error) {
	return Singleton(true), nil
}

func callFalse(_ Context, _ []Expr) (Sequence, error) {
	return Singleton(false), nil
}

func callDoc(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	file, err := getStringFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	p := xml.NewParser(r)
	n, err := p.Parse()
	if err != nil {
		return nil, err
	}
	return Singleton(n), nil
}

func callConstructor(xt XdmType) BuiltinFunc {
	return func(ctx Context, args []Expr) (Sequence, error) {
		if len(args) != 1 {
			return nil, ErrArgument
		}
		return xt.Cast(args[0])
	}
}

func getFloatFromExpr(expr Expr, ctx Context) (float64, error) {
	items, err := expr.find(ctx)
	if err != nil || !items.Singleton() {
		return math.NaN(), err
	}
	return toFloat(items[0].Value())
}

func getIntFromExpr(expr Expr, ctx Context) (int64, error) {
	items, err := expr.find(ctx)
	if err != nil || !items.Singleton() {
		return 0, err
	}
	return toInt(items[0].Value())
}

func getStringFromExpr(expr Expr, ctx Context) (string, error) {
	items, err := expr.find(ctx)
	if err != nil {
		return "", err
	}
	if items.Empty() {
		return "", nil
	}

	switch v := items.First().(type) {
	case literalItem:
		str, ok := v.Value().(string)
		if !ok {
			return "", ErrType
		}
		return str, nil
	case nodeItem:
		return v.Node().Value(), nil
	default:
		return "", ErrType
	}
}

func getBooleanFromItem(item Item) (bool, error) {
	return item.True(), nil
}

func expandArgs(ctx Context, args []Expr) (Sequence, error) {
	var list Sequence
	for _, a := range args {
		is, err := a.find(ctx)
		if err != nil {
			return nil, err
		}
		list.Concat(is)
	}
	return list, nil
}
