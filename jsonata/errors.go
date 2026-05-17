package jsonata

import (
	"errors"
)

var (
	errUndefined   = errors.New("undefined")
	errDiscard     = errors.New("discard")
	errType        = errors.New("type")
	errArgument    = errors.New("argument")
	errImplemented = errors.New("not implemented")
)
