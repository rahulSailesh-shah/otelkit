package otelkitlog

import (
	"log/slog"
	"os"
)

func NewHandler() slog.Handler {
	return slog.NewJSONHandler(os.Stdout, nil)
}
