package json

import (
	"bufio"
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func Find(r io.Reader, q string) (any, error) {
	doc, err := Parse(r)
	if err != nil {
		return nil, err
	}
	if q == "" {
		return doc, nil
	}
	query, err := Compile(q)
	if err != nil {
		return nil, err
	}
	return query.Get(doc)
}

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
