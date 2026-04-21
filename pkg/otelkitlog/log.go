package otelkitlog

import (
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/log"
)

func NewHandler(lp log.LoggerProvider) slog.Handler {
	return otelslog.NewHandler("otelkit", otelslog.WithLoggerProvider(lp))
}
