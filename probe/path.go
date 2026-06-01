package probe

import (
	"errors"
)

var (
	ErrType = errors.New("array or object expected")
	ErrEnd  = errors.New("unexpected end of path")
	ErrProp = errors.New("property not found")
)

type Path interface {
	Collect(any) (any, error)
}

type Expr interface {
	Eval(any) (any, error)
}

type single struct {
	Anchored bool
	Start    Expr
}

func (p single) Collect(in any) (any, error) {
	return p.Start.Eval(in)
}

type multi struct {
	paths []Path
}

func (p multi) Collect(in any) (any, error) {
	var list []any
	for _, i := range p.paths {
		res, err := i.Collect(in)
		if err != nil {
			return nil, err
		}
		list = append(list, res)
	}
	return list, nil
}

type alternative struct {
	paths []Path
}

func (p alternative) Collect(in any) (any, error) {
	for _, p := range p.paths {
		a, err := p.Collect(in)
		if err != nil {
			continue
		}
		switch a := a.(type) {
		case nil:
		case []any:
			if len(a) > 0 {
				return a, nil
			}
		default:
			return a, nil
		}
	}
	return nil, nil
}

type field struct {
	Name string
	Next Expr
}

func (s field) Eval(in any) (any, error) {
	return traverse(s, in)
}

type literal struct {
	value any
}

func (s literal) Collect(_ any) (any, error) {
	return s.value, nil
}

func (s literal) Eval(_ any) (any, error) {
	return s.value, nil
}

func traverse(e Expr, in any) (any, error) {
	if e, ok := e.(literal); ok {
		return []any{e.value}, nil
	}
	switch in := in.(type) {
	case []any:
		return traverseArray(e, in)
	case map[string]any:
		return traverseMap(e, in)
	default:
		return nil, ErrType
	}
}

func traverseArray(e Expr, in []any) (any, error) {
	var result []any
	for i := range in {
		tmp, err := traverse(e, in[i])
		if err != nil {
			return nil, err
		}
		result = append(result, tmp)
	}
	return result, nil
}

func traverseMap(e Expr, in map[string]any) (any, error) {
	if e == nil {
		return nil, ErrEnd
	}
	x, ok := e.(field)
	if !ok {
		return nil, nil
	}
	p, ok := in[x.Name]
	if !ok {
		return nil, nil
	}
	if x.Next == nil {
		return p, nil
	}
	return traverse(x.Next, p)
}
