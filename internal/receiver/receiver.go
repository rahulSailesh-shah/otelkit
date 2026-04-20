package receiver

import (
	"net"

	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
)

type Server struct {
	grpcServer *grpc.Server
	lis        net.Listener
}

func StartGRPC(
	addr string,
	traceHandler coltracepb.TraceServiceServer,
	metricsHandler colmetricspb.MetricsServiceServer,
) (*Server, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	s := grpc.NewServer()
	coltracepb.RegisterTraceServiceServer(s, traceHandler)
	colmetricspb.RegisterMetricsServiceServer(s, metricsHandler)

	go func() {
		_ = s.Serve(lis)
	}()

	return &Server{grpcServer: s, lis: lis}, nil
}

func (s *Server) Stop() {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
}
