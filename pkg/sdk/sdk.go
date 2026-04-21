package sdk

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type config struct {
	serviceName    string
	serviceVersion string
	environment    string
	endpoint       string
	insecure       bool
}

// Option configures sdk.Init.
type Option func(*config)

func WithServiceName(name string) Option {
	return func(c *config) { c.serviceName = name }
}

func WithServiceVersion(v string) Option {
	return func(c *config) { c.serviceVersion = v }
}

func WithEndpoint(endpoint string) Option {
	return func(c *config) { c.endpoint = endpoint }
}

func WithEnvironment(env string) Option {
	return func(c *config) { c.environment = env }
}

func WithInsecure(insecure bool) Option {
	return func(c *config) { c.insecure = insecure }
}

// ShutdownFunc flushes and shuts down all OTel providers.
type ShutdownFunc func(context.Context) error

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Init wires traces, metrics, and logs to the OTLP endpoint, sets OTel globals,
// sets slog.SetDefault, and redirects stdlib log through slog.
// Call once from main(); defer the returned ShutdownFunc.
func Init(opts ...Option) (ShutdownFunc, *slog.Logger, error) {
	c := &config{
		endpoint: "localhost:4317",
		insecure: true,
	}
	for _, opt := range opts {
		opt(c)
	}
	c.endpoint = envOr("OTEL_EXPORTER_OTLP_ENDPOINT", c.endpoint)
	c.serviceName = envOr("OTEL_SERVICE_NAME", c.serviceName)

	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(c.serviceName),
			semconv.ServiceVersion(c.serviceVersion),
			semconv.DeploymentEnvironment(c.environment),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("build resource: %w", err)
	}

	// --- TracerProvider ---
	traceOpts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(c.endpoint)}
	if c.insecure {
		traceOpts = append(traceOpts, otlptracegrpc.WithInsecure())
	}
	traceExporter, err := otlptracegrpc.New(ctx, traceOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(traceExporter)),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)

	// --- MeterProvider ---
	metricOpts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(c.endpoint)}
	if c.insecure {
		metricOpts = append(metricOpts, otlpmetricgrpc.WithInsecure())
	}
	metricExporter, err := otlpmetricgrpc.New(ctx, metricOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("metric exporter: %w", err)
	}
	mp := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
	)
	otel.SetMeterProvider(mp)

	// --- LoggerProvider ---
	logOpts := []otlploggrpc.Option{otlploggrpc.WithEndpoint(c.endpoint)}
	if c.insecure {
		logOpts = append(logOpts, otlploggrpc.WithInsecure())
	}
	logExporter, err := otlploggrpc.New(ctx, logOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("log exporter: %w", err)
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
	)
	global.SetLoggerProvider(lp)

	// --- Propagator ---
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// --- Logger ---
	handler := otelslog.NewHandler(c.serviceName, otelslog.WithLoggerProvider(lp))
	logger := slog.New(handler)
	slog.SetDefault(logger)
	log.SetOutput(slog.NewLogLogger(logger.Handler(), slog.LevelInfo).Writer())
	log.SetFlags(0)

	shutdown := func(ctx context.Context) error {
		var errs []error
		if err := tp.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("tracer: %w", err))
		}
		if err := mp.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("meter: %w", err))
		}
		if err := lp.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("logger: %w", err))
		}
		if len(errs) > 0 {
			return fmt.Errorf("shutdown: %v", errs)
		}
		return nil
	}

	return shutdown, logger, nil
}
