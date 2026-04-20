package receiver

import (
	"context"
	"log"

	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MetricPoint is a normalized representation of a single OTLP metric data point.
// Type: 1=gauge 2=sum 3=histogram
type MetricPoint struct {
	Name          string
	Description   string
	Unit          string
	Type          int
	ServiceName   string
	Attributes    map[string]any
	TimestampNs   int64
	ValueInt      *int64
	ValueDouble   *float64
	HistCount     *uint64
	HistSum       *float64
	HistBounds    []float64
	HistCounts    []uint64
	ResourceAttrs map[string]any
}

type MetricsHandler struct {
	colmetricspb.UnimplementedMetricsServiceServer
}

func NewMetricsHandler() *MetricsHandler {
	return &MetricsHandler{}
}

func (h *MetricsHandler) Export(
	ctx context.Context,
	req *colmetricspb.ExportMetricsServiceRequest,
) (*colmetricspb.ExportMetricsServiceResponse, error) {
	points, err := normalizeMetrics(req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "normalize metrics: %v", err)
	}
	log.Printf("received %d metric points", len(points))
	return &colmetricspb.ExportMetricsServiceResponse{}, nil
}

func normalizeMetrics(req *colmetricspb.ExportMetricsServiceRequest) ([]MetricPoint, error) {
	if req == nil {
		return nil, nil
	}

	var points []MetricPoint

	for _, rm := range req.GetResourceMetrics() {
		resourceAttrs, serviceName := extractResourceAttrs(rm.GetResource())

		for _, sm := range rm.GetScopeMetrics() {
			for _, m := range sm.GetMetrics() {
				pts := extractDataPoints(m, serviceName, resourceAttrs)
				points = append(points, pts...)
			}
		}
	}

	return points, nil
}

func extractDataPoints(m *metricspb.Metric, serviceName string, resourceAttrs map[string]any) []MetricPoint {
	base := MetricPoint{
		Name:          m.GetName(),
		Description:   m.GetDescription(),
		Unit:          m.GetUnit(),
		ServiceName:   serviceName,
		ResourceAttrs: resourceAttrs,
	}

	switch data := m.GetData().(type) {
	case *metricspb.Metric_Gauge:
		return numberDataPoints(base, 1, data.Gauge.GetDataPoints())

	case *metricspb.Metric_Sum:
		return numberDataPoints(base, 2, data.Sum.GetDataPoints())

	case *metricspb.Metric_Histogram:
		return histogramDataPoints(base, data.Histogram.GetDataPoints())

	default:
		log.Printf("unsupported metric data type for %q, skipping", m.GetName())
		return nil
	}
}

func numberDataPoints(base MetricPoint, typ int, dps []*metricspb.NumberDataPoint) []MetricPoint {
	pts := make([]MetricPoint, 0, len(dps))
	for _, dp := range dps {
		p := base
		p.Type = typ
		p.TimestampNs = int64(dp.GetTimeUnixNano())
		p.Attributes = keyValuesToMap(dp.GetAttributes())

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

func histogramDataPoints(base MetricPoint, dps []*metricspb.HistogramDataPoint) []MetricPoint {
	pts := make([]MetricPoint, 0, len(dps))
	for _, dp := range dps {
		p := base
		p.Type = 3
		p.TimestampNs = int64(dp.GetTimeUnixNano())
		p.Attributes = keyValuesToMap(dp.GetAttributes())

		count := dp.GetCount()
		p.HistCount = &count
		if dp.Sum != nil {
			sum := dp.GetSum()
			p.HistSum = &sum
		}
		p.HistBounds = dp.GetExplicitBounds()
		p.HistCounts = dp.GetBucketCounts()

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
