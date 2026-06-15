package probe

import (
	"errors"
	"fmt"
	"iter"
	"strconv"
)

var (
	ErrType  = errors.New("invalid type")
	ErrEnd   = errors.New("unexpected end of path")
	ErrProp  = errors.New("property not found")
	ErrIndex = errors.New("index out of range")
)

type (
	missing struct{}
	discard struct{}
)

var (
	missed    = missing{}
	discarded = discard{}
)

type Path interface {
	Collect(any, *Options) (any, error)
	All(any, *Options) iter.Seq2[any, error]
}

type Expr interface {
	Eval(any, *Options) (any, error)
}

type single struct {
	Anchored bool
	Start    Expr
}

func (p single) Collect(in any, opts *Options) (any, error) {
	return p.Start.Eval(in, opts)
}

func (p single) All(in any, opts *Options) iter.Seq2[any, error] {
	it := func(yield func(any, error) bool) {
		d, err := p.Collect(in, opts)
		yield(d, err)
	}
	return it
}

type root struct {
	base Path
	next []Path
}

func (p root) All(in any, opts *Options) iter.Seq2[any, error] {
	it := func(yield func(any, error) bool) {
		for in, err := range p.base.All(in, opts) {
			if err != nil {
				return
			}
			for _, n := range p.next {
				d, err := n.Collect(in, opts)
				if ok := yield(d, err); !ok {
					return
				}
			}
		}
	}
	return it
}

func (p root) Collect(in any, opts *Options) (any, error) {
	return nil, nil
}

type multi struct {
	paths []Path
}

func (p multi) Collect(in any, opts *Options) (any, error) {
	var list []any
	for _, i := range p.paths {
		res, err := i.Collect(in, opts)
		if err != nil {
			return nil, err
		}
		list = append(list, res)
	}
	return list, nil
}

func (p multi) All(in any, opts *Options) iter.Seq2[any, error] {
	it := func(yield func(any, error) bool) {
		d, err := p.Collect(in, opts)
		yield(d, err)
	}
	return it
}

type alternative struct {
	paths []Path
}

func (p alternative) Collect(in any, opts *Options) (any, error) {
	var last any
	for _, i := range p.paths {
		a, err := i.Collect(in, opts)
		if err != nil {
			continue
		}
		last = a
		if isDefined(a) {
			return a, nil
		}
	}
	return last, nil
}

func (p alternative) All(in any, opts *Options) iter.Seq2[any, error] {
	it := func(yield func(any, error) bool) {
		d, err := p.Collect(in, opts)
		yield(d, err)
	}
	return it
}

type call struct {
	Ident string
	Args  []Expr
}

func (c call) Eval(in any, _ *Options) (any, error) {
	fn, ok := builtins[c.Ident]
	if !ok {
		return nil, nil
	}
	return fn(in, c.Args)
}

type field struct {
	Name  string
	Alt   Expr
	Apply Expr
	Next  Expr
}

func (s field) Eval(in any, opts *Options) (any, error) {
	return traverse(s, in, opts)
}

type literal struct {
	value any
}

func (s literal) All(_ any, _ *Options) iter.Seq2[any, error] {
	it := func(yield func(any, error) bool) {
		yield(s.value, nil)
	}
	return it
}

func (s literal) Collect(_ any, _ *Options) (any, error) {
	return s.value, nil
}

func (s literal) Eval(_ any, _ *Options) (any, error) {
	return s.value, nil
}

func traverse(e Expr, in any, opts *Options) (any, error) {
	if isMissing(in) {
		return in, nil
	}
	if e, ok := e.(literal); ok {
		return []any{e.value}, nil
	}
	switch in := in.(type) {
	case []any:
		return traverseArray(e, in, opts)
	case map[string]any:
		return traverseMap(e, in, opts)
	default:
		return nil, fmt.Errorf("%w: array or object expected", ErrType)
	}
}

func traverseArray(e Expr, in []any, opts *Options) (any, error) {
	var result []any
	for i := range in {
		tmp, err := traverse(e, in[i], opts)
		if err != nil {
			return nil, err
		}
		result = append(result, tmp)
	}
	return result, nil
}

func traverseMap(e Expr, in map[string]any, opts *Options) (any, error) {
	if e == nil {
		return nil, ErrEnd
	}
	x, ok := e.(field)
	if !ok {
		return nil, nil
	}
	p, ok := in[x.Name]
	if !ok {
		if x.Alt == nil {
			p = missed
		} else {
			p, _ = x.Alt.Eval(in, opts)
		}
	}
	if x.Apply != nil {
		r, err := x.Apply.Eval(p, opts)
		if err != nil {
			return nil, err
		}
		p = r
	}
	if x.Next == nil {
		return p, nil
	}
	return traverse(x.Next, p, opts)
}

func isMissing(val any) bool {
	_, ok := val.(missing)
	return ok
}

func isDiscard(val any) bool {
	_, ok := val.(discard)
	return ok
}

func isDefined(val any) bool {
	if isMissing(val) {
		return false
	}
	switch a := val.(type) {
	case nil:
	case []any:
		return len(a) > 0
	case map[string]any:
		return len(a) > 0
	case string:
		return len(a) > 0
	case float64:
		return a != 0
	case bool:
		return a
	default:
	}
	return false
}

func isEqual(fst, snd any) bool {
	if other, ok := snd.(literal); ok {
		return isEqual(fst, other.value)
	}
	switch f := fst.(type) {

	case bool:
		other, ok := snd.(bool)
		if ok {
			return f == other
		}
		return ok
	case string:
		other, ok := snd.(string)
		if ok {
			return f == other
		}
		return ok
	case float64:
		other, ok := snd.(float64)
		if ok {
			return f == other
		}
		return ok
	case nil:
		return snd == nil
	default:
		return false
	}
}

func isLess(fst, snd any) bool {
	if other, ok := snd.(literal); ok {
		return isLess(fst, other.value)
	}
	switch f := fst.(type) {
	case string:
		other, ok := snd.(string)
		if ok {
			return f < other
		}
		return ok
	case float64:
		other, ok := snd.(float64)
		if ok {
			return f < other
		}
		return ok
	default:
		return false
	}
}

func getAnyFromExpr(expr Expr) (any, error) {
	lit, ok := expr.(literal)
	if !ok {
		return nil, ErrType
	}
	return lit.value, nil
}

func getStrFromExpr(expr Expr) (string, error) {
	val, err := getAnyFromExpr(expr)
	if err != nil {
		return "", err
	}
	s, ok := val.(string)
	if !ok {
		return "", ErrType
	}
	return s, nil
}

func getIntFromExpr(expr Expr) (int, error) {
	val, err := getAnyFromExpr(expr)
	if err != nil {
		return 0, err
	}
	n, ok := val.(float64)
	if !ok {
		return 0, ErrType
	}
	return int(n), nil
}

func castToString(val any) (any, error) {
	if isMissing(val) || isDiscard(val) {
		return "", nil
	}
	if val == nil {
		return "null", nil
	}
	return fmt.Sprint(val), nil
}

func castToBool(val any) (any, error) {
	switch v := val.(type) {
	case bool:
		return v, nil
	case string:
		return v != "", nil
	case float64:
		return v != 0, nil
	case nil:
		return false, nil
	case missing:
		return false, nil
	case discard:
		return false, nil
	default:
		return false, ErrType
	}
}

func castToNumber(val any) (any, error) {
	switch v := val.(type) {
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
		return v, nil
	case string:
		x, err := strconv.ParseFloat(v, 64)
		if err != nil {
			err = ErrType
		}
		return x, err
	case float64:
		return v, nil
	case nil:
		return 0, nil
	case missing:
		return 0, nil
	case discard:
		return 0, nil
	default:
		return 0, ErrType
	}
}

func arrayExpected(fn string) error {
	return fmt.Errorf("%w: expected array as input of %s", ErrType, fn)
}

func objectExpected(fn string) error {
	return fmt.Errorf("%w: expected object as input of %s", ErrType, fn)
}

func compositeExpected(fn string) error {
	return fmt.Errorf("%w: expected array or object as input of %s", ErrType, fn)
}
