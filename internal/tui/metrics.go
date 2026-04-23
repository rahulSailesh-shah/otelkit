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

const metricWindowSec int64 = 900

type metricsModel struct {
	list     table.Model
	names    []MetricNameRow
	selected string

	kpis       KPIData
	groups     []MetricGroupStats
	aggLatest  float64
	aggMin     float64
	aggMax     float64
	aggAvg     float64
	unit       string
	metricType int64
	groupTable table.Model

	width    int
	height   int
	lastSync time.Time

	ctx     context.Context
	queries *repo.Queries
}

func newMetricsModel() metricsModel {
	listCols := []table.Column{{Title: "Metric", Width: 26}}
	t := table.New(
		table.WithColumns(listCols),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	t.SetStyles(table.Styles{
		Header:   lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(colorText).Background(colorBrand),
		Cell:     lipgloss.NewStyle().Padding(0, 1).Foreground(colorText),
		Selected: lipgloss.NewStyle().Padding(0, 1).Foreground(colorText).Background(colorSelect).Bold(true),
	})

	groupCols := []table.Column{
		{Title: "Attributes", Width: 30},
		{Title: "Latest", Width: 8},
		{Title: "Min", Width: 8},
		{Title: "Max", Width: 8},
		{Title: "Avg", Width: 8},
	}
	gt := table.New(
		table.WithColumns(groupCols),
		table.WithHeight(10),
	)
	gt.SetStyles(table.Styles{
		Header: lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(colorText).Background(colorBrand),
		Cell:   lipgloss.NewStyle().Padding(0, 1).Foreground(colorMuted),
	})

	return metricsModel{list: t, groupTable: gt}
}

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
		m.groups = nil
		m.groupTable.SetRows(nil)
		m.unit = ""
		m.metricType = 0
		return
	}

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
			m.groups = nil
			m.groupTable.SetRows(nil)
		}
	}
}

func (m *metricsModel) setKPIs(d KPIData) {
	m.kpis = d
}

func (m *metricsModel) setGroups(msg metricGroupsLoadedMsg) {
	m.selected = msg.Name
	m.groups = msg.Groups
	m.aggLatest = msg.AggLatest
	m.aggMin = msg.AggMin
	m.aggMax = msg.AggMax
	m.aggAvg = msg.AggAvg
	m.unit = msg.Unit
	m.metricType = msg.Type

	rows := make([]table.Row, 0, len(msg.Groups))
	for _, g := range msg.Groups {
		rows = append(rows, table.Row{
			g.Label,
			fmtFloat(g.Latest),
			fmtFloat(g.Min),
			fmtFloat(g.Max),
			fmtFloat(g.Avg),
		})
	}
	m.groupTable.SetRows(rows)
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

func (m *metricsModel) Update(ctx context.Context, q *repo.Queries, msg tea.Msg) (Tab, tea.Cmd) {
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
	m.groups = nil
	m.groupTable.SetRows(nil)
	loadCmd := m.loadSelectedGroupsCmd()
	switch {
	case cmd == nil:
		return m, loadCmd
	case loadCmd == nil:
		return m, cmd
	default:
		return m, tea.Batch(cmd, loadCmd)
	}
}

func (m metricsModel) loadSelectedGroupsCmd() tea.Cmd {
	if m.queries == nil || m.ctx == nil || m.selected == "" {
		return nil
	}
	return loadMetricGroupsCmd(m.ctx, m.queries, m.selected, metricWindowSec)
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

	rightW := max(20, w-listW-6)
	tableH := max(3, contentH-10)

	tblCols := m.groupTable.Columns()
	if len(tblCols) > 0 {
		attrW := max(10, rightW-8*4-10)
		tblCols[0].Width = attrW
		m.groupTable.SetColumns(tblCols)
	}
	m.groupTable.SetHeight(tableH)
	m.groupTable.SetWidth(rightW)
}

func (m metricsModel) View() string {
	header := titleStyle.Render("Metrics") +
		helpStyle.Render(fmt.Sprintf("  last sync %s · r refresh · ↑/↓ select · tab switch", nowFmt()))

	leftBody := borderStyle.Render(m.list.View())
	rightBody := borderStyle.Render(m.renderRight())
	panes := lipgloss.JoinHorizontal(lipgloss.Top, leftBody, rightBody)
	return lipgloss.JoinVertical(lipgloss.Left, header, panes)
}

func (m metricsModel) renderRight() string {
	kpiRow := lipgloss.JoinHorizontal(lipgloss.Top,
		m.kpiCard("SQL Exec Avg", m.kpis.SQLExecLatency),
		m.kpiCard("Avg GET", m.kpis.AvgGETDuration),
		m.kpiCard("Avg POST", m.kpis.AvgPOSTDuration),
		m.kpiCard("Idle Conns", m.kpis.IdleConns),
	)

	var detail string
	if m.selected == "" {
		detail = helpStyle.Italic(true).Render("select a metric")
	} else {
		qualifier := typeLabel(m.metricType)
		if m.unit != "" {
			qualifier = qualifier + " · " + m.unit
		}
		metricHeader := titleStyle.Render(m.selected) + "  " + helpStyle.Render("("+qualifier+")")
		aggStats := fmt.Sprintf("latest: %s  min: %s  max: %s  avg: %s",
			fmtFloat(m.aggLatest), fmtFloat(m.aggMin), fmtFloat(m.aggMax), fmtFloat(m.aggAvg))
		detail = lipgloss.JoinVertical(lipgloss.Left,
			metricHeader,
			attrVal.Render(aggStats),
			"",
			m.groupTable.View(),
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left, kpiRow, "", detail)
}

func (m metricsModel) kpiCard(label, val string) string {
	return kpiCardStyle.Render(
		helpStyle.Render(label) + "\n" + attrVal.Bold(true).Render(val),
	)
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

func (m metricsModel) Label() string    { return "Metrics" }
func (m *metricsModel) HelpKeys() []TabKey {
	return []TabKey{
		{Keys: "↑/↓", Help: "select metric"},
	}
}
func (m *metricsModel) SetSize(w, h int) { m.setSize(w, h) }

func (m metricsModel) RefreshCmd(ctx context.Context, q *repo.Queries) tea.Cmd {
	cmds := []tea.Cmd{
		loadMetricNamesCmd(ctx, q),
		loadKPIsCmd(ctx, q),
	}
	if name, ok := m.selectedName(); ok {
		cmds = append(cmds, loadMetricGroupsCmd(ctx, q, name, metricWindowSec))
	}
	return tea.Batch(cmds...)
}

func (m *metricsModel) OnLeave() {}

func (m metricsModel) ConsumesTab() bool { return false }

func (m metricsModel) ConsumesEnter() bool { return false }
