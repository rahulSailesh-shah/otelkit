package receiver

import (
	"encoding/json"
	"testing"

	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
)

func ptr[T any](v T) *T { return &v }

func TestNormalizeMetrics_NilRequest(t *testing.T) {
	points, err := normalizeMetrics(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(points) != 0 {
		t.Fatalf("expected 0 points, got %d", len(points))
	}
}

func TestNormalizeMetrics_Gauge(t *testing.T) {
	req := &colmetricspb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "test-svc"}}},
					},
				},
				ScopeMetrics: []*metricspb.ScopeMetrics{
					{
						Metrics: []*metricspb.Metric{
							{
								Name:        "http_requests_total",
								Description: "Total requests",
								Unit:        "1",
								Data: &metricspb.Metric_Gauge{
									Gauge: &metricspb.Gauge{
										DataPoints: []*metricspb.NumberDataPoint{
											{
												TimeUnixNano: 1000000,
												Value:        &metricspb.NumberDataPoint_AsInt{AsInt: 42},
												Attributes: []*commonpb.KeyValue{
													{Key: "method", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "GET"}}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	points, err := normalizeMetrics(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	p := points[0]
	if p.Name != "http_requests_total" {
		t.Errorf("name: got %q, want %q", p.Name, "http_requests_total")
	}
	if p.Type != metricTypeGauge {
		t.Errorf("type: got %d, want 1 (gauge)", p.Type)
	}
	if p.ServiceName != "test-svc" {
		t.Errorf("service: got %q, want %q", p.ServiceName, "test-svc")
	}
	if p.ValueInt == nil || *p.ValueInt != 42 {
		t.Errorf("value_int: got %v, want 42", p.ValueInt)
	}
	if p.TimestampNs != 1000000 {
		t.Errorf("timestamp_ns: got %d, want 1000000", p.TimestampNs)
	}
	// Attributes is a JSON string, need to unmarshal to access
	var attrs map[string]string
	if p.Attributes != nil {
		if err := json.Unmarshal([]byte(*p.Attributes), &attrs); err != nil {
			t.Fatalf("failed to unmarshal attributes: %v", err)
		}
	}
	if attrs["method"] != "GET" {
		t.Errorf("attributes method: got %v, want GET", attrs["method"])
	}
}

func TestNormalizeMetrics_Sum(t *testing.T) {
	req := &colmetricspb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "test-svc"}}},
					},
				},
				ScopeMetrics: []*metricspb.ScopeMetrics{
					{
						Metrics: []*metricspb.Metric{
							{
								Name: "db_operations_total",
								Data: &metricspb.Metric_Sum{
									Sum: &metricspb.Sum{
										DataPoints: []*metricspb.NumberDataPoint{
											{
												TimeUnixNano: 2000000,
												Value:        &metricspb.NumberDataPoint_AsDouble{AsDouble: 3.14},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	points, err := normalizeMetrics(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	p := points[0]
	if p.Type != metricTypeSum {
		t.Errorf("type: got %d, want 2 (sum)", p.Type)
	}
	if p.Name != "db_operations_total" {
		t.Errorf("name: got %q, want %q", p.Name, "db_operations_total")
	}
	if p.ServiceName != "test-svc" {
		t.Errorf("service: got %q, want %q", p.ServiceName, "test-svc")
	}
	if p.ValueDouble == nil || *p.ValueDouble != 3.14 {
		t.Errorf("value_double: got %v, want 3.14", p.ValueDouble)
	}
}

func TestNormalizeMetrics_Histogram(t *testing.T) {
	req := &colmetricspb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "test-svc"}}},
					},
				},
				ScopeMetrics: []*metricspb.ScopeMetrics{
					{
						Metrics: []*metricspb.Metric{
							{
								Name: "http_request_duration_seconds",
								Data: &metricspb.Metric_Histogram{
									Histogram: &metricspb.Histogram{
										DataPoints: []*metricspb.HistogramDataPoint{
											{
												TimeUnixNano:   3000000,
												Count:          10,
												Sum:            ptr(5.5),
												ExplicitBounds: []float64{0.1, 0.5, 1.0},
												BucketCounts:   []uint64{2, 5, 2, 1},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	points, err := normalizeMetrics(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	p := points[0]
	if p.Type != metricTypeHistogram {
		t.Errorf("type: got %d, want 3 (histogram)", p.Type)
	}
	if p.ServiceName != "test-svc" {
		t.Errorf("service: got %q, want %q", p.ServiceName, "test-svc")
	}
	if p.HistCount == nil || *p.HistCount != 10 {
		t.Errorf("hist_count: got %v, want 10", p.HistCount)
	}
	if p.HistSum == nil || *p.HistSum != 5.5 {
		t.Errorf("hist_sum: got %v, want 5.5", p.HistSum)
	}
	// HistBounds and HistCounts are JSON strings, need to unmarshal to access
	var bounds []float64
	if p.HistBounds != nil {
		if err := json.Unmarshal([]byte(*p.HistBounds), &bounds); err != nil {
			t.Fatalf("failed to unmarshal hist_bounds: %v", err)
		}
	}
	if len(bounds) != 3 || bounds[0] != 0.1 {
		t.Errorf("hist_bounds: got %v, want [0.1, 0.5, 1.0]", bounds)
	}

	var counts []int64
	if p.HistCounts != nil {
		if err := json.Unmarshal([]byte(*p.HistCounts), &counts); err != nil {
			t.Fatalf("failed to unmarshal hist_counts: %v", err)
		}
	}
	if len(counts) != 4 || counts[0] != 2 {
		t.Errorf("hist_counts: got %v, want [2, 5, 2, 1]", counts)
	}
}
