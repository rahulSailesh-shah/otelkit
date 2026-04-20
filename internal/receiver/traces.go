package receiver

import (
	"context"
	"log"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

type TraceHandler struct {
	coltracepb.UnimplementedTraceServiceServer
}

func NewTraceHandler() *TraceHandler {
	return &TraceHandler{}
}

func (h *TraceHandler) Export(
	ctx context.Context,
	req *coltracepb.ExportTraceServiceRequest,
) (*coltracepb.ExportTraceServiceResponse, error) {
	_ = ctx
	spanCount := 0
	for _, rs := range req.GetResourceSpans() {
		for _, ss := range rs.GetScopeSpans() {
			spanCount += len(ss.GetSpans())
		}
	}
	log.Printf("received OTLP export: resourceSpans=%d spans=%d", len(req.GetResourceSpans()), spanCount)

	dump, err := protojson.MarshalOptions{
		Multiline:       true,
		Indent:          "  ",
		EmitUnpopulated: true,
	}.Marshal(req)
	if err != nil {
		log.Printf("marshal ExportTraceServiceRequest: %v", err)
	} else {
		log.Printf("ExportTraceServiceRequest (JSON):\n%s", string(dump))
	}
	return &coltracepb.ExportTraceServiceResponse{}, nil
}
