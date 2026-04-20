package export

import (
	"context"
	"log"
	"sync"
	"time"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
)

const defaultExportTimeout = 5 * time.Second

type Fanout struct {
	traces  []TraceExporter
	timeout time.Duration
}

func NewFanout(exporters ...TraceExporter) *Fanout {
	return &Fanout{traces: exporters, timeout: defaultExportTimeout}
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

func (f *Fanout) Shutdown(ctx context.Context) {
	if f == nil {
		return
	}
	for _, exp := range f.traces {
		if err := exp.Shutdown(ctx); err != nil {
			log.Printf("shutdown exporter %s: %v", exp.Name(), err)
		}
	}
}
