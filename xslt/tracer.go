package xslt

import (
	"io"
	"log/slog"
	"os"
	"time"
)

type Tracer interface {
	Start()
	Done()
	Enter(*Context)
	Leave(*Context)
	Error(*Context, error)
	Query(*Context, string)
}

func NoopTracer() Tracer {
	return discardTracer{}
}

type discardTracer struct{}

func (_ discardTracer) Start() {}

func (_ discardTracer) Done() {}

func (_ discardTracer) Enter(_ *Context) {}

func (_ discardTracer) Leave(_ *Context) {}

func (_ discardTracer) Error(_ *Context, _ error) {}

func (_ discardTracer) Query(_ *Context, _ string) {}

type stdioTracer struct {
	logger     *slog.Logger
	when       time.Time
	errCount   int
	instrCount int
	queryCount int
}

func Stdout() Tracer {
	return &stdioTracer{
		logger: stdioLogger(os.Stdout),
		when:   time.Now(),
	}
}

func Stderr() Tracer {
	return &stdioTracer{
		logger: stdioLogger(os.Stderr),
		when:   time.Now(),
	}
}

func stdioLogger(w io.Writer) *slog.Logger {
	opts := slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	return slog.New(slog.NewTextHandler(w, &opts))
}

func (t *stdioTracer) Start() {
	t.logger.Info("start")
}

func (t *stdioTracer) Done() {
	args := []any{
		"elapsed",
		time.Since(t.when),
		"instructions",
		t.instrCount,
		"errors",
		t.errCount,
		"query",
		t.queryCount,
	}
	t.logger.Info("done", args...)
}

func (t *stdioTracer) Println(msg string) {
	t.logger.Info(msg)
}

func (t *stdioTracer) Enter(ctx *Context) {
	args := []any{
		"instruction",
		ctx.XslNode.QualifiedName(),
		"node",
		ctx.ContextNode.QualifiedName(),
		"depth",
		ctx.Depth,
	}
	t.instrCount++
	t.logger.Debug("start instruction", args...)
}

func (t *stdioTracer) Leave(ctx *Context) {
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

func (t *stdioTracer) Error(ctx *Context, err error) {
	args := []any{
		"instruction",
		ctx.XslNode.QualifiedName(),
		"node",
		ctx.ContextNode.QualifiedName(),
		"depth",
		ctx.Depth,
		"err",
		err.Error(),
	}
	t.errCount++
	t.logger.Error("error while processing instruction", args...)
}

func (t *stdioTracer) Query(ctx *Context, query string) {
	args := []any{
		"instruction",
		ctx.XslNode.QualifiedName(),
		"node",
		ctx.ContextNode.QualifiedName(),
		"query",
		query,
	}
	t.queryCount++
	t.logger.Debug("run query", args...)
}
