package receiver

import (
	"net"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
)

type Server struct {
	grpcServer *grpc.Server
	lis        net.Listener
}

func StartGRPC(addr string, traceHandler coltracepb.TraceServiceServer) (*Server, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	s := grpc.NewServer()
	coltracepb.RegisterTraceServiceServer(s, traceHandler)

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
