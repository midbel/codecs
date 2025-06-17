package xpath

import (
	"io"
	"os"
	"log/slog"
)

type Tracer interface {
	Enter(string)
	Leave(string)
	Error(string, error)
}

type discardTracer struct{}

func (_ discardTracer) Enter(_ string)          {}
func (_ discardTracer) Leave(_ string)          {}
func (_ discardTracer) Error(_ string, _ error) {}

type stdioTracer struct {
	logger   *slog.Logger
	depth    int
	errcount int
}

func TraceStdout() Tracer {
	tracer := stdioTracer{
		logger: stdioLogger(os.Stdout),
	}
	return &tracer
}

func TraceStderr() Tracer {
	tracer := stdioTracer{
		logger: stdioLogger(os.Stderr),
	}
	return &tracer
}

func stdioLogger(w io.Writer) *slog.Logger {
	opts := slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	return slog.New(slog.NewTextHandler(w, &opts))
}

func (t *stdioTracer) Enter(rule string) {
	t.depth++
	args := []any{
		"expression",
		rule,
		"depth",
		t.depth,
	}
	t.logger.Debug("start compile expr", args...)
}

func (t *stdioTracer) Leave(rule string) {
	t.depth--
	args := []any{
		"expression",
		rule,
		"depth",
		t.depth,
	}
	t.logger.Debug("done compile expr", args...)
}

func (t *stdioTracer) Error(rule string, err error) {

}