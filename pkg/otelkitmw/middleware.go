package otelkitmw

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

// Option reserved for future use (e.g. WithSpanNameFormatter).
type Option func()

// Wrap instruments a single handler or route with an OTel span.
// operation is the span name (e.g. "GET /todos").
// Reads TracerProvider and MeterProvider from OTel globals set by sdk.Init().
func Wrap(h http.Handler, operation string, opts ...Option) http.Handler {
	return otelhttp.NewHandler(h, operation,
		otelhttp.WithTracerProvider(otel.GetTracerProvider()),
		otelhttp.WithMeterProvider(otel.GetMeterProvider()),
	)
}

// NewHandler wraps the root mux and should be assigned to http.Server.Handler.
// serviceName becomes the root span operation name for all incoming traffic.
// Reads TracerProvider and MeterProvider from OTel globals set by sdk.Init().
func NewHandler(mux http.Handler, serviceName string, opts ...Option) http.Handler {
	return otelhttp.NewHandler(mux, serviceName,
		otelhttp.WithTracerProvider(otel.GetTracerProvider()),
		otelhttp.WithMeterProvider(otel.GetMeterProvider()),
	)
}
