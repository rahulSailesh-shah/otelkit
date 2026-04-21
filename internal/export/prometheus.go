package export

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
)

type PrometheusExporter struct {
	addr     string
	registry *prometheus.Registry
	server   *http.Server
}

func NewPrometheusExporter(addr string) (*PrometheusExporter, error) {
	if addr == "" {
		addr = ":9091"
	}

	registry := prometheus.NewRegistry()

	p := &PrometheusExporter{
		addr:     addr,
		registry: registry,
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	p.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Printf("Prometheus exporter listening on %s", addr)
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Prometheus exporter server error: %v", err)
		}
	}()

	return p, nil
}

func (p *PrometheusExporter) Name() string { return "prometheus" }

func (p *PrometheusExporter) ExportMetrics(ctx context.Context, req *colmetricspb.ExportMetricsServiceRequest) error {
	if req == nil {
		return nil
	}

	for _, rm := range req.GetResourceMetrics() {
		resourceLabels := extractResourceLabels(rm.GetResource())
		for _, sm := range rm.GetScopeMetrics() {
			for _, m := range sm.GetMetrics() {
				if err := p.processMetric(m, resourceLabels); err != nil {
					log.Printf("prometheus exporter: process metric %s: %v", m.GetName(), err)
				}
			}
		}
	}

	return nil
}

func (p *PrometheusExporter) processMetric(m *metricspb.Metric, resourceLabels map[string]string) error {
	name := sanitizeMetricName(m.GetName())
	help := m.GetDescription()

	switch data := m.GetData().(type) {
	case *metricspb.Metric_Gauge:
		return p.processGauge(name, help, data.Gauge, resourceLabels)
	case *metricspb.Metric_Sum:
		if data.Sum.IsMonotonic {
			return p.processCounter(name, help, data.Sum, resourceLabels)
		}
		return p.processSumAsGauge(name, help, data.Sum, resourceLabels)
	case *metricspb.Metric_Histogram:
		return p.processHistogram(name, help, data.Histogram, resourceLabels)
	default:
		return fmt.Errorf("unsupported metric type: %T", data)
	}
}

func (p *PrometheusExporter) processGauge(name, help string, gauge *metricspb.Gauge, resourceLabels map[string]string) error {
	for _, dp := range gauge.GetDataPoints() {
		labels := mergeLabels(resourceLabels, attributesToLabels(dp.GetAttributes()))
		gauge := prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        name,
			Help:        help,
			ConstLabels: labels,
		})
		if err := p.registry.Register(gauge); err != nil {
			p.registry.Unregister(gauge)
			if err := p.registry.Register(gauge); err != nil {
				return fmt.Errorf("register gauge: %w", err)
			}
		}

		switch v := dp.GetValue().(type) {
		case *metricspb.NumberDataPoint_AsInt:
			gauge.Set(float64(v.AsInt))
		case *metricspb.NumberDataPoint_AsDouble:
			gauge.Set(v.AsDouble)
		}
	}
	return nil
}

func (p *PrometheusExporter) processSumAsGauge(name, help string, sum *metricspb.Sum, resourceLabels map[string]string) error {
	for _, dp := range sum.GetDataPoints() {
		labels := mergeLabels(resourceLabels, attributesToLabels(dp.GetAttributes()))
		gauge := prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        name,
			Help:        help,
			ConstLabels: labels,
		})
		if err := p.registry.Register(gauge); err != nil {
			p.registry.Unregister(gauge)
			if err := p.registry.Register(gauge); err != nil {
				return fmt.Errorf("register gauge: %w", err)
			}
		}

		switch v := dp.GetValue().(type) {
		case *metricspb.NumberDataPoint_AsInt:
			gauge.Set(float64(v.AsInt))
		case *metricspb.NumberDataPoint_AsDouble:
			gauge.Set(v.AsDouble)
		}
	}
	return nil
}

func (p *PrometheusExporter) processCounter(name, help string, sum *metricspb.Sum, resourceLabels map[string]string) error {
	for _, dp := range sum.GetDataPoints() {
		labels := mergeLabels(resourceLabels, attributesToLabels(dp.GetAttributes()))
		counter := prometheus.NewCounter(prometheus.CounterOpts{
			Name:        name,
			Help:        help,
			ConstLabels: labels,
		})
		if err := p.registry.Register(counter); err != nil {
			p.registry.Unregister(counter)
			if err := p.registry.Register(counter); err != nil {
				return fmt.Errorf("register counter: %w", err)
			}
		}

		switch v := dp.GetValue().(type) {
		case *metricspb.NumberDataPoint_AsInt:
			counter.Add(float64(v.AsInt))
		case *metricspb.NumberDataPoint_AsDouble:
			counter.Add(v.AsDouble)
		}
	}
	return nil
}

func (p *PrometheusExporter) processHistogram(name, help string, histogram *metricspb.Histogram, resourceLabels map[string]string) error {
	for _, dp := range histogram.GetDataPoints() {
		labels := mergeLabels(resourceLabels, attributesToLabels(dp.GetAttributes()))
		buckets := dp.GetExplicitBounds()
		if len(buckets) == 0 {
			buckets = prometheus.DefBuckets
		}
		hist := prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:        name,
			Help:        help,
			Buckets:     buckets,
			ConstLabels: labels,
		})
		if err := p.registry.Register(hist); err != nil {
			p.registry.Unregister(hist)
			if err := p.registry.Register(hist); err != nil {
				return fmt.Errorf("register histogram: %w", err)
			}
		}

		if dp.GetCount() > 0 {
			hist.Observe(dp.GetSum() / float64(dp.GetCount()))
		}
	}
	return nil
}

func (p *PrometheusExporter) Shutdown(ctx context.Context) error {
	if p.server != nil {
		return p.server.Shutdown(ctx)
	}
	return nil
}

func extractResourceLabels(resource *resourcepb.Resource) map[string]string {
	if resource == nil {
		return nil
	}
	return attributesToLabels(resource.GetAttributes())
}

func attributesToLabels(attrs []*commonpb.KeyValue) map[string]string {
	if len(attrs) == 0 {
		return nil
	}
	labels := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		key := sanitizeLabelName(attr.GetKey())
		switch v := attr.GetValue().GetValue().(type) {
		case *commonpb.AnyValue_StringValue:
			labels[key] = v.StringValue
		case *commonpb.AnyValue_BoolValue:
			labels[key] = strconv.FormatBool(v.BoolValue)
		case *commonpb.AnyValue_IntValue:
			labels[key] = strconv.FormatInt(v.IntValue, 10)
		case *commonpb.AnyValue_ArrayValue:
			labels[key] = fmt.Sprintf("%v", v.ArrayValue.GetValues())
		case *commonpb.AnyValue_DoubleValue:
			labels[key] = strconv.FormatFloat(v.DoubleValue, 'f', -1, 64)
		}
	}
	return labels
}

func mergeLabels(a, b map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}

func sanitizeMetricName(name string) string {
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "-", "_")
	return name
}

func sanitizeLabelName(name string) string {
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "-", "_")
	return name
}
