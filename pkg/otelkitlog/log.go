package otelkitlog

import (
	"context"
	"encoding/json"
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/log/global"
)

// Wrap takes any slog.Handler and returns a handler that tees to both
// the original handler AND the OTel LoggerProvider (via OTLP).
// Reads LoggerProvider from the global set by sdk.Init().
//
// Usage:
//
//	slog.SetDefault(slog.New(
//	    otelkitlog.Wrap(slog.NewTextHandler(os.Stdout, nil)),
//	))
func Wrap(h slog.Handler) slog.Handler {
	lp := global.GetLoggerProvider()
	otlp := &bodyEnrichHandler{
		inner: otelslog.NewHandler("otelkit", otelslog.WithLoggerProvider(lp)),
	}
	return &teeHandler{handlers: []slog.Handler{otlp, h}}
}

// teeHandler fans out to multiple handlers.
type teeHandler struct {
	handlers []slog.Handler
}

func (h *teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, hh := range h.handlers {
		if hh.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *teeHandler) Handle(ctx context.Context, r slog.Record) error {
	var last error
	for _, hh := range h.handlers {
		if hh.Enabled(ctx, r.Level) {
			if err := hh.Handle(ctx, r.Clone()); err != nil {
				last = err
			}
		}
	}
	return last
}

func (h *teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(h.handlers))
	for i, hh := range h.handlers {
		next[i] = hh.WithAttrs(attrs)
	}
	return &teeHandler{handlers: next}
}

func (h *teeHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(h.handlers))
	for i, hh := range h.handlers {
		next[i] = hh.WithGroup(name)
	}
	return &teeHandler{handlers: next}
}

// bodyEnrichHandler rewrites the OTLP log body to JSON so Loki shows full context in the line.
type bodyEnrichHandler struct {
	inner slog.Handler
}

func (h *bodyEnrichHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *bodyEnrichHandler) Handle(ctx context.Context, r slog.Record) error {
	m := make(map[string]any)
	m["msg"] = r.Message
	r.Attrs(func(a slog.Attr) bool {
		m[a.Key] = a.Value.Any()
		return true
	})
	body, _ := json.Marshal(m)

	nr := slog.NewRecord(r.Time, r.Level, string(body), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		nr.AddAttrs(a)
		return true
	})
	return h.inner.Handle(ctx, nr)
}

func (h *bodyEnrichHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &bodyEnrichHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *bodyEnrichHandler) WithGroup(name string) slog.Handler {
	return &bodyEnrichHandler{inner: h.inner.WithGroup(name)}
}
