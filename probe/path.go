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
	Collect(any) ([]any, error)
}

type Expr interface {
	Eval(any) (any, error)
}

type single struct {
	Anchored bool
	Start    Expr
}

func (p single) Collect(in any) ([]any, error) {
	return nil, nil
}

type multi struct {
	paths []Path
}

func (p multi) Collect(in any) ([]any, error) {
	return nil, nil
}

type alternative struct {
	paths []Path
}

func (p alternative) Collect(in any) ([]any, error) {
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

func (s literal) Collect(_ any) ([]any, error) {
	return []any{s.value}, nil
}

func (s literal) Eval(_ any) (any, error) {
	return s.value, nil
}
