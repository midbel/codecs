package jsonata

import (
	"cmp"
	"fmt"
	"math"
	"strconv"

	"github.com/midbel/codecs/internal/jsonkit"
)

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
	case jsonkit.And:
		return toBool(left) && toBool(right), nil
	case jsonkit.Or:
		return toBool(left) || toBool(right), nil
	case jsonkit.Add:
		return apply(left, right, func(left, right float64) float64 {
			return left + right
		})
	case jsonkit.Sub:
		return apply(left, right, func(left, right float64) float64 {
			return left - right
		})
	case jsonkit.Mul:
		return apply(left, right, func(left, right float64) float64 {
			return left * right
		})
	case jsonkit.Div:
		return apply(left, right, func(left, right float64) float64 {
			if right == 0 {
				return 0
			}
			return left / right
		})
	case jsonkit.Mod:
		return apply(left, right, func(left, right float64) float64 {
			if right == 0 {
				return 0
			}
			return math.Mod(left, right)
		})
	case jsonkit.Eq:
		return isEq(left, right)
	case jsonkit.Ne:
		return isNe(left, right)
	case jsonkit.Lt:
		return isLe(left, right)
	case jsonkit.Le:
		ok, err := isLe(left, right)
		if !ok && err == nil {
			ok, err = isEq(left, right)
		}
		return ok, err
	case jsonkit.Gt:
		if ok, err := isEq(left, right); ok && err == nil {
			return !ok, nil
		}
		ok, err := isLe(left, right)
		return !ok, err
	case jsonkit.Ge:
		if ok, err := isEq(left, right); ok && err == nil {
			return ok, nil
		}
		ok, err := isLe(left, right)
		return !ok, err
	case jsonkit.Concat:
		return toStr(left) + toStr(right), nil
	case jsonkit.In:
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
