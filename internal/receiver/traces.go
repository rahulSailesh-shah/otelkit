package receiver

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/rahulSailesh-shah/otelkit/internal/export"
	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

type TraceHandler struct {
	db     *sql.DB
	fanout *export.Fanout
	coltracepb.UnimplementedTraceServiceServer
}

func NewTraceHandler(db *sql.DB, fanout *export.Fanout) *TraceHandler {
	return &TraceHandler{db: db, fanout: fanout}
}

func (h *TraceHandler) Export(
	ctx context.Context,
	req *coltracepb.ExportTraceServiceRequest,
) (*coltracepb.ExportTraceServiceResponse, error) {
	spans, err := normalize(req)
	if err != nil {
		return nil, err
	}

	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	q := repo.New(tx)
	for _, s := range spans {
		if err := q.InsertSpan(ctx, s); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if h.fanout != nil {
		go func(req *coltracepb.ExportTraceServiceRequest) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			h.fanout.ExportTraces(bgCtx, req)
		}(req)
	}

	return &coltracepb.ExportTraceServiceResponse{}, nil
}

func normalize(req *coltracepb.ExportTraceServiceRequest) ([]repo.InsertSpanParams, error) {
	if req == nil {
		return nil, nil
	}

	spanCount := 0
	for _, rs := range req.GetResourceSpans() {
		for _, ss := range rs.GetScopeSpans() {
			spanCount += len(ss.GetSpans())
		}
	}

	spans := make([]repo.InsertSpanParams, 0, spanCount)
	for _, rs := range req.GetResourceSpans() {
		resourceAttrs, serviceName, err := marshalAttributes(rs.GetResource())
		if err != nil {
			return nil, err
		}

		for _, ss := range rs.GetScopeSpans() {
			for _, span := range ss.GetSpans() {
				attributes, err := marshalKeyValues(span.GetAttributes())
				if err != nil {
					return nil, err
				}

				events, err := marshalEvents(span.GetEvents())
				if err != nil {
					return nil, err
				}

				spans = append(spans, repo.InsertSpanParams{
					SpanID:        hex.EncodeToString(span.GetSpanId()),
					TraceID:       hex.EncodeToString(span.GetTraceId()),
					ParentSpanID:  optionalHex(span.GetParentSpanId()),
					Name:          span.GetName(),
					ServiceName:   serviceName,
					SpanKind:      int64(span.GetKind()),
					StartTimeNs:   int64(span.GetStartTimeUnixNano()),
					EndTimeNs:     int64(span.GetEndTimeUnixNano()),
					StatusCode:    int64(span.GetStatus().GetCode()),
					StatusMessage: optionalString(span.GetStatus().GetMessage()),
					Attributes:    attributes,
					Events:        events,
					ResourceAttrs: resourceAttrs,
				})
			}
		}
	}

	return spans, nil
}

func marshalAttributes(resource *resourcepb.Resource) (*string, string, error) {
	if resource == nil {
		return nil, "", nil
	}

	values := keyValuesToMap(resource.GetAttributes())
	if len(values) == 0 {
		return nil, "", nil
	}

	raw, err := json.Marshal(values)
	if err != nil {
		return nil, "", err
	}

	serviceName, _ := values["service.name"].(string)
	encoded := string(raw)
	return &encoded, serviceName, nil
}

func marshalKeyValues(attrs []*commonpb.KeyValue) (*string, error) {
	if len(attrs) == 0 {
		return nil, nil
	}

	raw, err := json.Marshal(keyValuesToMap(attrs))
	if err != nil {
		return nil, err
	}

	encoded := string(raw)
	return &encoded, nil
}

func marshalEvents(events []*tracepb.Span_Event) (*string, error) {
	if len(events) == 0 {
		return nil, nil
	}

	normalized := make([]map[string]any, 0, len(events))
	for _, event := range events {
		item := map[string]any{
			"name":                     event.GetName(),
			"time_unix_nano":           event.GetTimeUnixNano(),
			"dropped_attributes_count": event.GetDroppedAttributesCount(),
		}

		if attrs := keyValuesToMap(event.GetAttributes()); len(attrs) > 0 {
			item["attributes"] = attrs
		}

		normalized = append(normalized, item)
	}

	raw, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}

	encoded := string(raw)
	return &encoded, nil
}

func keyValuesToMap(attrs []*commonpb.KeyValue) map[string]any {
	if len(attrs) == 0 {
		return nil
	}

	values := make(map[string]any, len(attrs))
	for _, attr := range attrs {
		values[attr.GetKey()] = anyValueToInterface(attr.GetValue())
	}
	return values
}

func anyValueToInterface(value *commonpb.AnyValue) any {
	if value == nil {
		return nil
	}

	switch v := value.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return v.StringValue
	case *commonpb.AnyValue_BoolValue:
		return v.BoolValue
	case *commonpb.AnyValue_IntValue:
		return v.IntValue
	case *commonpb.AnyValue_DoubleValue:
		return v.DoubleValue
	case *commonpb.AnyValue_ArrayValue:
		values := v.ArrayValue.GetValues()
		out := make([]any, 0, len(values))
		for _, item := range values {
			out = append(out, anyValueToInterface(item))
		}
		return out
	case *commonpb.AnyValue_KvlistValue:
		return keyValuesToMap(v.KvlistValue.GetValues())
	case *commonpb.AnyValue_BytesValue:
		return base64.StdEncoding.EncodeToString(v.BytesValue)
	default:
		return nil
	}
}

func optionalHex(value []byte) *string {
	if len(value) == 0 {
		return nil
	}

	encoded := hex.EncodeToString(value)
	return &encoded
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}
