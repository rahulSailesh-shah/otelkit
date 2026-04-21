package export

import (
	"context"
	"log"
	"sync"
	"time"

	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
)

const defaultExportTimeout = 5 * time.Second

type Fanout struct {
	traces  []TraceExporter
	metrics []MetricsExporter
	timeout time.Duration
}

func NewFanout(exporters ...TraceExporter) *Fanout {
	return &Fanout{traces: exporters, timeout: defaultExportTimeout}
}

func NewFanoutWithMetrics(traces []TraceExporter, metrics []MetricsExporter) *Fanout {
	return &Fanout{traces: traces, metrics: metrics, timeout: defaultExportTimeout}
}

func (f *Fanout) WithTimeout(d time.Duration) *Fanout {
	f.timeout = d
	return f
}

func (f *Fanout) ExportTraces(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) {
	if f == nil || len(f.traces) == 0 || req == nil {
		return
	}

	var wg sync.WaitGroup
	for _, exp := range f.traces {
		wg.Add(1)
		go func(e TraceExporter) {
			defer wg.Done()
			callCtx, cancel := context.WithTimeout(ctx, f.timeout)
			defer cancel()
			if err := e.ExportTraces(callCtx, req); err != nil {
				log.Printf("exporter %s: %v", e.Name(), err)
			}
		}(exp)
	}
	wg.Wait()
}

func (f *Fanout) ExportMetrics(ctx context.Context, req *colmetricspb.ExportMetricsServiceRequest) {
	if f == nil || len(f.metrics) == 0 || req == nil {
		return
	}

	var wg sync.WaitGroup
	for _, exp := range f.metrics {
		wg.Add(1)
		go func(e MetricsExporter) {
			defer wg.Done()
			callCtx, cancel := context.WithTimeout(ctx, f.timeout)
			defer cancel()
			if err := e.ExportMetrics(callCtx, req); err != nil {
				log.Printf("exporter %s: %v", e.Name(), err)
			}
		}(exp)
	}
	wg.Wait()
}

func (f *Fanout) Shutdown(ctx context.Context) {
	if f == nil {
		return
	}
	for _, exp := range f.traces {
		if err := exp.Shutdown(ctx); err != nil {
			log.Printf("shutdown exporter %s: %v", exp.Name(), err)
		}
	}
	for _, exp := range f.metrics {
		if err := exp.Shutdown(ctx); err != nil {
			log.Printf("shutdown exporter %s: %v", exp.Name(), err)
		}
	}
}
