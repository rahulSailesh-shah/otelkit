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

type appModel struct {
	ctx     context.Context
	opts    Options
	queries *repo.Queries

	width  int
	height int

	activeTab  tabID
	tracesView tracesView
	showHelp   bool

	traces      tracesModel
	waterfall   waterfallModel
	detail      spanDetailModel
	metrics     placeholderModel
	logs        placeholderModel
	procOut     procOutModel
	hasProcOut  bool
	lastTraceID string
}

func newAppModel(ctx context.Context, opts Options) appModel {
	m := appModel{
		ctx:       ctx,
		opts:      opts,
		queries:   opts.Queries,
		activeTab: tabTraces,
		traces:    newTracesModel(),
		detail:    newSpanDetailModel(),
		metrics:   newPlaceholder("Metrics coming soon"),
		logs:      newPlaceholder("Logs coming soon"),
	}
	if opts.ChildLogPath != "" {
		m.procOut = newProcOutModel(opts.ChildLogPath)
		m.hasProcOut = true
	}
	return m
}

func (m appModel) Init() tea.Cmd {
	return tea.Batch(
		loadTracesCmd(m.ctx, m.queries, 100),
		tickCmd(m.opts.RefreshInterval),
	)
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
			m.activeTab = m.nextTab()
			m = m.resize()
			return m, nil
		case key.Matches(msg, keys.ShiftTab):
			m.activeTab = m.prevTab()
			m = m.resize()
			return m, nil
		case key.Matches(msg, keys.Help):
			m.showHelp = !m.showHelp
			return m, nil
		case key.Matches(msg, keys.Refresh):
			return m, loadTracesCmd(m.ctx, m.queries, 100)
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

	case tickMsg:
		cmds := []tea.Cmd{
			loadTracesCmd(m.ctx, m.queries, 100),
			tickCmd(m.opts.RefreshInterval),
		}
		if m.hasProcOut && m.activeTab == tabProcOut {
			var cmd tea.Cmd
			m.procOut, cmd = m.procOut.tick()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)

	case tracesLoadedMsg:
		if msg.Err == nil {
			m.traces.setRows(msg.Rows)
		}
		return m, nil

	case traceSpansLoadedMsg:
		if msg.Err == nil && msg.TraceID == m.lastTraceID {
			m.waterfall.setSpans(msg.Spans)
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

func (m appModel) resize() appModel {
	contentH := max(5, m.height-3)
	m.traces.setSize(m.width, contentH)
	m.waterfall.setSize(m.width, contentH)
	m.detail.setSize(m.width, contentH)
	m.metrics = m.metrics.setSize(m.width, contentH)
	m.logs = m.logs.setSize(m.width, contentH)
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

	content := lipgloss.JoinVertical(lipgloss.Left, bar, body)

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
		return m.logs.View()
	case tabProcOut:
		if m.hasProcOut {
			return m.procOut.View()
		}
	}
	return ""
}

// ----- temporary procOut stub until Task 15 lands -----
// Task 15 will replace this file/type with the real tailing model in procout.go
// (this stub keeps the build green now).
type procOutModel struct {
	path   string
	width  int
	height int
}

func newProcOutModel(path string) procOutModel { return procOutModel{path: path} }

func (m procOutModel) View() string {
	return lipgloss.NewStyle().Foreground(colorMuted).Italic(true).
		Render("process output tailing from: " + m.path)
}

func (m procOutModel) setSize(w, h int) procOutModel {
	m.width, m.height = w, h
	return m
}

func (m procOutModel) tick() (procOutModel, tea.Cmd) { return m, nil }
