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

type logsView int

const (
	logsViewList logsView = iota
	logsViewDetail
)

type logsModel struct {
	view     logsView
	table    table.Model
	allRows  []LogRow
	rows     []LogRow
	filter   logsFilter
	detail   logDetailModel
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
		Selected: lipgloss.NewStyle().Padding(0, 1).Foreground(colorText).Background(colorSelect).Bold(true),
	})
	return logsModel{table: t, filter: newLogsFilter(), detail: newLogDetailModel()}
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

func renderLogTableRows(rows []LogRow) []table.Row {
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
	return tr
}

func (m *logsModel) applyFilters() {
	filtered := make([]LogRow, 0, len(m.allRows))
	for _, row := range m.allRows {
		if m.filter.Matches(row) {
			filtered = append(filtered, row)
		}
	}
	m.rows = filtered
	m.table.SetRows(renderLogTableRows(filtered))
	if cursor := m.table.Cursor(); len(filtered) == 0 {
		m.table.SetCursor(0)
	} else if cursor >= len(filtered) {
		m.table.SetCursor(len(filtered) - 1)
	}
}

func (m *logsModel) setRows(rows []LogRow) {
	m.allRows = rows
	m.applyFilters()
	m.lastSync = time.Now()
}

func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (m logsModel) Init() tea.Cmd { return nil }

func (m *logsModel) Update(ctx context.Context, q *repo.Queries, msg tea.Msg) (Tab, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			if key.Matches(keyMsg, keys.Enter) && !m.filter.Active() && m.view == logsViewList {
				if row, ok := m.selectedLog(); ok {
					m.detail.setLog(row)
					m.view = logsViewDetail
				}
				return m, nil
			}
			if key.Matches(keyMsg, keys.Back) && m.view == logsViewDetail {
				m.view = logsViewList
				return m, nil
			}
		}
		if !m.filter.Active() && keyMsg.String() == "f" {
			cmd := m.filter.Enter()
			return m, cmd
		}
		if m.filter.Active() {
			switch keyMsg.String() {
			case "esc":
				m.filter.Exit()
				return m, nil
			case "tab":
				return m, m.filter.NextField()
			case "shift+tab":
				return m, m.filter.PrevField()
			case "c":
				if m.filter.Field() == logsFilterFieldSeverity {
					cmd := m.filter.Clear()
					m.applyFilters()
					return m, cmd
				}
				cmd := m.filter.UpdateInput(msg)
				m.applyFilters()
				return m, cmd
			case "left":
				if m.filter.Field() == logsFilterFieldSeverity {
					m.filter.CycleSeverity(-1)
					m.applyFilters()
					return m, nil
				}
			case "right":
				if m.filter.Field() == logsFilterFieldSeverity {
					m.filter.CycleSeverity(1)
					m.applyFilters()
					return m, nil
				}
			}
			if m.filter.Field() == logsFilterFieldService || m.filter.Field() == logsFilterFieldBody {
				cmd := m.filter.UpdateInput(msg)
				m.applyFilters()
				return m, cmd
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	switch m.view {
	case logsViewList:
		m.table, cmd = m.table.Update(msg)
	case logsViewDetail:
		m.detail, cmd = m.detail.Update(msg)
	}
	return m, cmd
}

func (m logsModel) renderFilterField(label, value string, active bool) string {
	fieldStyle := lipgloss.NewStyle().Foreground(colorText)
	if active {
		fieldStyle = fieldStyle.Bold(true).Underline(true)
	} else {
		fieldStyle = fieldStyle.Foreground(colorMuted)
	}
	return fieldStyle.Render(fmt.Sprintf("%s: %s", label, value))
}

func (m logsModel) renderFilterInputField(label, input string, active bool) string {
	labelStyle := lipgloss.NewStyle().Foreground(colorMuted)
	if active {
		labelStyle = labelStyle.Foreground(colorText).Bold(true).Underline(true)
	}
	// textinput.View() already carries its own styling/cursor escapes; append it
	// directly so we don't re-style and leak raw ANSI fragments.
	return labelStyle.Render(label+": ") + input
}

func (m logsModel) renderFilterRow() string {
	focus := m.filter.Active()
	state := m.filter.State()
	severity := m.renderFilterField("Severity", state.Severity, focus && m.filter.Field() == logsFilterFieldSeverity)
	service := m.renderFilterField("Service", state.Service, focus && m.filter.Field() == logsFilterFieldService)
	body := m.renderFilterField("Body", state.Body, focus && m.filter.Field() == logsFilterFieldBody)
	if focus {
		service = m.renderFilterInputField("Service", m.filter.serviceView(), m.filter.Field() == logsFilterFieldService)
		body = m.renderFilterInputField("Body", m.filter.bodyView(), m.filter.Field() == logsFilterFieldBody)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, severity, "  ", service, "  ", body)
}

func (m logsModel) View() string {
	if m.view == logsViewDetail {
		return m.detail.View()
	}
	help := fmt.Sprintf("  last sync %s · f filter · r refresh · enter select · tab switch", nowFmt())
	if m.filter.Active() {
		help += " · c clear (severity field)"
	}
	header := titleStyle.Render("Logs") + helpStyle.Render(help)
	content := m.table.View()
	if m.filter.Active() || m.filter.HasCriteria() {
		content = lipgloss.JoinVertical(lipgloss.Left, m.renderFilterRow(), content)
	}
	body := borderStyle.Render(content)
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (m *logsModel) Reset() {
	m.view = logsViewList
}

func (m *logsModel) setSize(w, h int) {
	m.width, m.height = w, h
	m.table.SetWidth(w - 4)
	m.table.SetHeight(max(5, h-6))
	m.detail.setSize(w, h)
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
	m.filter.SetInputWidth(max(12, (w-20)/4))
}

func (m logsModel) selectedLog() (LogRow, bool) {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.rows) {
		return LogRow{}, false
	}
	return m.rows[idx], true
}

func (m logsModel) ConsumesTab() bool { return m.filter.Active() }

func (m logsModel) ConsumesEnter() bool { return m.filter.Active() }

func (m logsModel) Label() string { return "Logs" }

func (m *logsModel) HelpKeys() []TabKey {
	return []TabKey{
		{Keys: "↑/↓", Help: "navigate"},
		{Keys: "enter", Help: "select"},
		{Keys: "esc", Help: "back / close filter"},
		{Keys: "f", Help: "filter"},
		{Keys: "c", Help: "clear (severity field)"},
		{Keys: "tab", Help: "cycle filter field (in filter mode)"},
	}
}

func (m *logsModel) SetSize(w, h int) { m.setSize(w, h) }

func (m logsModel) RefreshCmd(ctx context.Context, q *repo.Queries) tea.Cmd {
	return loadLogsCmd(ctx, q, logsPageSize)
}

func (m *logsModel) OnLeave() { m.Reset() }
