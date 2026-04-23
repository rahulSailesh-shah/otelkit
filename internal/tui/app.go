package tui

import (
	"context"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

type tabID int

const (
	tabTraces tabID = iota
	tabMetrics
	tabLogs
	tabProcOut
)

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

type tracesView int

const (
	viewList tracesView = iota
	viewWaterfall
	viewDetail
)

type logsView int

const (
	viewLogList logsView = iota
	viewLogDetail
)

type appModel struct {
	ctx     context.Context
	opts    Options
	queries *repo.Queries

	width  int
	height int

	activeTab  tabID
	tracesView tracesView
	logsView   logsView
	showHelp   bool

	traces      tracesModel
	waterfall   waterfallModel
	detail      spanDetailModel
	metrics     metricsModel
	logs        logsModel
	logDetail   logDetailModel
	procOut     procOutModel
	hasProcOut  bool
	lastTraceID string
	lastErr     string
}

func newAppModel(ctx context.Context, opts Options) appModel {
	m := appModel{
		ctx:       ctx,
		opts:      opts,
		queries:   opts.Queries,
		activeTab: tabTraces,
		traces:    newTracesModel(),
		detail:    newSpanDetailModel(),
		metrics:   newMetricsModel(),
		logs:      newLogsModel(),
		logDetail: newLogDetailModel(),
	}
	m.metrics.setSource(ctx, opts.Queries)
	if opts.ChildLogPath != "" {
		m.procOut = newProcOutModel(opts.ChildLogPath)
		m.hasProcOut = true
	}
	return m
}

func (m appModel) Init() tea.Cmd {
	return tea.Batch(
		loadTracesCmd(m.ctx, m.queries, 100),
		loadLogsCmd(m.ctx, m.queries, 200),
		loadMetricNamesCmd(m.ctx, m.queries),
		loadKPIsCmd(m.ctx, m.queries),
		tickCmd(m.opts.RefreshInterval),
	)
}

func (m appModel) logsFilterModeOnList() bool {
	return m.activeTab == tabLogs && m.logsView == viewLogList && m.logs.filterMode
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.resize()
		return m, nil

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Tab):
			if m.logsFilterModeOnList() {
				break
			}
			prev := m.activeTab
			m.activeTab = m.nextTab()
			if prev == tabTraces && m.activeTab != tabTraces {
				m.tracesView = viewList
				m.lastTraceID = ""
			}
			if prev == tabLogs && m.activeTab != tabLogs {
				m.logsView = viewLogList
			}
			m = m.resize()
			return m, m.procOutKickCmd()
		case key.Matches(msg, keys.ShiftTab):
			if m.logsFilterModeOnList() {
				break
			}
			prev := m.activeTab
			m.activeTab = m.prevTab()
			if prev == tabTraces && m.activeTab != tabTraces {
				m.tracesView = viewList
				m.lastTraceID = ""
			}
			if prev == tabLogs && m.activeTab != tabLogs {
				m.logsView = viewLogList
			}
			m = m.resize()
			return m, m.procOutKickCmd()
		case key.Matches(msg, keys.Help):
			m.showHelp = !m.showHelp
			return m, nil
		case key.Matches(msg, keys.Refresh):
			cmds := []tea.Cmd{
				loadTracesCmd(m.ctx, m.queries, 100),
				loadLogsCmd(m.ctx, m.queries, 200),
				loadMetricNamesCmd(m.ctx, m.queries),
				loadKPIsCmd(m.ctx, m.queries),
			}
			if name, ok := m.metrics.selectedName(); ok {
				cmds = append(cmds, loadMetricGroupsCmd(m.ctx, m.queries, name, metricWindowSec))
			}
			return m, tea.Batch(cmds...)
		}

		if m.activeTab == tabTraces {
			switch {
			case key.Matches(msg, keys.Enter):
				switch m.tracesView {
				case viewList:
					id, ok := m.traces.selectedTraceID()
					if ok {
						m.lastTraceID = id
						m.tracesView = viewWaterfall
						return m, loadTraceSpansCmd(m.ctx, m.queries, id)
					}
				case viewWaterfall:
					if s, ok := m.waterfall.selectedSpan(); ok {
						m.detail.setSpan(s)
						m.tracesView = viewDetail
					}
				}
				return m, nil
			case key.Matches(msg, keys.Back):
				switch m.tracesView {
				case viewDetail:
					m.tracesView = viewWaterfall
				case viewWaterfall:
					m.tracesView = viewList
				}
				return m, nil
			}
		}

		if m.activeTab == tabLogs {
			switch {
			case key.Matches(msg, keys.Enter):
				if m.logsFilterModeOnList() {
					break
				}
				if m.logsView == viewLogList {
					if row, ok := m.logs.selectedLog(); ok {
						m.logDetail.setLog(row)
						m.logsView = viewLogDetail
					}
				}
				return m, nil
			case key.Matches(msg, keys.Back):
				if m.logsView == viewLogDetail {
					m.logsView = viewLogList
					return m, nil
				}
				if m.logsFilterModeOnList() {
					break
				}
				return m, nil
			}
		}

	case tickMsg:
		cmds := []tea.Cmd{
			loadTracesCmd(m.ctx, m.queries, 100),
			loadLogsCmd(m.ctx, m.queries, 200),
			loadMetricNamesCmd(m.ctx, m.queries),
			loadKPIsCmd(m.ctx, m.queries),
			tickCmd(m.opts.RefreshInterval),
		}
		if name, ok := m.metrics.selectedName(); ok {
			cmds = append(cmds, loadMetricGroupsCmd(m.ctx, m.queries, name, metricWindowSec))
		}
		return m, tea.Batch(cmds...)

	case procOutTickMsg:
		if m.hasProcOut && m.activeTab == tabProcOut {
			m.procOut = m.procOut.tick()
			return m, procOutTickCmd()
		}
		return m, nil

	case tracesLoadedMsg:
		if msg.Err != nil {
			m.lastErr = "traces load: " + msg.Err.Error()
		} else {
			m.lastErr = ""
			m.traces.setRows(msg.Rows)
		}
		return m, nil

	case traceSpansLoadedMsg:
		if msg.Err != nil {
			m.lastErr = "spans load: " + msg.Err.Error()
			return m, nil
		}
		if msg.TraceID == m.lastTraceID {
			m.lastErr = ""
			m.waterfall.setSpans(msg.Spans)
		}
		return m, nil

	case logsLoadedMsg:
		if msg.Err != nil {
			m.lastErr = "logs load: " + msg.Err.Error()
		} else {
			m.lastErr = ""
			m.logs.setRows(msg.Rows)
		}
		return m, nil

	case metricNamesLoadedMsg:
		if msg.Err != nil {
			m.lastErr = "metric names load: " + msg.Err.Error()
			return m, nil
		}
		prev, hadPrev := m.metrics.selectedName()
		m.lastErr = ""
		m.metrics.setNames(msg.Rows)
		cur, hasCur := m.metrics.selectedName()
		if hasCur && (!hadPrev || prev != cur) {
			return m, loadMetricGroupsCmd(m.ctx, m.queries, cur, metricWindowSec)
		}
		return m, nil

	case kpisLoadedMsg:
		if msg.Err != nil {
			m.lastErr = "kpis load: " + msg.Err.Error()
			return m, nil
		}
		m.lastErr = ""
		m.metrics.setKPIs(msg.Data)
		return m, nil

	case metricGroupsLoadedMsg:
		if msg.Err != nil {
			m.lastErr = "metric groups load: " + msg.Err.Error()
			return m, nil
		}
		if name, ok := m.metrics.selectedName(); ok && name == msg.Name {
			m.lastErr = ""
			m.metrics.setGroups(msg)
		}
		return m, nil
	}

	return m.updateActive(msg)
}

func (m appModel) updateActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.activeTab {
	case tabTraces:
		switch m.tracesView {
		case viewList:
			m.traces, cmd = m.traces.Update(msg)
		case viewWaterfall:
			m.waterfall, cmd = m.waterfall.Update(msg)
		case viewDetail:
			m.detail, cmd = m.detail.Update(msg)
		}
	case tabMetrics:
		m.metrics, cmd = m.metrics.Update(msg)
	case tabLogs:
		switch m.logsView {
		case viewLogList:
			m.logs, cmd = m.logs.Update(msg)
		case viewLogDetail:
			m.logDetail, cmd = m.logDetail.Update(msg)
		}
	case tabProcOut:
		if m.hasProcOut {
			m.procOut, cmd = m.procOut.Update(msg)
		}
	}
	return m, cmd
}

func (m appModel) nextTab() tabID {
	order := m.tabOrder()
	for i, t := range order {
		if t == m.activeTab {
			return order[(i+1)%len(order)]
		}
	}
	return m.activeTab
}

func (m appModel) prevTab() tabID {
	order := m.tabOrder()
	for i, t := range order {
		if t == m.activeTab {
			return order[(i-1+len(order))%len(order)]
		}
	}
	return m.activeTab
}

func (m appModel) tabOrder() []tabID {
	order := []tabID{tabTraces, tabMetrics, tabLogs}
	if m.hasProcOut {
		order = append(order, tabProcOut)
	}
	return order
}

func (m appModel) procOutKickCmd() tea.Cmd {
	if m.hasProcOut && m.activeTab == tabProcOut {
		return procOutTickCmd()
	}
	return nil
}

func (m appModel) resize() appModel {
	contentH := max(5, m.height-3)
	m.traces.setSize(m.width, contentH)
	m.waterfall.setSize(m.width, contentH)
	m.detail.setSize(m.width, contentH)
	m.metrics.setSize(m.width, contentH)
	m.logs.setSize(m.width, contentH)
	m.logDetail.setSize(m.width, contentH)
	if m.hasProcOut {
		m.procOut = m.procOut.setSize(m.width, contentH)
	}
	return m
}

func (m appModel) View() tea.View {
	labels := []string{}
	for _, t := range m.tabOrder() {
		labels = append(labels, t.label())
	}
	active := 0
	for i, t := range m.tabOrder() {
		if t == m.activeTab {
			active = i
			break
		}
	}

	bar := joinTabs(labels, active) + "   " + helpStyle.Render("? help · q quit")
	body := m.viewActive()
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

func (m appModel) viewActive() string {
	switch m.activeTab {
	case tabTraces:
		switch m.tracesView {
		case viewList:
			return m.traces.View()
		case viewWaterfall:
			return m.waterfall.View()
		case viewDetail:
			return m.detail.View()
		}
	case tabMetrics:
		return m.metrics.View()
	case tabLogs:
		switch m.logsView {
		case viewLogList:
			return m.logs.View()
		case viewLogDetail:
			return m.logDetail.View()
		}
	case tabProcOut:
		if m.hasProcOut {
			return m.procOut.View()
		}
	}
	return ""
}
