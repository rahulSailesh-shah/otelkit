package export

import (
	"context"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
)

type TraceExporter interface {
	Name() string
	ExportTraces(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) error
	Shutdown(ctx context.Context) error
}
