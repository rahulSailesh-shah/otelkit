package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

func f64Ptr(f float64) *float64 { return &f }

// --- attrLabel ---

func TestAttrLabelHTTPMethodAndStatus(t *testing.T) {
	raw := `{"http.request.method":"GET","http.response.status_code":"200"}`
	if got := attrLabel(strPtr(raw)); got != "GET 200" {
		t.Errorf("got %q, want %q", got, "GET 200")
	}
}

func TestAttrLabelMethod(t *testing.T) {
	raw := `{"method":"sql.conn.exec"}`
	if got := attrLabel(strPtr(raw)); got != "sql.conn.exec" {
		t.Errorf("got %q, want %q", got, "sql.conn.exec")
	}
}

func TestAttrLabelHTTPMethodOnly(t *testing.T) {
	raw := `{"http.request.method":"POST"}`
	if got := attrLabel(strPtr(raw)); got != "POST" {
		t.Errorf("got %q, want %q", got, "POST")
	}
}

func TestAttrLabelStatus(t *testing.T) {
	raw := `{"status":"idle"}`
	if got := attrLabel(strPtr(raw)); got != "idle" {
		t.Errorf("got %q, want %q", got, "idle")
	}
}

func TestAttrLabelFallback(t *testing.T) {
	raw := `{"region":"us-east-1","tier":"premium"}`
	if got := attrLabel(strPtr(raw)); got != "region=us-east-1 tier=premium" {
		t.Errorf("got %q, want %q", got, "region=us-east-1 tier=premium")
	}
}

func TestAttrLabelNil(t *testing.T) {
	if got := attrLabel(nil); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// --- pointValue ---

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

func TestPointValueHistogramAvg(t *testing.T) {
	p := repo.MetricPoint{Type: 3, HistSum: f64Ptr(100.0), HistCount: i64Ptr(4)}
	if got := pointValue(p); got != 25.0 {
		t.Errorf("got %v, want 25.0", got)
	}
}

func TestPointValueHistogramZeroCount(t *testing.T) {
	p := repo.MetricPoint{Type: 3, HistSum: f64Ptr(100.0), HistCount: i64Ptr(0)}
	if got := pointValue(p); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

// --- groupMetricPoints ---

func TestGroupMetricPointsEmpty(t *testing.T) {
	r := groupMetricPoints(nil)
	if len(r.Groups) != 0 || r.AggLatest != 0 || r.AggMin != 0 || r.AggMax != 0 || r.AggAvg != 0 {
		t.Error("empty input should return zero result")
	}
}

func TestGroupMetricPointsSingleGroup(t *testing.T) {
	pts := []repo.MetricPoint{
		{Type: 1, Attributes: strPtr(`{"method":"exec"}`), ValueDouble: f64Ptr(10)},
		{Type: 1, Attributes: strPtr(`{"method":"exec"}`), ValueDouble: f64Ptr(20)},
	}
	r := groupMetricPoints(pts)
	if len(r.Groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(r.Groups))
	}
	if r.Groups[0].Label != "exec" {
		t.Errorf("label = %q, want exec", r.Groups[0].Label)
	}
	if r.Groups[0].Latest != 20 || r.Groups[0].Min != 10 || r.Groups[0].Max != 20 || r.Groups[0].Avg != 15 {
		t.Errorf("stats: latest=%v min=%v max=%v avg=%v", r.Groups[0].Latest, r.Groups[0].Min, r.Groups[0].Max, r.Groups[0].Avg)
	}
	if r.AggAvg != 15 {
		t.Errorf("agg avg = %v, want 15", r.AggAvg)
	}
}

func TestGroupMetricPointsTwoGroups(t *testing.T) {
	pts := []repo.MetricPoint{
		{Type: 1, Attributes: strPtr(`{"method":"exec"}`), ValueDouble: f64Ptr(10)},
		{Type: 1, Attributes: strPtr(`{"method":"query"}`), ValueDouble: f64Ptr(30)},
	}
	r := groupMetricPoints(pts)
	if len(r.Groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(r.Groups))
	}
	if r.Groups[0].Label != "exec" || r.Groups[1].Label != "query" {
		t.Errorf("labels = [%q %q], want [exec query]", r.Groups[0].Label, r.Groups[1].Label)
	}
	if r.AggAvg != 20 {
		t.Errorf("agg avg = %v, want 20", r.AggAvg)
	}
}

func TestGroupMetricPointsPreservesUnit(t *testing.T) {
	unit := "ms"
	pts := []repo.MetricPoint{
		{Type: 2, Unit: &unit, Attributes: strPtr(`{}`), ValueDouble: f64Ptr(5)},
	}
	r := groupMetricPoints(pts)
	if r.Unit != "ms" || r.Type != 2 {
		t.Errorf("unit=%q type=%d, want ms/2", r.Unit, r.Type)
	}
}

// --- metricsModel ---

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
	m.setSource(context.Background(), &repo.Queries{})
	m.setNames([]MetricNameRow{{Name: "a"}, {Name: "b"}})
	m.selected = "a"

	if _, cmd := m.Update(context.Background(), &repo.Queries{}, struct{}{}); cmd != nil {
		t.Fatalf("no-op update should not emit a cmd, got %T", cmd)
	}

	down := tea.KeyPressMsg{Code: 'j', Text: "j"}
	m2, cmd := m.Update(context.Background(), &repo.Queries{}, down)
	if cmd == nil {
		t.Fatalf("selection change should emit a groups-load cmd")
	}
	if m2.(*metricsModel).selected != "b" {
		t.Fatalf("selected after move = %q, want b", m2.(*metricsModel).selected)
	}

	if _, cmd := m2.Update(context.Background(), &repo.Queries{}, struct{}{}); cmd != nil {
		t.Fatalf("cursor held at b should not emit a cmd, got %T", cmd)
	}
}

func TestMetricsModelSetGroups(t *testing.T) {
	m := newMetricsModel()
	m.setSize(100, 40)
	m.setNames([]MetricNameRow{{Name: "foo"}})
	m.setGroups(metricGroupsLoadedMsg{
		Name:      "foo",
		Groups:    []MetricGroupStats{{Label: "GET 200", Latest: 5, Min: 1, Max: 10, Avg: 5, Count: 3}},
		AggLatest: 5, AggMin: 1, AggMax: 10, AggAvg: 5,
		Unit: "s", Type: 3,
	})
	if m.selected != "foo" {
		t.Fatalf("selected = %q, want foo", m.selected)
	}
	if len(m.groups) != 1 {
		t.Fatalf("groups len = %d, want 1", len(m.groups))
	}
	if m.unit != "s" || m.metricType != 3 {
		t.Fatalf("unit=%q type=%d, want s/3", m.unit, m.metricType)
	}
}

func TestMetricsModelSetKPIs(t *testing.T) {
	m := newMetricsModel()
	kpis := KPIData{
		SQLExecLatency:  "2.00ms",
		AvgGETDuration:  "5.00ms",
		AvgPOSTDuration: "12.00ms",
		IdleConns:       "3",
	}
	m.setKPIs(kpis)
	if m.kpis != kpis {
		t.Fatalf("kpis mismatch: got %+v, want %+v", m.kpis, kpis)
	}
}
