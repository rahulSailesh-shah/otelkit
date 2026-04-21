package export

import (
	"context"
	"fmt"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type SigNozExporter struct {
	endpoint string
	conn     *grpc.ClientConn
	traces   coltracepb.TraceServiceClient
	metrics  colmetricspb.MetricsServiceClient
	logs     collogspb.LogsServiceClient
}

func NewSigNozExporter(endpoint string) (*SigNozExporter, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("signoz: empty endpoint")
	}
	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("signoz: dial %s: %w", endpoint, err)
	}
	return &SigNozExporter{
		endpoint: endpoint,
		conn:     conn,
		traces:   coltracepb.NewTraceServiceClient(conn),
		metrics:  colmetricspb.NewMetricsServiceClient(conn),
		logs:     collogspb.NewLogsServiceClient(conn),
	}, nil
}

func (s *SigNozExporter) Name() string { return "signoz" }

func (s *SigNozExporter) ExportTraces(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) error {
	_, err := s.traces.Export(ctx, req)
	return err
}

func (s *SigNozExporter) ExportMetrics(ctx context.Context, req *colmetricspb.ExportMetricsServiceRequest) error {
	_, err := s.metrics.Export(ctx, req)
	return err
}

func (s *SigNozExporter) ExportLogs(ctx context.Context, req *collogspb.ExportLogsServiceRequest) error {
	_, err := s.logs.Export(ctx, req)
	return err
}

func (s *SigNozExporter) Shutdown(_ context.Context) error {
	if s.conn == nil {
		return nil
	}
	return s.conn.Close()
}
