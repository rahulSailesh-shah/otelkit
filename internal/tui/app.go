package tui

import (
	"context"
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

const (
	tracesPageSize int64 = 100
	logsPageSize   int64 = 200
)

const chromeHeight = 3

type tickMsg time.Time

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (t tabID) label() string {
	switch t {
	case tabTraces:
		return "Traces"
	case tabMetrics:
		return "Metrics"
	case tabLogs:
		return "Logs"
	case tabProcOut:
		return "Process Output"
	}
	return ""
}

type appModel struct {
	ctx     context.Context
	opts    Options
	queries *repo.Queries

	width  int
	height int

	tabs       []Tab
	ids        []tabID // parallel to tabs — identity for each index
	active     int
	showHelp   bool
	procOut    *procOutModel
	hasProcOut bool
	lastErr    string
}

func newAppModel(ctx context.Context, opts Options) appModel {
	traces := newTracesModel()
	logs := newLogsModel()
	metrics := newMetricsModel()
	metrics.setSource(ctx, opts.Queries)

	m := appModel{
		ctx:     ctx,
		opts:    opts,
		queries: opts.Queries,
		tabs:    []Tab{&traces, &metrics, &logs},
		ids:     []tabID{tabTraces, tabMetrics, tabLogs},
	}
	if opts.ChildLogPath != "" {
		po := newProcOutModel(opts.ChildLogPath)
		m.procOut = &po
		m.hasProcOut = true
		m.tabs = append(m.tabs, m.procOut)
		m.ids = append(m.ids, tabProcOut)
	}
	return m
}

func (m appModel) Init() tea.Cmd {
	return tea.Batch(m.refreshAllCmd(), tickCmd(m.opts.RefreshInterval))
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeTabs()
		return m, nil

	case tea.KeyPressMsg:
		active := m.tabs[m.active]
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Tab):
			if active.ConsumesTab() {
				break
			}
			m.switchTab(+1)
			return m, m.procOutKickCmd()
		case key.Matches(msg, keys.ShiftTab):
			if active.ConsumesTab() {
				break
			}
			m.switchTab(-1)
			return m, m.procOutKickCmd()
		case key.Matches(msg, keys.Refresh):
			return m, m.refreshAllCmd()
		case key.Matches(msg, keys.Help):
			m.showHelp = !m.showHelp
			return m, nil
		case key.Matches(msg, keys.Enter):
			if active.ConsumesEnter() {
				// fall through to dispatch
			}
		}

	case tickMsg:
		return m, tea.Batch(m.refreshAllCmd(), tickCmd(m.opts.RefreshInterval))

	case procOutTickMsg:
		if m.hasProcOut && m.ids[m.active] == tabProcOut {
			*m.procOut = m.procOut.tick()
			return m, procOutTickCmd()
		}
		return m, nil

	case tracesLoadedMsg:
		if msg.Err != nil {
			m.lastErr = "traces load: " + msg.Err.Error()
			return m, nil
		}
		m.lastErr = ""
		// Traces tab owns its data; look it up by id.
		if t, ok := m.findTab(tabTraces).(*tracesModel); ok {
			t.setRows(msg.Rows)
		}
		return m, nil

	case traceSpansLoadedMsg:
		if msg.Err != nil {
			m.lastErr = "spans load: " + msg.Err.Error()
			return m, nil
		}
		if t, ok := m.findTab(tabTraces).(*tracesModel); ok {
			t.SetSpans(msg)
			m.lastErr = ""
		}
		return m, nil

	case logsLoadedMsg:
		if msg.Err != nil {
			m.lastErr = "logs load: " + msg.Err.Error()
			return m, nil
		}
		m.lastErr = ""
		if l, ok := m.findTab(tabLogs).(*logsModel); ok {
			l.setRows(msg.Rows)
		}
		return m, nil

	case metricNamesLoadedMsg:
		if msg.Err != nil {
			m.lastErr = "metric names load: " + msg.Err.Error()
			return m, nil
		}
		m.lastErr = ""
		mm, _ := m.findTab(tabMetrics).(*metricsModel)
		if mm == nil {
			return m, nil
		}
		prev, hadPrev := mm.selectedName()
		mm.setNames(msg.Rows)
		cur, hasCur := mm.selectedName()
		if hasCur && (!hadPrev || prev != cur) {
			return m, loadMetricGroupsCmd(m.ctx, m.queries, cur, metricWindowSec)
		}
		return m, nil

	case kpisLoadedMsg:
		if msg.Err != nil {
			m.lastErr = "kpis load: " + msg.Err.Error()
			return m, nil
		}
		if mm, ok := m.findTab(tabMetrics).(*metricsModel); ok {
			mm.setKPIs(msg.Data)
			m.lastErr = ""
		}
		return m, nil

	case metricGroupsLoadedMsg:
		if msg.Err != nil {
			m.lastErr = "metric groups load: " + msg.Err.Error()
			return m, nil
		}
		if mm, ok := m.findTab(tabMetrics).(*metricsModel); ok {
			if name, ok := mm.selectedName(); ok && name == msg.Name {
				mm.setGroups(msg)
				m.lastErr = ""
			}
		}
		return m, nil
	}

	_, cmd := m.tabs[m.active].Update(m.ctx, m.queries, msg)
	return m, cmd
}

func (m appModel) refreshAllCmd() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.tabs))
	for _, t := range m.tabs {
		if cmd := t.RefreshCmd(m.ctx, m.queries); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (m appModel) View() tea.View {
	labels := []string{}
	for _, t := range m.tabs {
		labels = append(labels, t.Label())
	}
	bar := joinTabs(labels, m.active) + "   " + helpStyle.Render("? help · q quit")
	body := m.tabs[m.active].View()
	if m.showHelp {
		body = m.renderHelp()
	}
	parts := []string{bar}
	if m.lastErr != "" {
		parts = append(parts, lipgloss.NewStyle().Foreground(colorError).Render("! "+m.lastErr))
	}
	parts = append(parts, body)
	content := lipgloss.JoinVertical(lipgloss.Left, parts...)

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m appModel) renderHelp() string {
	lines := []string{
		titleStyle.Render("Global"),
		helpStyle.Render("  tab / shift+tab  switch tab"),
		helpStyle.Render("  r                refresh"),
		helpStyle.Render("  ?                toggle this help"),
		helpStyle.Render("  q                quit"),
		"",
		titleStyle.Render(m.tabs[m.active].Label() + " keys"),
	}
	for _, k := range m.tabs[m.active].HelpKeys() {
		lines = append(lines, helpStyle.Render(fmt.Sprintf("  %-12s %s", k.Keys, k.Help)))
	}
	return borderStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m *appModel) switchTab(delta int) {
	prev := m.tabs[m.active]
	n := len(m.tabs)
	m.active = (m.active + delta + n) % n
	if m.tabs[m.active] != prev {
		prev.OnLeave()
	}
	m.resizeTabs()
}

func (m *appModel) resizeTabs() {
	contentH := max(5, m.height-chromeHeight)
	for _, t := range m.tabs {
		t.SetSize(m.width, contentH)
	}
}

func (m appModel) findTab(id tabID) Tab {
	for i, t := range m.ids {
		if t == id {
			return m.tabs[i]
		}
	}
	return nil
}

func (m appModel) procOutKickCmd() tea.Cmd {
	if m.hasProcOut && m.ids[m.active] == tabProcOut {
		return procOutTickCmd()
	}
	return nil
}
