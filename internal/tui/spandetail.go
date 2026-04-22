package tui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

type attrRow struct {
	Key   string
	Value string
}

func parseAttrs(raw *string) []attrRow {
	if raw == nil || *raw == "" {
		return nil
	}
	var kv map[string]any
	if err := json.Unmarshal([]byte(*raw), &kv); err != nil {
		return []attrRow{{Key: "_raw", Value: *raw}}
	}
	keys := make([]string, 0, len(kv))
	for k := range kv {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]attrRow, 0, len(keys))
	for _, k := range keys {
		out = append(out, attrRow{Key: k, Value: stringifyValue(kv[k])})
	}
	return out
}

func stringifyValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case bool:
		return fmt.Sprintf("%t", val)
	case nil:
		return "null"
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(b)
	}
}

type spanDetailModel struct {
	vp     viewport.Model
	span   repo.Span
	hasSel bool
	width  int
	height int
}

func newSpanDetailModel() spanDetailModel {
	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	return spanDetailModel{vp: vp}
}

func (m *spanDetailModel) setSpan(s repo.Span) {
	m.span = s
	m.hasSel = true
	m.vp.SetContent(renderSpanDetail(s))
}

func (m spanDetailModel) Init() tea.Cmd { return nil }

func (m spanDetailModel) Update(msg tea.Msg) (spanDetailModel, tea.Cmd) {
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m spanDetailModel) View() string {
	header := titleStyle.Render("Span detail") + helpStyle.Render("  esc back · ↑/↓ scroll")
	if !m.hasSel {
		return lipgloss.JoinVertical(lipgloss.Left,
			header,
			borderStyle.Render("no span selected"),
		)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, borderStyle.Render(m.vp.View()))
}

func (m *spanDetailModel) setSize(w, h int) {
	m.width, m.height = w, h
	m.vp.SetWidth(max(40, w-4))
	m.vp.SetHeight(max(5, h-6))
}

func renderSpanDetail(s repo.Span) string {
	var b strings.Builder
	writeField(&b, "Trace", s.TraceID)
	writeField(&b, "Span", s.SpanID)
	parent := "-"
	if s.ParentSpanID != nil && *s.ParentSpanID != "" {
		parent = *s.ParentSpanID
	}
	writeField(&b, "Parent", parent)
	writeField(&b, "Name", s.Name)
	writeField(&b, "Service", s.ServiceName)
	writeField(&b, "Duration", fmtDuration(s.EndTimeNs-s.StartTimeNs))
	writeField(&b, "Status", fmt.Sprintf("%d", s.StatusCode))
	if s.StatusMessage != nil && *s.StatusMessage != "" {
		writeField(&b, "Message", *s.StatusMessage)
	}

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Attributes"))
	b.WriteString("\n")
	b.WriteString(renderAttrRows(parseAttrs(s.Attributes)))

	b.WriteString("\n\n")
	b.WriteString(titleStyle.Render("Events"))
	b.WriteString("\n")
	b.WriteString(renderEvents(s.Events))

	return b.String()
}

func writeField(b *strings.Builder, key, val string) {
	b.WriteString(attrKey.Render(key + ":"))
	b.WriteString(" ")
	b.WriteString(attrVal.Render(val))
	b.WriteString("\n")
}

func renderAttrRows(rows []attrRow) string {
	if len(rows) == 0 {
		return lipgloss.NewStyle().Foreground(colorMuted).Italic(true).Render("(none)")
	}
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(attrKey.Render(r.Key))
		b.WriteString(" = ")
		b.WriteString(attrVal.Render(r.Value))
		b.WriteString("\n")
	}
	return b.String()
}

func renderEvents(raw *string) string {
	if raw == nil || *raw == "" {
		return lipgloss.NewStyle().Foreground(colorMuted).Italic(true).Render("(none)")
	}
	var events []map[string]any
	if err := json.Unmarshal([]byte(*raw), &events); err != nil {
		return *raw
	}
	if len(events) == 0 {
		return lipgloss.NewStyle().Foreground(colorMuted).Italic(true).Render("(none)")
	}
	var b strings.Builder
	for _, ev := range events {
		name, _ := ev["name"].(string)
		b.WriteString(attrKey.Render("• " + name))
		b.WriteString("\n")
		if attrs, ok := ev["attributes"].(map[string]any); ok {
			keys := make([]string, 0, len(attrs))
			for k := range attrs {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				b.WriteString("    ")
				b.WriteString(attrKey.Render(k))
				b.WriteString(" = ")
				b.WriteString(attrVal.Render(stringifyValue(attrs[k])))
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}
