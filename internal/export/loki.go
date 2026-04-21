package export

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

type lokiPushPayload struct {
	Streams []lokiStream `json:"streams"`
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][2]string       `json:"values"`
}

type LokiExporter struct {
	endpoint string
	client   *http.Client
}

func NewLokiExporter(endpoint string) *LokiExporter {
	return &LokiExporter{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (l *LokiExporter) Name() string { return "loki" }

func (l *LokiExporter) ExportLogs(ctx context.Context, req *collogspb.ExportLogsServiceRequest) error {
	if req == nil {
		return nil
	}

	// group records into streams keyed by {service_name, severity_text}
	streamsMap := make(map[string]*lokiStream)

	for _, rl := range req.GetResourceLogs() {
		serviceName := ""
		for _, attr := range rl.GetResource().GetAttributes() {
			if attr.GetKey() == "service.name" {
				serviceName = attr.GetValue().GetStringValue()
				break
			}
		}

		for _, sl := range rl.GetScopeLogs() {
			for _, lr := range sl.GetLogRecords() {
				severityText := lr.GetSeverityText()
				if severityText == "" {
					severityText = "INFO"
				}

				key := serviceName + "|" + severityText
				stream, ok := streamsMap[key]
				if !ok {
					streamsMap[key] = &lokiStream{
						Stream: map[string]string{
							"service_name": serviceName,
							"severity":     severityText,
						},
						Values: nil,
					}
					stream = streamsMap[key]
				}

				ts := strconv.FormatUint(lr.GetTimeUnixNano(), 10)
				body := bodyToString(lr.GetBody())
				stream.Values = append(stream.Values, [2]string{ts, body})
			}
		}
	}

	if len(streamsMap) == 0 {
		return nil
	}

	payload := lokiPushPayload{}
	for _, s := range streamsMap {
		payload.Streams = append(payload.Streams, *s)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("loki: marshal payload: %w", err)
	}

	url := l.endpoint + "/loki/api/v1/push"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("loki: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("loki: push request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("loki: unexpected status %d", resp.StatusCode)
	}

	return nil
}

func (l *LokiExporter) Shutdown(_ context.Context) error { return nil }

func bodyToString(v *commonpb.AnyValue) string {
	if v == nil {
		return ""
	}
	switch val := v.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return val.StringValue
	case *commonpb.AnyValue_IntValue:
		return strconv.FormatInt(val.IntValue, 10)
	case *commonpb.AnyValue_DoubleValue:
		return strconv.FormatFloat(val.DoubleValue, 'f', -1, 64)
	case *commonpb.AnyValue_BoolValue:
		return strconv.FormatBool(val.BoolValue)
	default:
		return fmt.Sprintf("%v", v)
	}
}
