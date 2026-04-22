package tui

import (
	"testing"

	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

func TestSummarizeQueryRows(t *testing.T) {
	rows := []repo.ListTraceSummariesRow{
		{
			TraceID:     "abc123",
			StartTimeNs: 1_000_000_000,
			DurationNs:  5_000_000,
			SpanCount:   3,
			HasErrors:   1,
			RootService: "svc-a",
			RootName:    "GET /foo",
		},
		{
			TraceID:     "def456",
			StartTimeNs: 2_000_000_000,
			DurationNs:  0,
			SpanCount:   1,
			HasErrors:   0,
			RootService: "",
			RootName:    "",
		},
	}
	got := summarizeQueryRows(rows)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	a := got[0]
	if a.TraceID != "abc123" || a.Service != "svc-a" || a.RootName != "GET /foo" ||
		a.SpanCount != 3 || !a.HasErrors || a.DurationNs != 5_000_000 {
		t.Errorf("row 0 unexpected: %+v", a)
	}
	b := got[1]
	if b.Service != "-" || b.RootName != "-" || b.HasErrors {
		t.Errorf("row 1 unexpected: %+v", b)
	}
}
