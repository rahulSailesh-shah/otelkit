package tui

import (
	"context"
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

type tracesView int

const (
	tracesViewList tracesView = iota
	tracesViewWaterfall
	tracesViewDetail
)

type tracesModel struct {
	view        tracesView
	table       table.Model
	rows        []TraceRow
	waterfall   waterfallModel
	detail      spanDetailModel
	lastTraceID string
	lastSync    time.Time
	width       int
	height      int
}

func newTracesModel() tracesModel {
	cols := []table.Column{
		{Title: "Time", Width: 8},
		{Title: "Trace", Width: 10},
		{Title: "Service", Width: 22},
		{Title: "Root", Width: 28},
		{Title: "Dur", Width: 8},
		{Title: "Spans", Width: 5},
		{Title: "Status", Width: 6},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	t.SetStyles(table.Styles{
		Header:   lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(colorText).Background(colorBrand),
		Cell:     lipgloss.NewStyle().Padding(0, 1).Foreground(colorText),
		Selected: lipgloss.NewStyle().Padding(0, 1).Foreground(colorText).Background(colorSelect).Bold(true),
	})
	return tracesModel{table: t, detail: newSpanDetailModel()}
}

func (m *tracesModel) setRows(rows []TraceRow) {
	m.rows = rows
	tr := make([]table.Row, 0, len(rows))
	for _, r := range rows {
		status := "OK"
		if r.HasErrors {
			status = "ERR"
		}
		ts := time.Unix(0, r.StartTimeNs).Format("15:04:05")
		tr = append(tr, table.Row{
			ts,
			shortID(r.TraceID),
			truncate(r.Service, 22),
			truncate(r.RootName, 28),
			fmtDuration(r.DurationNs),
			fmt.Sprintf("%d", r.SpanCount),
			status,
		})
	}
	m.table.SetRows(tr)
	m.lastSync = time.Now()
}

func (m tracesModel) Init() tea.Cmd { return nil }

func (m *tracesModel) Update(ctx context.Context, q *repo.Queries, msg tea.Msg) (Tab, tea.Cmd) {
	// Intercept Enter/Back at this level so the sub-view stays a private detail.
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(keyMsg, keys.Enter):
			switch m.view {
			case tracesViewList:
				if id, ok := m.selectedTraceID(); ok {
					m.lastTraceID = id
					m.view = tracesViewWaterfall
					return m, loadTraceSpansCmd(ctx, q, id)
				}
			case tracesViewWaterfall:
				if s, ok := m.waterfall.selectedSpan(); ok {
					m.detail.setSpan(s)
					m.view = tracesViewDetail
				}
			}
			return m, nil
		case key.Matches(keyMsg, keys.Back):
			switch m.view {
			case tracesViewDetail:
				m.view = tracesViewWaterfall
			case tracesViewWaterfall:
				m.view = tracesViewList
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	switch m.view {
	case tracesViewList:
		m.table, cmd = m.table.Update(msg)
	case tracesViewWaterfall:
		m.waterfall, cmd = m.waterfall.Update(msg)
	case tracesViewDetail:
		m.detail, cmd = m.detail.Update(msg)
	}
	return m, cmd
}

func (m tracesModel) View() string {
	header := titleStyle.Render("Traces") +
		helpStyle.Render(fmt.Sprintf("  last sync %s · r refresh · enter select · tab switch", nowFmt()))
	switch m.view {
	case tracesViewWaterfall:
		return m.waterfall.View()
	case tracesViewDetail:
		return m.detail.View()
	}
	body := borderStyle.Render(m.table.View())
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}
func (m *tracesModel) Reset() {
	m.view = tracesViewList
	m.lastTraceID = ""
}

func (m *tracesModel) SetSpans(msg traceSpansLoadedMsg) {
	if msg.TraceID != m.lastTraceID {
		return
	}
	m.waterfall.setSpans(msg.Spans)
}

func (m *tracesModel) setSize(w, h int) {
	m.width, m.height = w, h
	m.table.SetWidth(w - 4)
	m.table.SetHeight(max(5, h-6))
	m.waterfall.setSize(w, h)
	m.detail.setSize(w, h)
}

func (m tracesModel) selectedTraceID() (string, bool) {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.rows) {
		return "", false
	}
	return m.rows[idx].TraceID, true
}

func (m tracesModel) Label() string { return "Traces" }

func (m *tracesModel) HelpKeys() []TabKey {
	return []TabKey{
		{Keys: "↑/↓", Help: "navigate"},
		{Keys: "enter", Help: "select"},
		{Keys: "esc", Help: "back"},
	}
}

func (m *tracesModel) SetSize(w, h int) { m.setSize(w, h) }

func (m tracesModel) RefreshCmd(ctx context.Context, q *repo.Queries) tea.Cmd {
	return loadTracesCmd(ctx, q, tracesPageSize)
}

func (m *tracesModel) OnLeave() { m.Reset() }

func (m tracesModel) ConsumesTab() bool { return false }

func (m tracesModel) ConsumesEnter() bool { return false }
