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
	return traverse(p.Start, in)
}

type multi struct {
	paths []Path
}

func (p multi) Collect(in any) (any, error) {
	var (
		list []any
		size int
	)
	for _, i := range p.paths {
		res, err := i.Collect(in)
		if err != nil {
			return nil, err
		}
		if z, ok := res.([]any); ok {
			size = max(size, len(z))
		}
		list = append(list, res)
	}
	out := make([]any, 0, size)
	for i := 0; i < size; i++ {
		tmp := make([]any, 0, size)
		for j := range list {
			switch a := list[j].(type) {
			case []any:
				if i < len(a) {
					tmp = append(tmp, a[i])
				} else {
					tmp = append(tmp, nil)
				}
			default:
				tmp = append(tmp, a)
			}
		}
		out = append(out, tmp)
	}
	return out, nil
}

type alternative struct {
	paths []Path
}

func (p alternative) Collect(in any) (any, error) {
	return nil, nil
}

type field struct {
	Name string
	Cast string
	Next Expr
}

func (s field) Eval(in any) (any, error) {
	return nil, nil
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
