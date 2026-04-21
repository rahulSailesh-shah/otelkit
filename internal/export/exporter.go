package export

import (
	"context"

	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
)

type TraceExporter interface {
	Name() string
	ExportTraces(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) error
	Shutdown(ctx context.Context) error
}

type MetricsExporter interface {
	Name() string
	ExportMetrics(ctx context.Context, req *colmetricspb.ExportMetricsServiceRequest) error
	Shutdown(ctx context.Context) error
}
