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
	return registeredBuiltin{
		QName: xml.QualifiedName(name, space),
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

func (f *funcset) enableFuncSet(set []registeredBuiltin) {
	for _, b := range set {
		f.Define(b.QualifiedName(), b.Func)
	}
}

var builtins = []registeredBuiltin{
	registerFunc("true", "", callTrue),
	registerFunc("true", "fn", callTrue),
	registerFunc("false", "", callFalse),
	registerFunc("false", "fn", callFalse),
	registerFunc("boolean", "", callBoolean),
	registerFunc("boolean", "fn", callBoolean),
	registerFunc("not", "", callNot),
	registerFunc("not", "fn", callNot),
	registerFunc("name", "", callName),
	registerFunc("name", "fn", callName),
	registerFunc("local-name", "", callLocalName),
	registerFunc("local-name", "fn", callLocalName),
	registerFunc("root", "", callRoot),
	registerFunc("root", "fn", callRoot),
	registerFunc("path", "", callPath),
	registerFunc("path", "fn", callPath),
	registerFunc("has-children", "", callHasChildren),
	registerFunc("has-children", "fn", callHasChildren),
	registerFunc("innermost", "", callInnermost),
	registerFunc("innermost", "fn", callInnermost),
	registerFunc("outermost", "", callOutermost),
	registerFunc("outermost", "fn", callOutermost),
	registerFunc("string", "", callString),
	registerFunc("string", "fn", callString),
	registerFunc("compare", "", callCompare),
	registerFunc("compare", "fn", callCompare),
	registerFunc("concat", "", callConcat),
	registerFunc("concat", "fn", callConcat),
	registerFunc("string-join", "", callStringJoin),
	registerFunc("string-join", "fn", callStringJoin),
	registerFunc("substring", "", callSubstring),
	registerFunc("substring", "fn", callSubstring),
	registerFunc("string-length", "", callStringLength),
	registerFunc("string-length", "fn", callStringLength),
	registerFunc("normalize-space", "", callNormalizeSpace),
	registerFunc("normalize-space", "fn", callNormalizeSpace),
	registerFunc("upper-case", "", callUppercase),
	registerFunc("upper-case", "fn", callUppercase),
	registerFunc("lower-case", "", callLowercase),
	registerFunc("lower-case", "fn", callLowercase),
	registerFunc("translate", "", callTranslate),
	registerFunc("translate", "fn", callTranslate),
	registerFunc("contains", "", callContains),
	registerFunc("contains", "fn", callContains),
	registerFunc("starts-with", "", callStartsWith),
	registerFunc("starts-with", "fn", callStartsWith),
	registerFunc("ends-with", "", callEndsWith),
	registerFunc("ends-with", "fn", callEndsWith),
	registerFunc("substring-before", "", callSubstringBefore),
	registerFunc("substring-before", "fn", callSubstringBefore),
	registerFunc("substring-after", "", callSubstringAfter),
	registerFunc("substring-after", "fn", callSubstringAfter),
	registerFunc("matches", "", callMatches),
	registerFunc("matches", "fn", callMatches),
	registerFunc("tokenize", "", callTokenize),
	registerFunc("tokenize", "fn", callTokenize),
	registerFunc("sum", "", callSum),
	registerFunc("sum", "fn", callSum),
	registerFunc("count", "", callCount),
	registerFunc("count", "fn", callCount),
	registerFunc("avg", "", callAvg),
	registerFunc("avg", "fn", callAvg),
	registerFunc("min", "", callMin),
	registerFunc("min", "fn", callMin),
	registerFunc("max", "", callMax),
	registerFunc("max", "fn", callMax),
	registerFunc("zero-or-one", "", callZeroOrOne),
	registerFunc("zero-or-one", "fn", callZeroOrOne),
	registerFunc("one-or-more", "", callOneOrMore),
	registerFunc("one-or-more", "fn", callOneOrMore),
	registerFunc("exactly-one", "", callExactlyOne),
	registerFunc("exactly-one", "fn", callExactlyOne),
	registerFunc("position", "", callPosition),
	registerFunc("position", "fn", callPosition),
	registerFunc("last", "", callLast),
	registerFunc("last", "fn", callLast),
	registerFunc("current-date", "", callCurrentDate),
	registerFunc("current-date", "fn", callCurrentDate),
	registerFunc("current-dateTime", "", callCurrentDatetime),
	registerFunc("current-dateTime", "fn", callCurrentDatetime),
	registerFunc("exists", "", callExists),
	registerFunc("exists", "fn", callExists),
	registerFunc("empty", "", callEmpty),
	registerFunc("empty", "fn", callEmpty),
	registerFunc("distinct-values", "", callDistinctValues),
	registerFunc("distinct-values", "fn", callDistinctValues),
	registerFunc("tail", "", callTail),
	registerFunc("tail", "fn", callTail),
	registerFunc("head", "", callHead),
	registerFunc("head", "fn", callHead),
	registerFunc("reverse", "", callReverse),
	registerFunc("reverse", "fn", callReverse),
	registerFunc("round", "", callRound),
	registerFunc("round", "fn", callRound),
	registerFunc("floor", "", callFloor),
	registerFunc("floor", "fn", callFloor),
	registerFunc("ceiling", "", callCeil),
	registerFunc("ceiling", "fn", callCeil),
	registerFunc("number", "", callNumber),
	registerFunc("number", "fn", callNumber),
	registerFunc("abs", "", callAbs),
	registerFunc("abs", "fn", callAbs),
	registerFunc("date", "", callDate),
	registerFunc("date", "xs", callDate),
	registerFunc("decimal", "", callDecimal),
	registerFunc("decimal", "xs", callDecimal),
	registerFunc("doc", "fn", callDoc),
	registerFunc("doc", "", callDoc),
}

var fileFuncs = []registeredBuiltin{
	registerFunc("read-file", "file", callReadFile),
	registerFunc("write-file", "file", callWriteFile),
	registerFunc("exists", "file", callFileExists),
	registerFunc("delete", "file", callDeleteFile),
	registerFunc("list", "file", callListDir),
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

func callDecimal(ctx Context, args []Expr) (Sequence, error) {
	if len(args) != 1 {
		return nil, ErrArgument
	}
	val, err := getFloatFromExpr(args[0], ctx)
	if err != nil {
		return nil, err
	}
	return Singleton(val), nil
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
	if ctx.Index != ctx.Size {
		return nil, nil
	}
	return Singleton(ctx.Node), nil
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

func callString(ctx Context, args []Expr) (Sequence, error) {
	if len(args) == 0 {
		return Singleton(ctx.Value()), nil
	}
	items, err := expandArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return Singleton(""), nil
	}
	if !items[0].Atomic() {
		return callString(DefaultContext(items[0].Node()), nil)
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
		return nil, ErrType
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
		if !i.Atomic() {
			return nil, fmt.Errorf("literal value expected")
		}
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
		return nil, nil
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
		return callStringLength(DefaultContext(items[0].Node()), nil)
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
		if len(set) < ix {
			return set[ix]
		}
		return -1
	}, str)
	return Singleton(str), ErrImplemented
}

func callContains(ctx Context, args []Expr) (Sequence, error) {
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
	return callPath(DefaultContext(n.Node()), nil)
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
	return callHasChildren(DefaultContext(n.Node()), nil)
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
	if len(args) != 0 {
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

func getFloatFromExpr(expr Expr, ctx Context) (float64, error) {
	items, err := expr.find(ctx)
	if err != nil || len(items) != 1 {
		return math.NaN(), err
	}
	if !items[0].Atomic() {
		return toFloat(items[0].Value())
	}
	return toFloat(items[0].Value())
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
