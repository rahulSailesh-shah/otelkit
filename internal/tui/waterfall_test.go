package tui

import (
	"testing"

	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

func TestLayoutBarsBasic(t *testing.T) {
	spans := []repo.Span{
		{SpanID: "root", StartTimeNs: 0, EndTimeNs: 1000, StatusCode: 0},
		{SpanID: "child1", StartTimeNs: 100, EndTimeNs: 400, StatusCode: 0},
		{SpanID: "child2", StartTimeNs: 500, EndTimeNs: 900, StatusCode: 2},
	}
	bars := layoutBars(spans, 100)
	if len(bars) != 3 {
		t.Fatalf("len = %d, want 3", len(bars))
	}
	if bars[0].Offset != 0 || bars[0].Width != 100 {
		t.Errorf("root bar = %+v, want offset=0 width=100", bars[0])
	}
	if bars[1].Offset != 10 || bars[1].Width != 30 {
		t.Errorf("child1 bar = %+v, want offset=10 width=30", bars[1])
	}
	if bars[2].Style != BarError {
		t.Errorf("child2 style = %v, want BarError", bars[2].Style)
	}
}

func TestLayoutBarsMinWidth(t *testing.T) {
	spans := []repo.Span{
		{SpanID: "r", StartTimeNs: 0, EndTimeNs: 10_000},
		{SpanID: "s", StartTimeNs: 9, EndTimeNs: 10},
	}
	bars := layoutBars(spans, 100)
	if bars[1].Width < 1 {
		t.Errorf("narrow span width = %d, want >= 1", bars[1].Width)
	}
}

func TestLayoutBarsEmpty(t *testing.T) {
	if got := layoutBars(nil, 80); got != nil {
		t.Errorf("nil spans should return nil, got %v", got)
	}
}
