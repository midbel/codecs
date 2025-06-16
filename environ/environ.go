package environ

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

var ErrDefined = errors.New("undefined identifier")

type Environ[T any] interface {
	Resolve(string) (T, error)
	Define(string, T)
	Names() []string
	Len() int
}

type Env[T any] struct {
	values map[string]T
	parent Environ[T]
}

func Empty[T any]() Environ[T] {
	return Enclosed[T](nil)
}

func Enclosed[T any](parent Environ[T]) Environ[T] {
	e := Env[T]{
		values: make(map[string]T),
		parent: parent,
	}
	return &e
}

func (e *Env[T]) Len() int {
	return len(e.values)
}

func (e *Env[T]) Names() []string {
	return slices.Collect(maps.Keys(e.values))
}

func (e *Env[T]) Define(ident string, expr T) {
	e.values[ident] = expr
}

func (e *Env[T]) Resolve(ident string) (T, error) {
	expr, ok := e.values[ident]
	if ok {
		return expr, nil
	}
	if e.parent != nil {
		return e.parent.Resolve(ident)
	}
	var t T
	return t, fmt.Errorf("%s: %w", ident, ErrDefined)
}

func (e *Env[T]) Unwrap() Environ[T] {
	if e.parent == nil {
		return e
	}
	return e.parent
}

func (e *Env[T]) Merge(other Environ[T]) {
	x, ok := other.(*Env[T])
	if !ok {
		return
	}
	maps.Copy(e.values, x.values)
}

func (e *Env[T]) Clone() Environ[T] {
	var x Env[T]
	x.values = make(map[string]T)
	maps.Copy(x.values, e.values)

	if c, ok := e.parent.(interface{ Clone() Environ[T] }); ok {
		x.parent = c.Clone()
	}
	return &x
}
