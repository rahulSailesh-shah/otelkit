package export

import (
	"context"
	"fmt"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type JaegerExporter struct {
	endpoint string
	conn     *grpc.ClientConn
	client   coltracepb.TraceServiceClient
}

func NewJaegerExporter(endpoint string) (*JaegerExporter, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("jaeger: empty endpoint")
	}
	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("jaeger: dial %s: %w", endpoint, err)
	}
	return &JaegerExporter{
		endpoint: endpoint,
		conn:     conn,
		client:   coltracepb.NewTraceServiceClient(conn),
	}, nil
}

func (j *JaegerExporter) Name() string { return "jaeger" }

func (j *JaegerExporter) Endpoint() string { return j.endpoint }

func (j *JaegerExporter) ExportTraces(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) error {
	_, err := j.client.Export(ctx, req)
	return err
}

func (j *JaegerExporter) Shutdown(_ context.Context) error {
	if j.conn == nil {
		return nil
	}
	return j.conn.Close()
}
