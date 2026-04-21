package export

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/otlptranslator"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
)

// otlpCollector implements prometheus.Collector by accumulating OTLP metrics
// and converting them on each Prometheus scrape.
type otlpCollector struct {
	mu          sync.RWMutex
	families    map[string]*metricFamily
	metricNamer otlptranslator.MetricNamer
	labelNamer  otlptranslator.LabelNamer
}

type metricFamily struct {
	metric         *metricspb.Metric
	resourceLabels map[string]string
}

func newOTLPCollector() *otlpCollector {
	return &otlpCollector{
		families: make(map[string]*metricFamily),
		metricNamer: otlptranslator.MetricNamer{
			WithMetricSuffixes: true,
			UTF8Allowed:        false,
		},
		labelNamer: otlptranslator.LabelNamer{
			UTF8Allowed: false,
		},
	}
}

func (c *otlpCollector) Describe(ch chan<- *prometheus.Desc) {}

func (c *otlpCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, fam := range c.families {
		c.collectMetric(ch, fam.metric, fam.resourceLabels)
	}
}

func (c *otlpCollector) collectMetric(ch chan<- prometheus.Metric, m *metricspb.Metric, resourceLabels map[string]string) {
	name := c.translateMetricName(m)

	switch data := m.GetData().(type) {
	case *metricspb.Metric_Gauge:
		c.collectGauge(ch, name, m.GetDescription(), data.Gauge, resourceLabels)
	case *metricspb.Metric_Sum:
		if data.Sum.IsMonotonic {
			c.collectCounter(ch, name, m.GetDescription(), data.Sum, resourceLabels)
		} else {
			c.collectGauge(ch, name, m.GetDescription(), &metricspb.Gauge{DataPoints: data.Sum.GetDataPoints()}, resourceLabels)
		}
	case *metricspb.Metric_Histogram:
		c.collectHistogram(ch, name, m.GetDescription(), data.Histogram, resourceLabels)
	case *metricspb.Metric_Summary:
		c.collectSummary(ch, name, m.GetDescription(), data.Summary, resourceLabels)
	}
}

func (c *otlpCollector) translateMetricName(m *metricspb.Metric) string {
	var metricType otlptranslator.MetricType
	switch data := m.GetData().(type) {
	case *metricspb.Metric_Gauge:
		metricType = otlptranslator.MetricTypeGauge
		_ = data
	case *metricspb.Metric_Sum:
		if data.Sum.IsMonotonic {
			metricType = otlptranslator.MetricTypeMonotonicCounter
		} else {
			metricType = otlptranslator.MetricTypeGauge
		}
	case *metricspb.Metric_Histogram:
		metricType = otlptranslator.MetricTypeHistogram
	case *metricspb.Metric_Summary:
		metricType = otlptranslator.MetricTypeSummary
	}

	translated, err := c.metricNamer.Build(otlptranslator.Metric{
		Name: m.GetName(),
		Unit: m.GetUnit(),
		Type: metricType,
	})
	if err != nil {
		// Fallback to raw name with basic sanitization
		return m.GetName()
	}
	return translated
}

func (c *otlpCollector) translateLabels(resourceLabels map[string]string, attrs []*commonpb.KeyValue) ([]string, []string) {
	merged := mergeLabels(resourceLabels, attributesToMap(attrs))

	keys := make([]string, 0, len(merged))
	vals := make([]string, 0, len(merged))
	for k, v := range merged {
		translated, err := c.labelNamer.Build(k)
		if err != nil {
			translated = k
		}
		keys = append(keys, translated)
		vals = append(vals, v)
	}
	return keys, vals
}

func (c *otlpCollector) collectGauge(ch chan<- prometheus.Metric, name, help string, gauge *metricspb.Gauge, resourceLabels map[string]string) {
	for _, dp := range gauge.GetDataPoints() {
		labelKeys, labelVals := c.translateLabels(resourceLabels, dp.GetAttributes())
		desc := prometheus.NewDesc(name, help, labelKeys, nil)

		var val float64
		switch v := dp.GetValue().(type) {
		case *metricspb.NumberDataPoint_AsInt:
			val = float64(v.AsInt)
		case *metricspb.NumberDataPoint_AsDouble:
			val = v.AsDouble
		}

		m, err := prometheus.NewConstMetric(desc, prometheus.GaugeValue, val, labelVals...)
		if err != nil {
			log.Printf("prometheus: gauge %s: %v", name, err)
			continue
		}
		ch <- m
	}
}

func (c *otlpCollector) collectCounter(ch chan<- prometheus.Metric, name, help string, sum *metricspb.Sum, resourceLabels map[string]string) {
	for _, dp := range sum.GetDataPoints() {
		labelKeys, labelVals := c.translateLabels(resourceLabels, dp.GetAttributes())
		desc := prometheus.NewDesc(name, help, labelKeys, nil)

		var val float64
		switch v := dp.GetValue().(type) {
		case *metricspb.NumberDataPoint_AsInt:
			val = float64(v.AsInt)
		case *metricspb.NumberDataPoint_AsDouble:
			val = v.AsDouble
		}

		m, err := prometheus.NewConstMetric(desc, prometheus.CounterValue, val, labelVals...)
		if err != nil {
			log.Printf("prometheus: counter %s: %v", name, err)
			continue
		}
		ch <- m
	}
}

func (c *otlpCollector) collectHistogram(ch chan<- prometheus.Metric, name, help string, histogram *metricspb.Histogram, resourceLabels map[string]string) {
	for _, dp := range histogram.GetDataPoints() {
		labelKeys, labelVals := c.translateLabels(resourceLabels, dp.GetAttributes())
		desc := prometheus.NewDesc(name, help, labelKeys, nil)

		bounds := dp.GetExplicitBounds()
		bucketCounts := dp.GetBucketCounts()
		buckets := make(map[float64]uint64, len(bounds))

		var cumulativeCount uint64
		for i, bound := range bounds {
			if i < len(bucketCounts) {
				cumulativeCount += bucketCounts[i]
			}
			buckets[bound] = cumulativeCount
		}

		count := dp.GetCount()
		sum := dp.GetSum()

		m, err := prometheus.NewConstHistogram(
			desc,
			count,
			sum,
			buckets,
			labelVals...,
		)
		if err != nil {
			log.Printf("prometheus: histogram %s: %v", name, err)
			continue
		}
		ch <- m
	}
}

func (c *otlpCollector) collectSummary(ch chan<- prometheus.Metric, name, help string, summary *metricspb.Summary, resourceLabels map[string]string) {
	for _, dp := range summary.GetDataPoints() {
		labelKeys, labelVals := c.translateLabels(resourceLabels, dp.GetAttributes())
		desc := prometheus.NewDesc(name, help, labelKeys, nil)

		quantiles := make(map[float64]float64)
		for _, qv := range dp.GetQuantileValues() {
			quantiles[qv.GetQuantile()] = qv.GetValue()
		}

		m, err := prometheus.NewConstSummary(
			desc,
			dp.GetCount(),
			dp.GetSum(),
			quantiles,
			labelVals...,
		)
		if err != nil {
			log.Printf("prometheus: summary %s: %v", name, err)
			continue
		}
		ch <- m
	}
}

// update replaces the stored metrics with fresh data from an OTLP export request.
func (c *otlpCollector) update(req *colmetricspb.ExportMetricsServiceRequest) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.families = make(map[string]*metricFamily)

	for _, rm := range req.GetResourceMetrics() {
		resourceLabels := extractResourceLabels(rm.GetResource())
		for _, sm := range rm.GetScopeMetrics() {
			for _, m := range sm.GetMetrics() {
				key := metricKey(m, resourceLabels)
				c.families[key] = &metricFamily{
					metric:         m,
					resourceLabels: resourceLabels,
				}
			}
		}
	}
}

func metricKey(m *metricspb.Metric, resourceLabels map[string]string) string {
	return fmt.Sprintf("%s_%v", m.GetName(), resourceLabels)
}

type PrometheusExporter struct {
	addr      string
	server    *http.Server
	collector *otlpCollector
}

func NewPrometheusExporter(addr string) (*PrometheusExporter, error) {
	if addr == "" {
		addr = ":9091"
	}

	registry := prometheus.NewRegistry()
	collector := newOTLPCollector()

	registry.MustRegister(collector)

	p := &PrometheusExporter{
		addr:      addr,
		collector: collector,
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
	p.collector.update(req)
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
	return attributesToMap(resource.GetAttributes())
}

func attributesToMap(attrs []*commonpb.KeyValue) map[string]string {
	if len(attrs) == 0 {
		return nil
	}
	labels := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		key := attr.GetKey()
		switch v := attr.GetValue().GetValue().(type) {
		case *commonpb.AnyValue_StringValue:
			labels[key] = v.StringValue
		case *commonpb.AnyValue_BoolValue:
			labels[key] = strconv.FormatBool(v.BoolValue)
		case *commonpb.AnyValue_IntValue:
			labels[key] = strconv.FormatInt(v.IntValue, 10)
		case *commonpb.AnyValue_DoubleValue:
			labels[key] = strconv.FormatFloat(v.DoubleValue, 'f', -1, 64)
		case *commonpb.AnyValue_ArrayValue:
			labels[key] = fmt.Sprintf("%v", v.ArrayValue.GetValues())
		}
	}
	return labels
}

func mergeLabels(a, b map[string]string) map[string]string {
	result := make(map[string]string, len(a)+len(b))
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}

// suppress unused import warning
var _ = math.Inf
