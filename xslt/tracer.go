package xslt

import (
	"io"
	"log/slog"
	"os"
)

type Tracer interface {
	Enter(*Context)
	Leave(*Context)
	Error(*Context, error)
}

func NoopTracer() Tracer {
	return discardTracer{}
}

type discardTracer struct{}

func (_ discardTracer) Enter(_ *Context) {}

func (_ discardTracer) Leave(_ *Context) {}

func (_ discardTracer) Error(_ *Context, _ error) {}

type stdioTracer struct {
	logger *slog.Logger
}

func Stdout() Tracer {
	return stdioTracer{
		logger: stdioLogger(os.Stdout),
	}
}

func Stderr() Tracer {
	return stdioTracer{
		logger: stdioLogger(os.Stderr),
	}
}

func stdioLogger(w io.Writer) *slog.Logger {
	opts := slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	return slog.New(slog.NewTextHandler(w, &opts))
}

func (t stdioTracer) Println(msg string) {
	t.logger.Info(msg)
}

func (t stdioTracer) Enter(ctx *Context) {
	args := []any{
		"instruction",
		ctx.XslNode.QualifiedName(),
		"node",
		ctx.ContextNode.QualifiedName(),
		"depth",
		ctx.Depth,
	}
	t.logger.Debug("start instruction", args...)
}

func (t stdioTracer) Leave(ctx *Context) {
	args := []any{
		"instruction",
		ctx.XslNode.QualifiedName(),
		"node",
		ctx.ContextNode.QualifiedName(),
		"depth",
		ctx.Depth,
	}
	t.logger.Debug("done instruction", args...)
}

func (t stdioTracer) Error(ctx *Context, err error) {
	t.logger.Error("error while processing instruction", "node", ctx.ContextNode.QualifiedName(), "depth", ctx.Depth, "err", err.Error())
}
