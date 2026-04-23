package tui

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func strPtr(s string) *string { return &s }
func i64Ptr(i int64) *int64   { return &i }

func TestSeverityLabel(t *testing.T) {
	cases := []struct {
		num      *int64
		text     *string
		wantText string
	}{
		{nil, nil, "-"},
		{nil, strPtr("custom"), "custom"},
		{i64Ptr(2), nil, "TRACE"},
		{i64Ptr(6), nil, "DEBUG"},
		{i64Ptr(9), nil, "INFO"},
		{i64Ptr(13), nil, "WARN"},
		{i64Ptr(17), nil, "ERROR"},
		{i64Ptr(21), nil, "FATAL"},
		{i64Ptr(999), nil, "-"},
	}
	for _, c := range cases {
		got := severityLabel(c.num, c.text)
		if got != c.wantText {
			t.Errorf("severityLabel(%v,%v) = %q, want %q", c.num, c.text, got, c.wantText)
		}
	}
}

func TestToLogRow(t *testing.T) {
	rec := repo.LogRecord{
		ID:            7,
		TraceID:       strPtr("abc123deadbeef"),
		SpanID:        strPtr("span123"),
		ServiceName:   "api",
		Severity:      i64Ptr(17),
		SeverityText:  strPtr("ERROR"),
		Body:          strPtr("boom"),
		Attributes:    strPtr(`{"k":"v"}`),
		ResourceAttrs: strPtr(`{"r":"1"}`),
		TimestampNs:   1_700_000_000_000_000_000,
	}
	row := toLogRow(rec)
	if row.ID != 7 || row.Service != "api" || row.TraceID != "abc123deadbeef" ||
		row.SpanID != "span123" || row.Body != "boom" || row.SeverityText != "ERROR" {
		t.Fatalf("unexpected row: %+v", row)
	}
	if row.Severity == nil || *row.Severity != 17 {
		t.Fatalf("severity not preserved: %+v", row.Severity)
	}
}

func TestToLogRowNilSafe(t *testing.T) {
	rec := repo.LogRecord{
		ID:          1,
		ServiceName: "svc",
		TimestampNs: 42,
	}
	row := toLogRow(rec)
	if row.TraceID != "" || row.SpanID != "" || row.Body != "" || row.SeverityText != "" {
		t.Fatalf("nil pointers should become empty strings: %+v", row)
	}
	if row.Severity != nil {
		t.Fatalf("nil severity should remain nil")
	}
}

func TestLogsModelSetRows(t *testing.T) {
	m := newLogsModel()
	rows := []LogRow{
		{ID: 1, TimestampNs: 1_700_000_000_000_000_000, Severity: i64Ptr(17), Service: "api", TraceID: "abcdef1234567890", Body: "boom"},
		{ID: 2, TimestampNs: 1_700_000_000_000_000_000, Severity: nil, SeverityText: "", Service: "svc", Body: "hi"},
	}
	m.setRows(rows)
	if len(m.rows) != 2 {
		t.Fatalf("rows not stored, got %d", len(m.rows))
	}
	got, ok := m.selectedLog()
	if !ok || got.ID != 1 {
		t.Fatalf("selectedLog expected ID=1, got %+v ok=%v", got, ok)
	}
}

func TestLogsModelSelectedLogEmpty(t *testing.T) {
	m := newLogsModel()
	if _, ok := m.selectedLog(); ok {
		t.Fatalf("empty model should return ok=false")
	}
}

func sampleLogRows() []LogRow {
	return []LogRow{
		{ID: 3, TimestampNs: 1_700_000_002_000_000_000, Severity: i64Ptr(9), Service: "api", TraceID: "trace-c", Body: "request complete"},
		{ID: 1, TimestampNs: 1_700_000_000_000_000_000, SeverityText: "ERROR", Service: "api", TraceID: "trace-a", Body: "database timeout"},
		{ID: 2, TimestampNs: 1_700_000_001_000_000_000, Severity: i64Ptr(13), Service: "worker", TraceID: "trace-b", Body: "retry scheduled"},
	}
}

func visibleServices(m logsModel) []string {
	rows := m.table.Rows()
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row[2])
	}
	return out
}

func visibleBodies(m logsModel) []string {
	rows := m.table.Rows()
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row[4])
	}
	return out
}

func keyRunes(text string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: text, Code: []rune(text)[0]})
}

func keyCode(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func TestApplyLogFiltersSeverity(t *testing.T) {
	m := newLogsModel()
	m.setRows(sampleLogRows())
	if got := visibleBodies(m); len(got) != 3 || got[0] != "request complete" || got[1] != "database timeout" || got[2] != "retry scheduled" {
		t.Fatalf("visible bodies before severity filter = %#v, want all rows in original order", got)
	}
	got, ok := m.selectedLog()
	if !ok || got.ID != 3 {
		t.Fatalf("selectedLog before severity filter = %+v ok=%v, want unfiltered ID 3", got, ok)
	}
	m.filters.Severity = "error"
	m.applyFilters()
	if got := visibleBodies(m); len(got) != 1 || got[0] != "database timeout" {
		t.Fatalf("visible bodies after severity filter = %#v, want only database timeout", got)
	}
	got, ok = m.selectedLog()
	if !ok || got.ID != 1 {
		t.Fatalf("severity filter selectedLog = %+v ok=%v, want only ID 1 visible", got, ok)
	}
}

func TestApplyLogFiltersSeverityWarnNumeric(t *testing.T) {
	m := newLogsModel()
	m.setRows(sampleLogRows())
	if got := visibleBodies(m); len(got) != 3 || got[0] != "request complete" || got[1] != "database timeout" || got[2] != "retry scheduled" {
		t.Fatalf("visible bodies before warn severity filter = %#v, want all rows in original order", got)
	}
	got, ok := m.selectedLog()
	if !ok || got.ID != 3 {
		t.Fatalf("selectedLog before warn severity filter = %+v ok=%v, want unfiltered ID 3", got, ok)
	}
	m.filters.Severity = "warn"
	m.applyFilters()
	if got := visibleBodies(m); len(got) != 1 || got[0] != "retry scheduled" {
		t.Fatalf("visible bodies after warn severity filter = %#v, want only retry scheduled", got)
	}
	got, ok = m.selectedLog()
	if !ok || got.ID != 2 {
		t.Fatalf("warn severity filter selectedLog = %+v ok=%v, want only ID 2 visible", got, ok)
	}
}

func TestApplyLogFiltersSeverityExactMatch(t *testing.T) {
	m := newLogsModel()
	m.setRows(sampleLogRows())
	if got := visibleBodies(m); len(got) != 3 || got[0] != "request complete" || got[1] != "database timeout" || got[2] != "retry scheduled" {
		t.Fatalf("visible bodies before exact severity filter = %#v, want all rows in original order", got)
	}
	m.filters.Severity = "err"
	m.applyFilters()
	if got := visibleBodies(m); len(got) != 0 {
		t.Fatalf("visible bodies after exact severity filter = %#v, want no visible rows", got)
	}
	if got, ok := m.selectedLog(); ok {
		t.Fatalf("selectedLog after exact severity filter = %+v ok=%v, want no selection", got, ok)
	}
}

func TestApplyLogFiltersServiceSubstring(t *testing.T) {
	m := newLogsModel()
	m.setRows(sampleLogRows())
	if got := visibleServices(m); len(got) != 3 || got[0] != "api" || got[1] != "api" || got[2] != "worker" {
		t.Fatalf("visible services before service filter = %#v, want all rows in original order", got)
	}
	got, ok := m.selectedLog()
	if !ok || got.ID != 3 {
		t.Fatalf("selectedLog before service filter = %+v ok=%v, want unfiltered ID 3", got, ok)
	}
	m.filters.Service = "WORK"
	m.applyFilters()
	if got := visibleServices(m); len(got) != 1 || got[0] != "worker" {
		t.Fatalf("visible services after service filter = %#v, want only worker", got)
	}
	got, ok = m.selectedLog()
	if !ok || got.ID != 2 {
		t.Fatalf("service filter selectedLog = %+v ok=%v, want only ID 2 visible", got, ok)
	}
}

func TestApplyLogFiltersBodySubstringCaseInsensitive(t *testing.T) {
	m := newLogsModel()
	m.setRows(sampleLogRows())
	if got := visibleBodies(m); len(got) != 3 || got[0] != "request complete" || got[1] != "database timeout" || got[2] != "retry scheduled" {
		t.Fatalf("visible bodies before body filter = %#v, want all rows in original order", got)
	}
	got, ok := m.selectedLog()
	if !ok || got.ID != 3 {
		t.Fatalf("selectedLog before body filter = %+v ok=%v, want unfiltered ID 3", got, ok)
	}
	m.filters.Body = "TIMEOUT"
	m.applyFilters()
	if got := visibleBodies(m); len(got) != 1 || got[0] != "database timeout" {
		t.Fatalf("visible bodies after body filter = %#v, want only database timeout", got)
	}
	got, ok = m.selectedLog()
	if !ok || got.ID != 1 {
		t.Fatalf("body filter selectedLog = %+v ok=%v, want only ID 1 visible", got, ok)
	}
}

func TestApplyLogFiltersCombined(t *testing.T) {
	m := newLogsModel()
	m.setRows(sampleLogRows())
	if got := visibleBodies(m); len(got) != 3 || got[0] != "request complete" || got[1] != "database timeout" || got[2] != "retry scheduled" {
		t.Fatalf("visible bodies before combined filter = %#v, want all rows in original order", got)
	}
	got, ok := m.selectedLog()
	if !ok || got.ID != 3 {
		t.Fatalf("selectedLog before combined filter = %+v ok=%v, want unfiltered ID 3", got, ok)
	}
	m.filters.Severity = "error"
	m.filters.Service = "api"
	m.filters.Body = "timeout"
	m.applyFilters()
	if got := visibleBodies(m); len(got) != 1 || got[0] != "database timeout" {
		t.Fatalf("visible bodies after combined filter = %#v, want only database timeout", got)
	}
	got, ok = m.selectedLog()
	if !ok || got.ID != 1 {
		t.Fatalf("combined filter selectedLog = %+v ok=%v, want only ID 1 visible", got, ok)
	}
}

func TestApplyLogFiltersSeverityAll(t *testing.T) {
	m := newLogsModel()
	m.setRows(sampleLogRows())
	if got := visibleServices(m); len(got) != 3 || got[0] != "api" || got[1] != "api" || got[2] != "worker" {
		t.Fatalf("visible services before all severity filter = %#v, want all rows in original order", got)
	}
	got, ok := m.selectedLog()
	if !ok || got.ID != 3 {
		t.Fatalf("selectedLog before all severity filter = %+v ok=%v, want unfiltered ID 3", got, ok)
	}
	m.filters.Severity = "all"
	m.filters.Service = "WORK"
	m.applyFilters()
	if got := visibleServices(m); len(got) != 1 || got[0] != "worker" {
		t.Fatalf("visible services after severity=all filter = %#v, want only worker", got)
	}
	got, ok = m.selectedLog()
	if !ok || got.ID != 2 {
		t.Fatalf("severity=all selectedLog = %+v ok=%v, want service filter to show ID 2", got, ok)
	}
}

func TestClearLogFiltersRestoresAllRows(t *testing.T) {
	m := newLogsModel()
	m.setRows(sampleLogRows())
	if got := visibleBodies(m); len(got) != 3 || got[0] != "request complete" || got[1] != "database timeout" || got[2] != "retry scheduled" {
		t.Fatalf("visible bodies before clear test = %#v, want all rows in original order", got)
	}
	m.filters.Severity = "warn"
	m.filters.Service = "work"
	m.filters.Body = "retry"
	m.applyFilters()
	if got := visibleBodies(m); len(got) != 1 || got[0] != "retry scheduled" {
		t.Fatalf("visible bodies before clear = %#v, want only retry scheduled", got)
	}
	m.clearFilters()
	if m.filters.Severity != "all" || m.filters.Service != "" || m.filters.Body != "" {
		t.Fatalf("filters after clear = %+v, want severity=all and empty text filters", m.filters)
	}
	if got := visibleBodies(m); len(got) != 3 || got[0] != "request complete" || got[1] != "database timeout" || got[2] != "retry scheduled" {
		t.Fatalf("visible bodies after clear = %#v, want all rows visible again", got)
	}
}

func TestLogsFilterModeShowsInlineFilterRow(t *testing.T) {
	m := newLogsModel()
	m.setRows(sampleLogRows())
	m.setSize(100, 20)

	if view := stripANSI(m.View()); strings.Contains(view, "Severity") {
		t.Fatalf("inactive filter mode should not render filter row, got view %q", view)
	}

	m, _ = m.Update(keyRunes("f"))

	if !m.filterMode {
		t.Fatalf("pressing f should enable filter mode")
	}
	if m.activeFilterField != logsFilterFieldSeverity {
		t.Fatalf("active filter field = %v, want severity", m.activeFilterField)
	}

	view := stripANSI(m.View())
	for _, want := range []string{"Severity", "Service", "Body", "all"} {
		if !strings.Contains(view, want) {
			t.Fatalf("filter view missing %q in %q", want, view)
		}
	}
}

func TestLogsFilterModeUpdatesFiltersAndPreservesValuesOnEsc(t *testing.T) {
	m := newLogsModel()
	m.setRows(sampleLogRows())

	m, _ = m.Update(keyRunes("f"))
	m, _ = m.Update(keyCode(tea.KeyRight))
	m, _ = m.Update(keyCode(tea.KeyTab))
	for _, ch := range "work" {
		m, _ = m.Update(keyRunes(string(ch)))
	}
	m, _ = m.Update(keyCode(tea.KeyTab))
	for _, ch := range "retry" {
		m, _ = m.Update(keyRunes(string(ch)))
	}
	m, _ = m.Update(keyCode(tea.KeyEsc))

	if m.filterMode {
		t.Fatalf("esc should exit filter mode")
	}
	if m.filters.Severity != "fatal" || m.filters.Service != "work" || m.filters.Body != "retry" {
		t.Fatalf("filters after editing = %+v, want severity=fatal service=work body=retry", m.filters)
	}
	if got := visibleBodies(m); len(got) != 0 {
		t.Fatalf("visible bodies after edited filters = %#v, want no visible rows", got)
	}
	view := stripANSI(m.View())
	for _, want := range []string{"Severity", "fatal", "Service", "work", "Body", "retry"} {
		if !strings.Contains(view, want) {
			t.Fatalf("after esc, active filters should stay visible in view; missing %q in %q", want, view)
		}
	}
}

func TestLogsFilterModeCTypesInTextFields(t *testing.T) {
	m := newLogsModel()
	m.setRows(sampleLogRows())
	m, _ = m.Update(keyRunes("f"))
	m, _ = m.Update(keyCode(tea.KeyTab)) // service
	if m.activeFilterField != logsFilterFieldService {
		t.Fatalf("active field = %v, want service", m.activeFilterField)
	}
	m, _ = m.Update(keyRunes("c"))
	if m.filters.Service != "c" {
		t.Fatalf("service after typing c = %q, want c (clear must not run)", m.filters.Service)
	}
	if m.filters.Severity != "all" {
		t.Fatalf("severity = %q, want all (uncleared)", m.filters.Severity)
	}

	m, _ = m.Update(keyCode(tea.KeyTab)) // body
	m, _ = m.Update(keyRunes("a"))
	m, _ = m.Update(keyRunes("c"))
	if m.filters.Body != "ac" {
		t.Fatalf("body after ac = %q, want ac", m.filters.Body)
	}
}

func TestLogsFilterModeClearShortcut(t *testing.T) {
	m := newLogsModel()
	m.setRows(sampleLogRows())
	m.filters.Severity = "warn"
	m.filters.Service = "work"
	m.filters.Body = "retry"
	m.applyFilters()

	m, _ = m.Update(keyRunes("f"))
	m, _ = m.Update(keyRunes("c"))

	if !m.filterMode {
		t.Fatalf("clear shortcut should keep filter mode active")
	}
	if m.filters.Severity != "all" || m.filters.Service != "" || m.filters.Body != "" {
		t.Fatalf("filters after clear shortcut = %+v, want severity=all and empty text filters", m.filters)
	}
	if got := visibleBodies(m); len(got) != 3 || got[0] != "request complete" || got[1] != "database timeout" || got[2] != "retry scheduled" {
		t.Fatalf("visible bodies after clear shortcut = %#v, want all rows restored", got)
	}
}

func TestSelectedLogUsesFilteredRows(t *testing.T) {
	m := newLogsModel()
	m.setRows(sampleLogRows())
	if got := visibleBodies(m); len(got) != 3 || got[0] != "request complete" || got[1] != "database timeout" || got[2] != "retry scheduled" {
		t.Fatalf("visible bodies before filtered selection test = %#v, want all rows in original order", got)
	}
	got, ok := m.selectedLog()
	if !ok || got.ID != 3 {
		t.Fatalf("selectedLog before filtered selection test = %+v ok=%v, want unfiltered ID 3", got, ok)
	}
	m.filters.Body = "RE"
	m.applyFilters()
	if got := visibleBodies(m); len(got) != 2 || got[0] != "request complete" || got[1] != "retry scheduled" {
		t.Fatalf("visible bodies after filtered selection test = %#v, want [request complete retry scheduled]", got)
	}
	m.table.SetCursor(1)
	got, ok = m.selectedLog()
	if !ok || got.ID != 2 {
		t.Fatalf("selectedLog with filtered cursor expected ID=2, got %+v ok=%v", got, ok)
	}
}
