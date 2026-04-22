package tui

import (
	"testing"

	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

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
