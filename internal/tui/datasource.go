package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

// TraceRow is the UI-friendly row for the traces table.
type TraceRow struct {
	TraceID     string
	StartTimeNs int64
	DurationNs  int64
	SpanCount   int64
	HasErrors   bool
	Service     string
	RootName    string
}

// summarizeQueryRows converts sqlc rows to UI rows with sane fallbacks.
func summarizeQueryRows(rows []repo.ListTraceSummariesRow) []TraceRow {
	out := make([]TraceRow, 0, len(rows))
	for _, r := range rows {
		service := r.RootService
		if service == "" {
			service = "-"
		}
		name := r.RootName
		if name == "" {
			name = "-"
		}
		out = append(out, TraceRow{
			TraceID:     r.TraceID,
			StartTimeNs: r.StartTimeNs,
			DurationNs:  r.DurationNs,
			SpanCount:   r.SpanCount,
			HasErrors:   r.HasErrors != 0,
			Service:     service,
			RootName:    name,
		})
	}
	return out
}

// tickMsg is sent on a timer to drive periodic refreshes.
type tickMsg time.Time

// tracesLoadedMsg is emitted after ListTraceSummaries returns.
type tracesLoadedMsg struct {
	Rows []TraceRow
	Err  error
}

// traceSpansLoadedMsg is emitted after ListSpansByTrace returns.
type traceSpansLoadedMsg struct {
	TraceID string
	Spans   []repo.Span
	Err     error
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func loadTracesCmd(ctx context.Context, q *repo.Queries, limit int64) tea.Cmd {
	return func() tea.Msg {
		rows, err := q.ListTraceSummaries(ctx, repo.ListTraceSummariesParams{
			Limit:  limit,
			Offset: 0,
		})
		if err != nil {
			return tracesLoadedMsg{Err: err}
		}
		return tracesLoadedMsg{Rows: summarizeQueryRows(rows)}
	}
}

func loadTraceSpansCmd(ctx context.Context, q *repo.Queries, traceID string) tea.Cmd {
	return func() tea.Msg {
		spans, err := q.ListSpansByTrace(ctx, traceID)
		if err != nil {
			return traceSpansLoadedMsg{TraceID: traceID, Err: err}
		}
		return traceSpansLoadedMsg{TraceID: traceID, Spans: spans}
	}
}
