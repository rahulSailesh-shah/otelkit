package receiver

import (
	"context"
	"encoding/json"
	"log"

	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	metricTypeGauge     = 1
	metricTypeSum       = 2
	metricTypeHistogram = 3
)

type MetricsHandler struct {
	repo *repo.Queries
	colmetricspb.UnimplementedMetricsServiceServer
}

func NewMetricsHandler(repo *repo.Queries) *MetricsHandler {
	return &MetricsHandler{repo: repo}
}

func (h *MetricsHandler) Export(
	ctx context.Context,
	req *colmetricspb.ExportMetricsServiceRequest,
) (*colmetricspb.ExportMetricsServiceResponse, error) {
	params, err := normalizeMetrics(req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "normalize metrics: %v", err)
	}

	for _, p := range params {
		if err := h.repo.InsertMetricPoint(ctx, p); err != nil {
			log.Printf("failed to insert metric point: %v", err)
		}
	}

	return &colmetricspb.ExportMetricsServiceResponse{}, nil
}

// normalizeMetrics walks the OTLP proto hierarchy and returns a flat list of InsertMetricPointParams.
func normalizeMetrics(req *colmetricspb.ExportMetricsServiceRequest) ([]repo.InsertMetricPointParams, error) {
	if req == nil {
		return nil, nil
	}

	var params []repo.InsertMetricPointParams
	for _, rm := range req.GetResourceMetrics() {
		resourceAttrs, serviceName := extractResourceAttrs(rm.GetResource())
		for _, sm := range rm.GetScopeMetrics() {
			for _, m := range sm.GetMetrics() {
				pts := extractDataPoints(m, serviceName, resourceAttrs)
				params = append(params, pts...)
			}
		}
	}
	return params, nil
}

func extractDataPoints(m *metricspb.Metric, serviceName string, resourceAttrs map[string]any) []repo.InsertMetricPointParams {
	description := m.GetDescription()
	unit := m.GetUnit()
	resourceAttrsJSON := mapToJSONString(resourceAttrs)

	base := repo.InsertMetricPointParams{
		Name:          m.GetName(),
		Description:   strPtrIfNotEmpty(description),
		Unit:          strPtrIfNotEmpty(unit),
		ServiceName:   serviceName,
		ResourceAttrs: resourceAttrsJSON,
	}

	switch data := m.GetData().(type) {
	case *metricspb.Metric_Gauge:
		return numberDataPoints(base, int64(metricTypeGauge), data.Gauge.GetDataPoints())
	case *metricspb.Metric_Sum:
		return numberDataPoints(base, int64(metricTypeSum), data.Sum.GetDataPoints())
	case *metricspb.Metric_Histogram:
		return histogramDataPoints(base, data.Histogram.GetDataPoints())
	default:
		log.Printf("unsupported metric data type for %q, skipping", m.GetName())
		return nil
	}
}

func numberDataPoints(base repo.InsertMetricPointParams, typ int64, dps []*metricspb.NumberDataPoint) []repo.InsertMetricPointParams {
	pts := make([]repo.InsertMetricPointParams, 0, len(dps))
	for _, dp := range dps {
		p := base
		p.Type = typ
		p.TimestampNs = int64(dp.GetTimeUnixNano())
		p.Attributes = mapToJSONString(keyValuesToMap(dp.GetAttributes()))

		switch v := dp.GetValue().(type) {
		case *metricspb.NumberDataPoint_AsInt:
			val := v.AsInt
			p.ValueInt = &val
		case *metricspb.NumberDataPoint_AsDouble:
			val := v.AsDouble
			p.ValueDouble = &val
		}
		pts = append(pts, p)
	}
	return pts
}

func histogramDataPoints(base repo.InsertMetricPointParams, dps []*metricspb.HistogramDataPoint) []repo.InsertMetricPointParams {
	pts := make([]repo.InsertMetricPointParams, 0, len(dps))
	for _, dp := range dps {
		p := base
		p.Type = int64(metricTypeHistogram)
		p.TimestampNs = int64(dp.GetTimeUnixNano())
		p.Attributes = mapToJSONString(keyValuesToMap(dp.GetAttributes()))

		count := int64(dp.GetCount())
		p.HistCount = &count

		if dp.Sum != nil {
			sum := dp.GetSum()
			p.HistSum = &sum
		}

		if bounds := dp.GetExplicitBounds(); len(bounds) > 0 {
			boundsJSON, _ := json.Marshal(bounds)
			boundsStr := string(boundsJSON)
			p.HistBounds = &boundsStr
		}

		if counts := dp.GetBucketCounts(); len(counts) > 0 {
			countsInt := make([]int64, len(counts))
			for i, c := range counts {
				countsInt[i] = int64(c)
			}
			countsJSON, _ := json.Marshal(countsInt)
			countsStr := string(countsJSON)
			p.HistCounts = &countsStr
		}

		pts = append(pts, p)
	}
	return pts
}

func extractResourceAttrs(resource *resourcepb.Resource) (map[string]any, string) {
	if resource == nil {
		return nil, ""
	}
	attrs := keyValuesToMap(resource.GetAttributes())
	serviceName, _ := attrs["service.name"].(string)
	return attrs, serviceName
}

func strPtrIfNotEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func mapToJSONString(m map[string]any) *string {
	if len(m) == 0 {
		return nil
	}
	data, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	s := string(data)
	return &s
}
