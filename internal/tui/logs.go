package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type logsFilterField int

const (
	logsFilterFieldSeverity logsFilterField = iota
	logsFilterFieldService
	logsFilterFieldBody
)

var logsSeverityOptions = []string{"all", "fatal", "error", "warn", "info", "debug", "trace"}

type logsFilterState struct {
	Severity string
	Service  string
	Body     string
}

type logsModel struct {
	table    table.Model
	allRows  []LogRow
	rows     []LogRow
	filters  logsFilterState
	filterMode        bool
	activeFilterField logsFilterField
	serviceInput      textinput.Model
	bodyInput         textinput.Model
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
	serviceInput := textinput.New()
	serviceInput.Prompt = ""
	serviceInput.Placeholder = "service"
	serviceInput.Blur()
	bodyInput := textinput.New()
	bodyInput.Prompt = ""
	bodyInput.Placeholder = "body"
	bodyInput.Blur()
	return logsModel{
		table:        t,
		filters:      logsFilterState{Severity: "all"},
		serviceInput: serviceInput,
		bodyInput:    bodyInput,
	}
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

func (m logsModel) filtersActive() bool {
	sev := strings.ToLower(strings.TrimSpace(m.filters.Severity))
	if sev != "" && sev != "all" {
		return true
	}
	if strings.TrimSpace(m.filters.Service) != "" {
		return true
	}
	if strings.TrimSpace(m.filters.Body) != "" {
		return true
	}
	return false
}

func (m logsModel) matchesFilters(row LogRow) bool {
	severity := strings.ToLower(strings.TrimSpace(m.filters.Severity))
	if severity != "" && severity != "all" {
		label := strings.ToLower(severityLabel(row.Severity, strOrNil(row.SeverityText)))
		if label != severity {
			return false
		}
	}

	service := strings.ToLower(strings.TrimSpace(m.filters.Service))
	if service != "" && !strings.Contains(strings.ToLower(row.Service), service) {
		return false
	}

	body := strings.ToLower(strings.TrimSpace(m.filters.Body))
	if body != "" && !strings.Contains(strings.ToLower(row.Body), body) {
		return false
	}

	return true
}

func (m *logsModel) applyFilters() {
	filtered := make([]LogRow, 0, len(m.allRows))
	for _, row := range m.allRows {
		if m.matchesFilters(row) {
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

func (m *logsModel) clearFilters() {
	m.filters = logsFilterState{Severity: "all"}
	m.syncInputsFromFilters()
	m.applyFilters()
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

func (m *logsModel) syncInputsFromFilters() {
	m.serviceInput.SetValue(m.filters.Service)
	m.bodyInput.SetValue(m.filters.Body)
}

func (m *logsModel) focusActiveFilterField() tea.Cmd {
	m.serviceInput.Blur()
	m.bodyInput.Blur()
	switch m.activeFilterField {
	case logsFilterFieldService:
		return m.serviceInput.Focus()
	case logsFilterFieldBody:
		return m.bodyInput.Focus()
	default:
		return nil
	}
}

func (m *logsModel) enterFilterMode() tea.Cmd {
	m.filterMode = true
	m.activeFilterField = logsFilterFieldSeverity
	m.syncInputsFromFilters()
	return m.focusActiveFilterField()
}

func (m *logsModel) exitFilterMode() {
	m.filterMode = false
	m.serviceInput.Blur()
	m.bodyInput.Blur()
}

func (m *logsModel) cycleActiveFilterField() tea.Cmd {
	m.activeFilterField = (m.activeFilterField + 1) % 3
	return m.focusActiveFilterField()
}

func (m *logsModel) cycleActiveFilterFieldBack() tea.Cmd {
	m.activeFilterField = (m.activeFilterField + 2) % 3
	return m.focusActiveFilterField()
}

func (m *logsModel) cycleSeverity(delta int) {
	current := 0
	for i, option := range logsSeverityOptions {
		if option == m.filters.Severity {
			current = i
			break
		}
	}
	next := (current + delta + len(logsSeverityOptions)) % len(logsSeverityOptions)
	m.filters.Severity = logsSeverityOptions[next]
	m.applyFilters()
}

func (m *logsModel) updateActiveTextFilter(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.activeFilterField {
	case logsFilterFieldService:
		m.serviceInput, cmd = m.serviceInput.Update(msg)
		m.filters.Service = m.serviceInput.Value()
	case logsFilterFieldBody:
		m.bodyInput, cmd = m.bodyInput.Update(msg)
		m.filters.Body = m.bodyInput.Value()
	default:
		return nil
	}
	m.applyFilters()
	return cmd
}

func (m logsModel) Update(msg tea.Msg) (logsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if !m.filterMode && msg.String() == "f" {
			return m, m.enterFilterMode()
		}
		if m.filterMode {
			switch msg.String() {
			case "esc":
				m.exitFilterMode()
				return m, nil
			case "tab":
				return m, m.cycleActiveFilterField()
			case "shift+tab":
				return m, m.cycleActiveFilterFieldBack()
			case "c":
				if m.activeFilterField == logsFilterFieldService || m.activeFilterField == logsFilterFieldBody {
					return m, m.updateActiveTextFilter(msg)
				}
				m.clearFilters()
				return m, m.focusActiveFilterField()
			case "left":
				if m.activeFilterField == logsFilterFieldSeverity {
					m.cycleSeverity(-1)
					return m, nil
				}
			case "right":
				if m.activeFilterField == logsFilterFieldSeverity {
					m.cycleSeverity(1)
					return m, nil
				}
			}
			if m.activeFilterField == logsFilterFieldService || m.activeFilterField == logsFilterFieldBody {
				return m, m.updateActiveTextFilter(msg)
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
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
	focus := m.filterMode
	severity := m.renderFilterField("Severity", m.filters.Severity, focus && m.activeFilterField == logsFilterFieldSeverity)
	service := m.renderFilterField("Service", m.filters.Service, focus && m.activeFilterField == logsFilterFieldService)
	body := m.renderFilterField("Body", m.filters.Body, focus && m.activeFilterField == logsFilterFieldBody)
	if focus {
		service = m.renderFilterInputField("Service", m.serviceInput.View(), m.activeFilterField == logsFilterFieldService)
		body = m.renderFilterInputField("Body", m.bodyInput.View(), m.activeFilterField == logsFilterFieldBody)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, severity, "  ", service, "  ", body)
}

func (m logsModel) View() string {
	help := fmt.Sprintf("  last sync %s · f filter · r refresh · enter select · tab switch", nowFmt())
	if m.filterMode {
		help += " · c clear (severity field)"
	}
	header := titleStyle.Render("Logs") + helpStyle.Render(help)
	content := m.table.View()
	if m.filterMode || m.filtersActive() {
		content = lipgloss.JoinVertical(lipgloss.Left, m.renderFilterRow(), content)
	}
	body := borderStyle.Render(content)
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
	inputWidth := max(12, (w-20)/4)
	m.serviceInput.SetWidth(inputWidth)
	m.bodyInput.SetWidth(inputWidth * 2)
}

func (m logsModel) selectedLog() (LogRow, bool) {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.rows) {
		return LogRow{}, false
	}
	return m.rows[idx], true
}
