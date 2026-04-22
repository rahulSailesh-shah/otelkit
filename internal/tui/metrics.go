package tui

import (
	"context"
	"fmt"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

// metricWindowSec is the fixed look-back window for the right-pane chart.
const metricWindowSec int64 = 900

type metricsModel struct {
	list       table.Model
	names      []MetricNameRow
	selected   string
	series     []MetricSeriesPoint
	unit       string
	metricType int64
	chart      metricChart
	width      int
	height     int
	lastSync   time.Time

	ctx     context.Context
	queries *repo.Queries
}

func newMetricsModel() metricsModel {
	cols := []table.Column{
		{Title: "Metric", Width: 26},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	t.SetStyles(table.Styles{
		Header:   lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(colorText).Background(colorBrand),
		Cell:     lipgloss.NewStyle().Padding(0, 1).Foreground(colorText),
		Selected: lipgloss.NewStyle().Padding(0, 1).Foreground(colorText).Background(lipgloss.Color("#313244")).Bold(true),
	})
	return metricsModel{list: t, chart: newMetricChart()}
}

// setSource wires the data source used for building follow-up series-load
// commands on selection changes. Safe to omit in tests (commands become nil).
func (m *metricsModel) setSource(ctx context.Context, q *repo.Queries) {
	m.ctx = ctx
	m.queries = q
}

func (m *metricsModel) setNames(rows []MetricNameRow) {
	m.names = rows
	tr := make([]table.Row, 0, len(rows))
	for _, r := range rows {
		tr = append(tr, table.Row{truncate(r.Name, 64)})
	}
	m.list.SetRows(tr)
	m.lastSync = time.Now()

	if len(rows) == 0 {
		m.selected = ""
		m.series = nil
		m.unit = ""
		m.metricType = 0
		m.chart.setPoints(nil)
		return
	}

	// If the previously-selected name is gone, fall back to the current cursor row.
	stillPresent := false
	for _, r := range rows {
		if r.Name == m.selected {
			stillPresent = true
			break
		}
	}
	if !stillPresent {
		if n, ok := m.currentRowName(); ok {
			m.selected = n
			m.series = nil
			m.chart.setPoints(nil)
		}
	}
}

func (m *metricsModel) setSeries(name string, pts []MetricSeriesPoint, unit string, mtype int64) {
	m.selected = name
	m.series = pts
	m.unit = unit
	m.metricType = mtype
	m.chart.setPoints(pts)
}

func (m metricsModel) currentRowName() (string, bool) {
	idx := m.list.Cursor()
	if idx < 0 || idx >= len(m.names) {
		return "", false
	}
	return m.names[idx].Name, true
}

// selectedName returns the metric name at the list cursor.
func (m metricsModel) selectedName() (string, bool) {
	return m.currentRowName()
}

func (m metricsModel) Init() tea.Cmd { return nil }

func (m metricsModel) Update(msg tea.Msg) (metricsModel, tea.Cmd) {
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	newName, ok := m.currentRowName()
	if !ok {
		return m, cmd
	}
	if newName == m.selected {
		return m, cmd
	}

	m.selected = newName
	m.series = nil
	m.chart.setPoints(nil)
	loadCmd := m.loadSelectedSeriesCmd()
	switch {
	case cmd == nil:
		return m, loadCmd
	case loadCmd == nil:
		return m, cmd
	default:
		return m, tea.Batch(cmd, loadCmd)
	}
}

func (m metricsModel) loadSelectedSeriesCmd() tea.Cmd {
	if m.queries == nil || m.ctx == nil || m.selected == "" {
		return nil
	}
	return loadMetricSeriesCmd(m.ctx, m.queries, m.selected, metricWindowSec)
}

func (m *metricsModel) setSize(w, h int) {
	m.width, m.height = w, h

	listW := 32
	if w < 80 {
		listW = max(20, w/3)
	}
	if listW > w-20 {
		listW = max(10, w-20)
	}

	contentH := max(5, h-3)

	m.list.SetWidth(listW)
	m.list.SetHeight(max(5, contentH-3))

	cols := m.list.Columns()
	if len(cols) > 0 {
		cols[0].Width = max(6, listW-4)
		m.list.SetColumns(cols)
	}

	chartW := max(20, w-listW-6)
	chartH := max(5, contentH-5)
	m.chart.setSize(chartW, chartH)
}

func (m metricsModel) View() string {
	header := titleStyle.Render("Metrics") +
		helpStyle.Render(fmt.Sprintf("  last sync %s · r refresh · ↑/↓ select · tab switch", nowFmt()))

	leftBody := borderStyle.Render(m.list.View())

	var right string
	if m.selected == "" {
		msg := helpStyle.Italic(true).Render("select a metric")
		w := max(20, m.width-38)
		h := max(5, m.height-6)
		box := lipgloss.NewStyle().Width(w).Height(h).Align(lipgloss.Center, lipgloss.Center).Render(msg)
		right = borderStyle.Render(box)
	} else {
		right = borderStyle.Render(
			m.chart.chartView(m.selected, m.unit, m.metricType, m.series, metricWindowSec),
		)
	}

	panes := lipgloss.JoinHorizontal(lipgloss.Top, leftBody, right)
	return lipgloss.JoinVertical(lipgloss.Left, header, panes)
}
