package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type logDetailModel struct {
	vp     viewport.Model
	log    LogRow
	hasSel bool
	width  int
	height int
}

func newLogDetailModel() logDetailModel {
	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	return logDetailModel{vp: vp}
}

func (m *logDetailModel) setLog(l LogRow) {
	m.log = l
	m.hasSel = true
	m.vp.SetContent(renderLogDetail(l))
}

func (m logDetailModel) Init() tea.Cmd { return nil }

func (m logDetailModel) Update(msg tea.Msg) (logDetailModel, tea.Cmd) {
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m logDetailModel) View() string {
	header := titleStyle.Render("Log detail") + helpStyle.Render("  esc back · ↑/↓ scroll")
	if !m.hasSel {
		return lipgloss.JoinVertical(lipgloss.Left,
			header,
			borderStyle.Render("no log selected"),
		)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, borderStyle.Render(m.vp.View()))
}

func (m *logDetailModel) setSize(w, h int) {
	m.width, m.height = w, h
	m.vp.SetWidth(max(40, w-4))
	m.vp.SetHeight(max(5, h-6))
}

func renderLogDetail(l LogRow) string {
	var b strings.Builder
	writeField(&b, "Time", time.Unix(0, l.TimestampNs).Format("2006-01-02 15:04:05.000"))

	sevLabel := severityLabel(l.Severity, strOrNil(l.SeverityText))
	sevText := sevLabel
	if l.Severity != nil {
		sevText = fmt.Sprintf("%s (%d)", sevLabel, *l.Severity)
	}
	writeField(&b, "Severity", sevText)

	writeField(&b, "Service", l.Service)
	writeField(&b, "Trace", orDash(l.TraceID))
	writeField(&b, "Span", orDash(l.SpanID))

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Body"))
	b.WriteString("\n")
	if l.Body == "" {
		b.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Italic(true).Render("(empty)"))
	} else {
		b.WriteString(attrVal.Render(l.Body))
	}
	b.WriteString("\n\n")

	b.WriteString(titleStyle.Render("Attributes"))
	b.WriteString("\n")
	b.WriteString(renderAttrRows(parseAttrs(l.Attributes)))

	b.WriteString("\n\n")
	b.WriteString(titleStyle.Render("Resource"))
	b.WriteString("\n")
	b.WriteString(renderAttrRows(parseAttrs(l.ResourceAttrs)))

	return b.String()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
