package tui

import (
	"strings"
	"testing"
)

func TestRenderLogDetailBasic(t *testing.T) {
	row := LogRow{
		ID:            1,
		TimestampNs:   1_700_000_000_000_000_000,
		Severity:      i64Ptr(17),
		SeverityText:  "ERROR",
		Service:       "api",
		TraceID:       "abc123",
		SpanID:        "span1",
		Body:          "disk full",
		Attributes:    strPtr(`{"k":"v"}`),
		ResourceAttrs: strPtr(`{"host":"h1"}`),
	}
	out := renderLogDetail(row)
	wants := []string{"api", "abc123", "span1", "disk full", "ERROR", "Attributes", "Resource", "k", "v", "host"}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("output missing %q\n%s", w, out)
		}
	}
}

func TestRenderLogDetailNilSafe(t *testing.T) {
	row := LogRow{ID: 2, Service: "svc", TimestampNs: 42}
	out := renderLogDetail(row)
	if !strings.Contains(out, "svc") {
		t.Errorf("service missing:\n%s", out)
	}
	if !strings.Contains(out, "-") {
		t.Errorf("expected '-' placeholders for missing fields:\n%s", out)
	}
}
