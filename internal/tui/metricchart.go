package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/linechart/timeserieslinechart"
)

type metricChart struct {
	ts     timeserieslinechart.Model
	width  int
	height int
}

func newMetricChart() metricChart {
	ts := timeserieslinechart.New(60, 14)
	return metricChart{ts: ts, width: 60, height: 14}
}

func (c *metricChart) setSize(w, h int) {
	c.width = w
	c.height = h
	c.ts.Resize(w, h)
}

func (c *metricChart) setPoints(pts []MetricSeriesPoint) {
	c.ts.ClearAllData()
	for _, p := range pts {
		c.ts.Push(timeserieslinechart.TimePoint{
			Time:  time.Unix(0, p.TimestampNs),
			Value: p.Value,
		})
	}
	c.ts.DrawBraille()
}

func (c metricChart) view() string {
	return c.ts.View()
}

// metricStats returns latest, min, max, avg, and count for a time-sorted series.
// Empty input returns all zeros.
func metricStats(pts []MetricSeriesPoint) (latest, min, max, avg float64, n int) {
	n = len(pts)
	if n == 0 {
		return 0, 0, 0, 0, 0
	}
	latest = pts[n-1].Value
	min = pts[0].Value
	max = pts[0].Value
	var sum float64
	for _, p := range pts {
		if p.Value < min {
			min = p.Value
		}
		if p.Value > max {
			max = p.Value
		}
		sum += p.Value
	}
	avg = sum / float64(n)
	return latest, min, max, avg, n
}

func typeLabel(t int64) string {
	switch t {
	case 1:
		return "gauge"
	case 2:
		return "sum"
	case 3:
		return "histogram"
	default:
		return "-"
	}
}

func chartHeader(name, unit string, mtype int64) string {
	qualifier := typeLabel(mtype)
	if unit != "" {
		qualifier = qualifier + ", " + unit
	}
	return titleStyle.Render(name + "  " + helpStyle.Render("("+qualifier+")"))
}

func chartFooter(pts []MetricSeriesPoint, windowSec int64) string {
	latest, mn, mx, avg, n := metricStats(pts)
	if n == 0 {
		return helpStyle.Render("no points in window")
	}
	window := "last " + fmtWindow(windowSec)
	if n > 0 {
		spanNs := pts[n-1].TimestampNs - pts[0].TimestampNs
		spanSec := spanNs / int64(time.Second)
		if spanSec > windowSec {
			window = "all (" + fmtWindow(spanSec) + ")"
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "latest: %s  min: %s  max: %s  avg: %s   points: %d  window: %s",
		fmtFloat(latest), fmtFloat(mn), fmtFloat(mx), fmtFloat(avg), n, window)
	return attrVal.Render(b.String())
}

func fmtFloat(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%.3g", v)
}

func fmtWindow(sec int64) string {
	if sec%60 == 0 {
		return fmt.Sprintf("%dm", sec/60)
	}
	return fmt.Sprintf("%ds", sec)
}

func (c metricChart) chartView(name, unit string, mtype int64, pts []MetricSeriesPoint, windowSec int64) string {
	return lipgloss.JoinVertical(lipgloss.Left,
		chartHeader(name, unit, mtype),
		c.view(),
		chartFooter(pts, windowSec),
	)
}
