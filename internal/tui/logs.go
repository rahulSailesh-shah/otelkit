package tui

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type logsModel struct {
	table    table.Model
	rows     []LogRow
	lastSync time.Time
	width    int
	height   int
}

func newLogsModel() logsModel {
	cols := []table.Column{
		{Title: "Time", Width: 8},
		{Title: "Sev", Width: 5},
		{Title: "Service", Width: 22},
		{Title: "Trace", Width: 10},
		{Title: "Body", Width: 40},
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
	return logsModel{table: t}
}

func severityStyle(label string) lipgloss.Style {
	switch label {
	case "ERROR":
		return lipgloss.NewStyle().Foreground(colorError).Bold(true)
	case "FATAL":
		return lipgloss.NewStyle().Foreground(colorError).Bold(true).Underline(true)
	case "WARN":
		return lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	case "INFO":
		return lipgloss.NewStyle().Foreground(colorText)
	case "DEBUG", "TRACE":
		return lipgloss.NewStyle().Foreground(colorMuted)
	default:
		return lipgloss.NewStyle().Foreground(colorMuted)
	}
}

func (m *logsModel) setRows(rows []LogRow) {
	m.rows = rows
	tr := make([]table.Row, 0, len(rows))
	for _, r := range rows {
		ts := time.Unix(0, r.TimestampNs).Format("15:04:05")
		label := severityLabel(r.Severity, strOrNil(r.SeverityText))
		trace := "-"
		if r.TraceID != "" {
			trace = shortID(r.TraceID)
		}
		tr = append(tr, table.Row{
			ts,
			severityStyle(label).Render(truncate(label, 5)),
			truncate(r.Service, 22),
			trace,
			truncate(r.Body, 40),
		})
	}
	m.table.SetRows(tr)
	m.lastSync = time.Now()
}

func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (m logsModel) Init() tea.Cmd { return nil }

func (m logsModel) Update(msg tea.Msg) (logsModel, tea.Cmd) {
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m logsModel) View() string {
	header := titleStyle.Render("Logs") +
		helpStyle.Render(fmt.Sprintf("  last sync %s · r refresh · enter select · tab switch", nowFmt()))
	body := borderStyle.Render(m.table.View())
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (m *logsModel) setSize(w, h int) {
	m.width, m.height = w, h
	m.table.SetWidth(w - 4)
	m.table.SetHeight(max(5, h-6))
	cols := m.table.Columns()
	used := 0
	for i, c := range cols {
		if i == len(cols)-1 {
			break
		}
		used += c.Width + 2
	}
	remaining := max(20, (w-4)-used-4)
	if len(cols) > 0 {
		cols[len(cols)-1].Width = remaining
		m.table.SetColumns(cols)
	}
}

func (m logsModel) selectedLog() (LogRow, bool) {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.rows) {
		return LogRow{}, false
	}
	return m.rows[idx], true
}
