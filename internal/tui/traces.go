package tui

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type tracesModel struct {
	table    table.Model
	rows     []TraceRow
	lastSync time.Time
	width    int
	height   int
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

	s := table.Styles{
		Header:   lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(colorText).Background(colorBrand),
		Cell:     lipgloss.NewStyle().Padding(0, 1).Foreground(colorText),
		Selected: lipgloss.NewStyle().Padding(0, 1).Foreground(colorText).Background(lipgloss.Color("#313244")).Bold(true),
	}
	t.SetStyles(s)

	return tracesModel{table: t}
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

func (m tracesModel) Update(msg tea.Msg) (tracesModel, tea.Cmd) {
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m tracesModel) View() string {
	header := titleStyle.Render("Traces") +
		helpStyle.Render(fmt.Sprintf("  last sync %s · r refresh · enter select · tab switch", nowFmt()))
	body := borderStyle.Render(m.table.View())
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (m *tracesModel) setSize(w, h int) {
	m.width, m.height = w, h
	m.table.SetWidth(w - 4)
	m.table.SetHeight(max(5, h-6))
}

func (m tracesModel) selectedTraceID() (string, bool) {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.rows) {
		return "", false
	}
	return m.rows[idx].TraceID, true
}

func nowFmt() string { return time.Now().Format("15:04:05") }
