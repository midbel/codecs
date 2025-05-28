package xpath

import (
	"github.com/midbel/codecs/environ"
)

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
