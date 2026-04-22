package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

func f64Ptr(f float64) *float64 { return &f }

func TestPointValueGaugeDouble(t *testing.T) {
	p := repo.MetricPoint{Type: 1, ValueDouble: f64Ptr(3.14)}
	if got := pointValue(p); got != 3.14 {
		t.Errorf("got %v, want 3.14", got)
	}
}

func TestPointValueGaugeIntFallback(t *testing.T) {
	p := repo.MetricPoint{Type: 1, ValueInt: i64Ptr(42)}
	if got := pointValue(p); got != 42 {
		t.Errorf("got %v, want 42", got)
	}
}

func TestPointValueGaugeBothNil(t *testing.T) {
	p := repo.MetricPoint{Type: 1}
	if got := pointValue(p); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

func TestPointValueSumDouble(t *testing.T) {
	p := repo.MetricPoint{Type: 2, ValueDouble: f64Ptr(99.5)}
	if got := pointValue(p); got != 99.5 {
		t.Errorf("got %v, want 99.5", got)
	}
}

func TestPointValueHistogramAvg(t *testing.T) {
	p := repo.MetricPoint{
		Type:      3,
		HistSum:   f64Ptr(100.0),
		HistCount: i64Ptr(4),
	}
	if got := pointValue(p); got != 25.0 {
		t.Errorf("got %v, want 25.0", got)
	}
}

func TestPointValueHistogramZeroCount(t *testing.T) {
	p := repo.MetricPoint{
		Type:      3,
		HistSum:   f64Ptr(100.0),
		HistCount: i64Ptr(0),
	}
	if got := pointValue(p); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

func TestPointValueHistogramNilSum(t *testing.T) {
	p := repo.MetricPoint{Type: 3, HistCount: i64Ptr(4)}
	if got := pointValue(p); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

func TestMetricStatsEmpty(t *testing.T) {
	latest, mn, mx, avg, n := metricStats(nil)
	if latest != 0 || mn != 0 || mx != 0 || avg != 0 || n != 0 {
		t.Errorf("empty should return zeros, got latest=%v min=%v max=%v avg=%v n=%d", latest, mn, mx, avg, n)
	}
}

func TestMetricStatsSingle(t *testing.T) {
	pts := []MetricSeriesPoint{{TimestampNs: 1, Value: 5}}
	latest, mn, mx, avg, n := metricStats(pts)
	if latest != 5 || mn != 5 || mx != 5 || avg != 5 || n != 1 {
		t.Errorf("single: got latest=%v min=%v max=%v avg=%v n=%d", latest, mn, mx, avg, n)
	}
}

func TestMetricStatsMulti(t *testing.T) {
	pts := []MetricSeriesPoint{
		{TimestampNs: 1, Value: 10},
		{TimestampNs: 2, Value: 20},
		{TimestampNs: 3, Value: 30},
	}
	latest, mn, mx, avg, n := metricStats(pts)
	if latest != 30 || mn != 10 || mx != 30 || avg != 20 || n != 3 {
		t.Errorf("multi: got latest=%v min=%v max=%v avg=%v n=%d", latest, mn, mx, avg, n)
	}
}

func TestMetricsModelSetNames(t *testing.T) {
	m := newMetricsModel()
	m.setNames([]MetricNameRow{{Name: "a"}, {Name: "b"}})
	if len(m.names) != 2 {
		t.Fatalf("names len = %d, want 2", len(m.names))
	}
	name, ok := m.selectedName()
	if !ok || name != "a" {
		t.Fatalf("selectedName = %q ok=%v, want a/true", name, ok)
	}
}

func TestMetricsModelSelectedEmpty(t *testing.T) {
	m := newMetricsModel()
	if _, ok := m.selectedName(); ok {
		t.Fatalf("empty model should return ok=false")
	}
}

func TestMetricsModelSelectionTriggersLoad(t *testing.T) {
	m := newMetricsModel()
	m.setSize(100, 40)
	// Give the model a source so Update can build a non-nil series cmd.
	m.setSource(context.Background(), &repo.Queries{})
	m.setNames([]MetricNameRow{{Name: "a"}, {Name: "b"}})
	// Prime selected to match current cursor so same-cursor update is a no-op.
	m.selected = "a"

	// Same cursor (no movement) → no selection-triggered cmd.
	if _, cmd := m.Update(struct{}{}); cmd != nil {
		t.Fatalf("no-op update should not emit a cmd, got %T", cmd)
	}

	// Move cursor down via table key binding ("j").
	down := tea.KeyPressMsg{Code: 'j', Text: "j"}
	m2, cmd := m.Update(down)
	if cmd == nil {
		t.Fatalf("selection change should emit a series-load cmd")
	}
	if m2.selected != "b" {
		t.Fatalf("selected after move = %q, want b", m2.selected)
	}

	// Second update at the same cursor: no fresh selection-change cmd.
	if _, cmd := m2.Update(struct{}{}); cmd != nil {
		t.Fatalf("cursor held at b should not emit a cmd, got %T", cmd)
	}
}

func TestMetricsModelSetSeries(t *testing.T) {
	m := newMetricsModel()
	m.setSize(100, 40)
	m.setNames([]MetricNameRow{{Name: "foo"}})
	pts := []MetricSeriesPoint{{TimestampNs: 1, Value: 1}, {TimestampNs: 2, Value: 2}}
	m.setSeries("foo", pts, "ms", 3)
	if m.selected != "foo" {
		t.Fatalf("selected = %q, want foo", m.selected)
	}
	if len(m.series) != 2 || m.unit != "ms" || m.metricType != 3 {
		t.Fatalf("setSeries state wrong: series=%d unit=%q type=%d", len(m.series), m.unit, m.metricType)
	}
}
