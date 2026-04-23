package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

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
