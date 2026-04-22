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

// LogRow is the UI-friendly row for the logs table and detail view.
type LogRow struct {
	ID            int64
	TimestampNs   int64
	Severity      *int64
	SeverityText  string
	Service       string
	TraceID       string
	SpanID        string
	Body          string
	Attributes    *string
	ResourceAttrs *string
}

// logsLoadedMsg is emitted after ListRecentLogRecords returns.
type logsLoadedMsg struct {
	Rows []LogRow
	Err  error
}

func toLogRow(r repo.LogRecord) LogRow {
	derefStr := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	return LogRow{
		ID:            r.ID,
		TimestampNs:   r.TimestampNs,
		Severity:      r.Severity,
		SeverityText:  derefStr(r.SeverityText),
		Service:       r.ServiceName,
		TraceID:       derefStr(r.TraceID),
		SpanID:        derefStr(r.SpanID),
		Body:          derefStr(r.Body),
		Attributes:    r.Attributes,
		ResourceAttrs: r.ResourceAttrs,
	}
}

// severityLabel maps OTel severity number (1-24) to label, falling back to
// severity_text when the number is nil or unrecognized.
func severityLabel(num *int64, text *string) string {
	if num != nil {
		switch {
		case *num >= 1 && *num <= 4:
			return "TRACE"
		case *num >= 5 && *num <= 8:
			return "DEBUG"
		case *num >= 9 && *num <= 12:
			return "INFO"
		case *num >= 13 && *num <= 16:
			return "WARN"
		case *num >= 17 && *num <= 20:
			return "ERROR"
		case *num >= 21 && *num <= 24:
			return "FATAL"
		}
	}
	if text != nil && *text != "" {
		return *text
	}
	return "-"
}

func loadLogsCmd(ctx context.Context, q *repo.Queries, limit int64) tea.Cmd {
	return func() tea.Msg {
		recs, err := q.ListRecentLogRecords(ctx, repo.ListRecentLogRecordsParams{
			Limit:  limit,
			Offset: 0,
		})
		if err != nil {
			return logsLoadedMsg{Err: err}
		}
		rows := make([]LogRow, 0, len(recs))
		for _, r := range recs {
			rows = append(rows, toLogRow(r))
		}
		return logsLoadedMsg{Rows: rows}
	}
}

// MetricNameRow is the UI-friendly row for the metric names list.
type MetricNameRow struct {
	Name string
}

// MetricSeriesPoint is a single (time, value) for charting.
type MetricSeriesPoint struct {
	TimestampNs int64
	Value       float64
}

// metricNamesLoadedMsg is emitted after ListMetricNames returns.
type metricNamesLoadedMsg struct {
	Rows []MetricNameRow
	Err  error
}

// metricSeriesLoadedMsg is emitted after ListMetricPointsByNameAndTimeRange returns.
type metricSeriesLoadedMsg struct {
	Name   string
	Points []MetricSeriesPoint
	Unit   string
	Type   int64
	Err    error
}

// pointValue extracts a plottable float from a metric point.
//
// Gauge (1) / Sum (2): ValueDouble if non-nil, else float64(ValueInt), else 0.
// Histogram (3): HistSum / HistCount (simple average) when count > 0.
// Other types: 0.
func pointValue(p repo.MetricPoint) float64 {
	switch p.Type {
	case 1, 2:
		if p.ValueDouble != nil {
			return *p.ValueDouble
		}
		if p.ValueInt != nil {
			return float64(*p.ValueInt)
		}
		return 0
	case 3:
		if p.HistCount != nil && *p.HistCount > 0 && p.HistSum != nil {
			return *p.HistSum / float64(*p.HistCount)
		}
		return 0
	}
	return 0
}

func loadMetricNamesCmd(ctx context.Context, q *repo.Queries) tea.Cmd {
	return func() tea.Msg {
		names, err := q.ListMetricNames(ctx)
		if err != nil {
			return metricNamesLoadedMsg{Err: err}
		}
		rows := make([]MetricNameRow, 0, len(names))
		for _, n := range names {
			rows = append(rows, MetricNameRow{Name: n})
		}
		return metricNamesLoadedMsg{Rows: rows}
	}
}

func loadMetricSeriesCmd(ctx context.Context, q *repo.Queries, name string, windowSec int64) tea.Cmd {
	return func() tea.Msg {
		now := time.Now().UnixNano()
		start := now - windowSec*int64(time.Second)
		recs, err := q.ListMetricPointsByNameAndTimeRange(ctx, repo.ListMetricPointsByNameAndTimeRangeParams{
			Name:          name,
			TimestampNs:   start,
			TimestampNs_2: now,
		})
		if err != nil {
			return metricSeriesLoadedMsg{Name: name, Err: err}
		}

		// Fallback: when the time window yields nothing (stale data),
		// load the most recent points regardless of time.
		if len(recs) == 0 {
			recs, err = q.ListMetricPointsByName(ctx, repo.ListMetricPointsByNameParams{
				Name:  name,
				Limit: 1000,
			})
			if err != nil {
				return metricSeriesLoadedMsg{Name: name, Err: err}
			}
			// Query returns DESC order; reverse to ASC for charting.
			for i, j := 0, len(recs)-1; i < j; i, j = i+1, j-1 {
				recs[i], recs[j] = recs[j], recs[i]
			}
		}

		pts := make([]MetricSeriesPoint, 0, len(recs))
		var unit string
		var mtype int64
		for i, r := range recs {
			if i == 0 {
				mtype = r.Type
				if r.Unit != nil {
					unit = *r.Unit
				}
			} else if unit == "" && r.Unit != nil {
				unit = *r.Unit
			}
			pts = append(pts, MetricSeriesPoint{TimestampNs: r.TimestampNs, Value: pointValue(r)})
		}
		return metricSeriesLoadedMsg{Name: name, Points: pts, Unit: unit, Type: mtype}
	}
}
