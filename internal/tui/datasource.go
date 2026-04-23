package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

type TraceRow struct {
	TraceID     string
	StartTimeNs int64
	DurationNs  int64
	SpanCount   int64
	HasErrors   bool
	Service     string
	RootName    string
}

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

type tickMsg time.Time

type tracesLoadedMsg struct {
	Rows []TraceRow
	Err  error
}

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

type KPIData struct {
	SQLExecLatency  string
	AvgGETDuration  string
	AvgPOSTDuration string
	IdleConns       string
}

type MetricGroupStats struct {
	Label  string
	Latest float64
	Min    float64
	Max    float64
	Avg    float64
	Count  int
}

type groupResult struct {
	Groups    []MetricGroupStats
	AggLatest float64
	AggMin    float64
	AggMax    float64
	AggAvg    float64
	Unit      string
	Type      int64
}

type MetricNameRow struct {
	Name string
}

type metricNamesLoadedMsg struct {
	Rows []MetricNameRow
	Err  error
}

type kpisLoadedMsg struct {
	Data KPIData
	Err  error
}

type metricGroupsLoadedMsg struct {
	Name      string
	Groups    []MetricGroupStats
	AggLatest float64
	AggMin    float64
	AggMax    float64
	AggAvg    float64
	Unit      string
	Type      int64
	Err       error
}

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

const kpiWindowSec int64 = 900

func computeKPIs(dbPts, httpPts, connPts []repo.MetricPoint) KPIData {
	d := KPIData{
		SQLExecLatency:  "n/a",
		AvgGETDuration:  "n/a",
		AvgPOSTDuration: "n/a",
		IdleConns:       "n/a",
	}
	for _, p := range dbPts {
		if attrsMap(p.Attributes)["method"] == "sql.conn.exec" {
			if v := pointValue(p); v > 0 {
				d.SQLExecLatency = fmt.Sprintf("%.2fms", v)
			}
		}
	}
	for _, p := range httpPts {
		v := pointValue(p) * 1000
		switch attrsMap(p.Attributes)["http.request.method"] {
		case "GET":
			d.AvgGETDuration = fmt.Sprintf("%.2fms", v)
		case "POST":
			d.AvgPOSTDuration = fmt.Sprintf("%.2fms", v)
		}
	}
	for _, p := range connPts {
		if attrsMap(p.Attributes)["status"] == "idle" && p.ValueInt != nil {
			d.IdleConns = fmt.Sprintf("%d", *p.ValueInt)
		}
	}
	return d
}

func loadMetricPointsWindowOrLatest(ctx context.Context, q *repo.Queries, name string, windowSec int64) ([]repo.MetricPoint, error) {
	now := time.Now().UnixNano()
	start := now - windowSec*int64(time.Second)
	recs, err := q.ListMetricPointsByNameAndTimeRange(ctx, repo.ListMetricPointsByNameAndTimeRangeParams{
		Name:          name,
		TimestampNs:   start,
		TimestampNs_2: now,
	})
	if err != nil {
		return nil, err
	}
	if len(recs) > 0 {
		return recs, nil
	}

	recs, err = q.ListMetricPointsByName(ctx, repo.ListMetricPointsByNameParams{
		Name:  name,
		Limit: 1000,
	})
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(recs)-1; i < j; i, j = i+1, j-1 {
		recs[i], recs[j] = recs[j], recs[i]
	}
	return recs, nil
}

func loadKPIsCmd(ctx context.Context, q *repo.Queries) tea.Cmd {
	return func() tea.Msg {
		fetch := func(name string) ([]repo.MetricPoint, error) {
			return loadMetricPointsWindowOrLatest(ctx, q, name, kpiWindowSec)
		}

		dbPts, err := fetch("db.sql.latency")
		if err != nil {
			return kpisLoadedMsg{Err: err}
		}
		httpPts, err := fetch("http.server.request.duration")
		if err != nil {
			return kpisLoadedMsg{Err: err}
		}
		connPts, err := fetch("db.sql.connection.open")
		if err != nil {
			return kpisLoadedMsg{Err: err}
		}

		return kpisLoadedMsg{Data: computeKPIs(dbPts, httpPts, connPts)}
	}
}

func loadMetricGroupsCmd(ctx context.Context, q *repo.Queries, name string, windowSec int64) tea.Cmd {
	return func() tea.Msg {
		now := time.Now().UnixNano()
		start := now - windowSec*int64(time.Second)
		recs, err := q.ListMetricPointsByNameAndTimeRange(ctx, repo.ListMetricPointsByNameAndTimeRangeParams{
			Name:          name,
			TimestampNs:   start,
			TimestampNs_2: now,
		})
		if err != nil {
			return metricGroupsLoadedMsg{Name: name, Err: err}
		}
		if len(recs) == 0 {
			recs, err = q.ListMetricPointsByName(ctx, repo.ListMetricPointsByNameParams{
				Name:  name,
				Limit: 1000,
			})
			if err != nil {
				return metricGroupsLoadedMsg{Name: name, Err: err}
			}
			for i, j := 0, len(recs)-1; i < j; i, j = i+1, j-1 {
				recs[i], recs[j] = recs[j], recs[i]
			}
		}
		r := groupMetricPoints(recs)
		return metricGroupsLoadedMsg{
			Name:      name,
			Groups:    r.Groups,
			AggLatest: r.AggLatest,
			AggMin:    r.AggMin,
			AggMax:    r.AggMax,
			AggAvg:    r.AggAvg,
			Unit:      r.Unit,
			Type:      r.Type,
		}
	}
}

func attrsMap(raw *string) map[string]string {
	if raw == nil || *raw == "" {
		return nil
	}
	var m map[string]string
	_ = json.Unmarshal([]byte(*raw), &m)
	return m
}

func attrLabel(raw *string) string {
	m := attrsMap(raw)
	if len(m) == 0 {
		return ""
	}
	if httpMethod, ok := m["http.request.method"]; ok {
		if status, ok := m["http.response.status_code"]; ok {
			return httpMethod + " " + status
		}
		return httpMethod
	}
	if v, ok := m["method"]; ok {
		return v
	}
	if v, ok := m["status"]; ok {
		return v
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+m[k])
	}
	return strings.Join(parts, " ")
}

func statsFromValues(vals []float64) (latest, min, max, avg float64) {
	if len(vals) == 0 {
		return 0, 0, 0, 0
	}
	latest = vals[len(vals)-1]
	min, max = vals[0], vals[0]
	var sum float64
	for _, v := range vals {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}
	avg = sum / float64(len(vals))
	return
}

func groupMetricPoints(pts []repo.MetricPoint) groupResult {
	if len(pts) == 0 {
		return groupResult{}
	}
	var res groupResult
	res.Type = pts[0].Type
	if pts[0].Unit != nil {
		res.Unit = *pts[0].Unit
	}

	order := []string{}
	seen := map[string]bool{}
	buckets := map[string][]float64{}

	for _, p := range pts {
		label := attrLabel(p.Attributes)
		val := pointValue(p)
		if !seen[label] {
			seen[label] = true
			order = append(order, label)
		}
		buckets[label] = append(buckets[label], val)
	}

	var allVals []float64
	for _, label := range order {
		vals := buckets[label]
		latest, min, max, avg := statsFromValues(vals)
		res.Groups = append(res.Groups, MetricGroupStats{
			Label: label, Latest: latest, Min: min, Max: max, Avg: avg, Count: len(vals),
		})
		allVals = append(allVals, vals...)
	}
	_, res.AggMin, res.AggMax, res.AggAvg = statsFromValues(allVals)
	res.AggLatest = pointValue(pts[len(pts)-1])
	return res
}
